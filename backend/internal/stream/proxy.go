// Package stream proxies audio bytes from an OpenCloud DriveItem's
// WebDAV URL to the Subsonic client. It forwards conditional- and
// range-request headers so clients can seek into tracks and avoid
// re-downloading unchanged media.
package stream

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/opencloud-eu/opencloud-music/internal/auth"
)

// Proxier is configured once at server start with the TLS policy for
// OpenCloud (dev setups use self-signed certs) and reused across every
// /rest/stream call.
type Proxier struct {
	client *http.Client
}

// New constructs a Proxier. insecure enables TLS skip for self-signed
// OpenCloud instances — do not enable in production.
func New(insecure bool) *Proxier {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if insecure {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // opt-in for dev
	}
	return &Proxier{client: &http.Client{Transport: tr}}
}

// Serve executes an HTTP GET against webDavURL using the caller's
// OpenCloud credentials — Bearer for OIDC access tokens forwarded
// from the web UI, Basic for (username, app-token) pairs — then
// streams the response (status code, selected headers, body) back
// through w. Range / conditional headers from r are forwarded
// verbatim so Subsonic clients can request partial content.
func (p *Proxier) Serve(ctx context.Context, webDavURL string, creds auth.Credentials, w http.ResponseWriter, r *http.Request) error {
	if webDavURL == "" {
		return errors.New("stream: empty webDavUrl")
	}

	req, err := http.NewRequestWithContext(ctx, r.Method, webDavURL, nil)
	if err != nil {
		return fmt.Errorf("stream: build request: %w", err)
	}
	if creds.IsBearer() {
		req.Header.Set("Authorization", "Bearer "+creds.BearerToken)
	} else {
		req.SetBasicAuth(creds.Username, creds.Password)
	}

	// Forward the cache/range negotiation headers the client sent.
	// Content-Type is set on the response, not the request.
	for _, h := range []string{"Range", "If-Range", "If-Modified-Since", "If-None-Match"} {
		if v := r.Header.Get(h); v != "" {
			req.Header.Set(h, v)
		}
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("stream: upstream GET: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Copy the headers a media player actually cares about; skip
	// WebDAV-specific ones like DAV:, MS-Author-Via, etc.
	for _, h := range []string{
		"Content-Type", "Content-Length", "Content-Range",
		"Accept-Ranges", "ETag", "Last-Modified",
	} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// For HEAD responses the body is empty; io.Copy on an empty
	// reader is a no-op so no special-case is needed.
	if _, err := io.Copy(w, resp.Body); err != nil {
		// The client disconnected mid-stream — not much we can do,
		// the headers have already been flushed. Return so callers
		// can log at debug level.
		return fmt.Errorf("stream: copy body: %w", err)
	}
	return nil
}
