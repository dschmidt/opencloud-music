// Package auth extracts the Subsonic credentials from each request and
// stashes them on the request context so Graph/WebDAV calls can
// forward them as HTTP Basic Auth to OpenCloud.
//
// What's actually going on: OpenCloud's "app tokens" are a drop-in
// replacement for a user's primary password inside HTTP Basic Auth.
// They are NOT opaque API tokens in the Subsonic sense — they cannot
// identify the user on their own, so we always need both the username
// (`u=`) and the app token. We therefore DO NOT advertise the
// OpenSubsonic `apiKeyAuthentication` extension; the canonical flow
// is plain `u` + `p`.
//
// Supported authentication surfaces (first non-empty wins):
//
//  1. Standard HTTP Basic Auth — the client sends `Authorization: Basic
//     <base64(user:password)>`. Handy for reverse proxies and
//     curl/httpie.
//  2. Subsonic `?u=<user>&p=<password>` /
//     `?u=<user>&p=enc:<hex>` — the classic Subsonic credential
//     channel. `p` carries the OpenCloud app token.
//  3. `u` / `p` in a POST form body — same rules, read from the form
//     body after ParseForm.
//
// Explicitly rejected (HTTP 200 + Subsonic error 42 "auth mechanism not
// supported"):
//
//   - Legacy HMAC (`?u=X&t=<md5(pw+salt)>&s=<salt>`). The server
//     cannot validate these because OpenCloud app tokens are opaque
//     to the music service — we have nothing to hash.
package auth

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"
)

// Credentials is the (username, app-token) pair forwarded to
// OpenCloud as HTTP Basic Auth.
type Credentials struct {
	Username string
	Password string
}

type credsCtxKey struct{}

// FromContext returns the credentials stored on the request context,
// or (zero, false) if none were set. Handlers that need to talk to
// Graph/WebDAV must call this to forward the user's credentials.
func FromContext(ctx context.Context) (Credentials, bool) {
	c, ok := ctx.Value(credsCtxKey{}).(Credentials)
	return c, ok && c.Password != "" && c.Username != ""
}

// WithCredentials returns a derived context carrying creds. Exposed
// for tests; production code goes through Middleware.
func WithCredentials(ctx context.Context, c Credentials) context.Context {
	return context.WithValue(ctx, credsCtxKey{}, c)
}

// decodeSubsonicPassword handles Subsonic's `enc:<hex>` prefix that
// some clients use to obfuscate passwords in URLs. Plain values pass
// through unchanged.
func decodeSubsonicPassword(v string) string {
	rest, ok := strings.CutPrefix(v, "enc:")
	if !ok {
		return v
	}
	b, err := hex.DecodeString(rest)
	if err != nil {
		return ""
	}
	return string(b)
}

// extractBasicAuth decodes a standard `Authorization: Basic` header.
func extractBasicAuth(r *http.Request) (user, pass string, ok bool) {
	raw, found := strings.CutPrefix(r.Header.Get("Authorization"), "Basic ")
	if !found {
		return "", "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return "", "", false
	}
	user, pass, ok = strings.Cut(string(decoded), ":")
	return user, pass, ok && user != "" && pass != ""
}

// extractCredentials returns the (user, password) pair carried by r,
// if any. See the package doc for the resolution order.
func extractCredentials(r *http.Request) Credentials {
	// 1. HTTP Basic header — carries both halves on its own.
	if u, p, ok := extractBasicAuth(r); ok {
		return Credentials{Username: u, Password: p}
	}

	// 2. Subsonic `?u=` + `?p=` (with optional `enc:<hex>`).
	q := r.URL.Query()
	user := q.Get("u")
	password := decodeSubsonicPassword(q.Get("p"))

	// 3. POST form body fallback.
	if r.Method == http.MethodPost && (user == "" || password == "") {
		if err := r.ParseForm(); err == nil {
			if user == "" {
				user = r.PostForm.Get("u")
			}
			if password == "" {
				password = decodeSubsonicPassword(r.PostForm.Get("p"))
			}
		}
	}
	return Credentials{Username: user, Password: password}
}

// usedHMACAuth reports whether the request tries to use the classic
// Subsonic token-plus-salt HMAC challenge. We can't honour that scheme
// because OpenCloud app tokens never leave OpenCloud in plaintext —
// we have nothing to hash.
func usedHMACAuth(r *http.Request) bool {
	q := r.URL.Query()
	if q.Get("t") != "" || q.Get("s") != "" {
		return true
	}
	if r.Method == http.MethodPost {
		// ParseForm is idempotent so a second call here is cheap.
		if err := r.ParseForm(); err == nil {
			if r.PostForm.Get("t") != "" || r.PostForm.Get("s") != "" {
				return true
			}
		}
	}
	return false
}

// Middleware extracts the credentials and attaches them to the request
// context. It does NOT reject missing creds — the `/rest/ping` and
// `/rest/getOpenSubsonicExtensions` endpoints are public so clients
// can probe server capabilities without a key. Handlers that need
// creds check for them via FromContext and emit the appropriate
// Subsonic error themselves.
//
// Requests that try to use the unsupported HMAC auth flow are rejected
// upfront with a Subsonic-formatted failure envelope so clients get a
// clear "use apiKey instead" message rather than a confusing
// missing-parameter error.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if usedHMACAuth(r) {
			writeHMACRejection(w)
			return
		}
		if c := extractCredentials(r); c.Username != "" && c.Password != "" {
			r = r.WithContext(WithCredentials(r.Context(), c))
		}
		next.ServeHTTP(w, r)
	})
}

// writeHMACRejection emits a Subsonic-formatted failure (code 42) so
// clients receive the expected JSON envelope. Kept here rather than in
// subsonic/envelope.go to avoid the import cycle and because this is
// the only place in auth/ that produces a protocol response.
func writeHMACRejection(w http.ResponseWriter) {
	const body = `{"subsonic-response":{` +
		`"status":"failed","version":"1.16.1","type":"opencloud-music",` +
		`"serverVersion":"0.1.0","openSubsonic":true,` +
		`"error":{"code":42,"message":"token+salt HMAC auth is not supported; pass your OpenCloud app token as ?u=<user>&p=<password> (or via HTTP Basic Auth)"}` +
		`}}`
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}
