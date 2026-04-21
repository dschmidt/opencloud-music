// Package auth extracts the caller's OpenCloud credentials from each
// request and stashes them on the request context so Graph/WebDAV
// calls can forward them.
//
// Two credential shapes are accepted:
//
//  1. An OIDC access token via `Authorization: Bearer <token>`. This
//     is the path the OpenCloud web UI extension uses — the browser
//     already holds a token scoped to OpenCloud, and we just pipe it
//     through.
//  2. An OpenCloud app token paired with a username, via one of:
//     - `Authorization: Basic <base64(user:token)>`
//     - `?u=<user>&p=<token>` / `?u=<user>&p=enc:<hex>`
//     - `u` / `p` in a POST form body.
//     App tokens are drop-in replacements for the user's primary
//     password inside HTTP Basic Auth — they can't identify the user
//     on their own, which is why a companion username is always
//     required.
//
// Explicitly rejected (HTTP 200 + Subsonic error 42):
//
//   - Legacy HMAC (`?u=X&t=<md5(pw+salt)>&s=<salt>`). The server
//     cannot validate these because OpenCloud app tokens never leave
//     OpenCloud in plaintext — we have nothing to hash against the
//     client's salt.
package auth

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/opencloud-eu/opencloud-music/internal/subsonic/proto"
)

// Credentials is what the auth middleware extracts from the inbound
// request. Exactly one of BearerToken OR (Username + Password) is
// populated: callers that forward to OpenCloud branch on IsBearer.
type Credentials struct {
	// BearerToken is an OIDC access token scoped to OpenCloud. When
	// set, it's forwarded verbatim as `Authorization: Bearer …` and
	// the username/password fields stay empty.
	BearerToken string

	// Username + Password carry the app-token flavour of credential:
	// Password is the OpenCloud app token, Username is the OpenCloud
	// account it belongs to. Forwarded as HTTP Basic Auth.
	Username string
	Password string
}

// IsBearer reports whether the credentials represent a Bearer token
// rather than a Basic-auth (username, app-token) pair.
func (c Credentials) IsBearer() bool { return c.BearerToken != "" }

// Valid reports whether the credentials carry either an OIDC access
// token or a (username, app-token) pair that's safe to forward.
func (c Credentials) Valid() bool {
	if c.BearerToken != "" {
		return true
	}
	return c.Username != "" && c.Password != ""
}

type credsCtxKey struct{}

// FromContext returns the credentials stored on the request context,
// or (zero, false) if none were set. Handlers that need to talk to
// Graph/WebDAV must call this to forward the user's credentials.
func FromContext(ctx context.Context) (Credentials, bool) {
	c, ok := ctx.Value(credsCtxKey{}).(Credentials)
	return c, ok && c.Valid()
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

// extractBearer returns the token from an `Authorization: Bearer …`
// header, or "" if the header isn't a Bearer form.
func extractBearer(r *http.Request) string {
	raw, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !found {
		return ""
	}
	return strings.TrimSpace(raw)
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

// extractCredentials returns the credentials carried by r, if any.
// See the package doc for the resolution order.
func extractCredentials(r *http.Request) Credentials {
	// 1. Bearer token — no companion username needed, forwarded as-is.
	if tok := extractBearer(r); tok != "" {
		return Credentials{BearerToken: tok}
	}

	// 2. HTTP Basic header — (user, app-token) pair in one header.
	if u, p, ok := extractBasicAuth(r); ok {
		return Credentials{Username: u, Password: p}
	}

	// 3. Subsonic `?u=` + `?p=` (with optional `enc:<hex>`).
	q := r.URL.Query()
	user := q.Get("u")
	password := decodeSubsonicPassword(q.Get("p"))

	// 4. POST form body fallback.
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
// Subsonic token-plus-salt HMAC challenge.
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
// upfront with a Subsonic-formatted failure envelope (code 42) so
// clients get a clear "use app-token auth instead" message rather
// than a confusing missing-parameter error further down the chain.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if usedHMACAuth(r) {
			proto.WriteError(w, proto.ErrAuthNotSupported,
				"token+salt HMAC auth is not supported; pass your OpenCloud app token as ?u=<user>&p=<password> (or via HTTP Basic Auth)")
			return
		}
		if c := extractCredentials(r); c.Valid() {
			r = r.WithContext(WithCredentials(r.Context(), c))
		}
		next.ServeHTTP(w, r)
	})
}
