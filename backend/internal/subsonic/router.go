package subsonic

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/opencloud-eu/opencloud-music/internal/subsonic/model"
	"github.com/opencloud-eu/opencloud-music/internal/subsonic/proto"
)

// Mount registers every /rest/* Subsonic route against the given chi
// router. The classic Subsonic servlet convention exposes endpoints
// with a `.view` suffix (e.g. `/rest/ping.view`); clients like
// Substreamer still rely on that form, so we normalise the path with
// stripViewSuffix before handing off to the generated chi-server.
//
// The generated chi-server leaves the global NotFound handler at chi's
// default (plain-text "404 page not found") — fine for a JSON API
// behind a browser, but Subsonic clients parse every response as JSON
// and fail on the first unrecognised endpoint. We install our own
// NotFound handler that emits a Subsonic failure envelope (code 70)
// so client logs stay intact.
func Mount(r chi.Router, s *Server) {
	r.Use(stripViewSuffix)
	r.Use(fillSearch3Query)
	model.HandlerFromMux(s, r)
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		proto.WriteError(w, proto.ErrNotFound, "no such endpoint: "+req.URL.Path)
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		proto.WriteError(w, proto.ErrGeneric, "method not allowed for "+req.URL.Path)
	})
}

// fillSearch3Query tolerates Subsonic clients that omit the required
// `query` parameter on /rest/search3 (or send it empty). The spec
// marks it required but real clients — Substreamer notably — send
// either no `query` at all or `query=` until the user types
// something. oapi-codegen rejects both with a plain-text 400 ("Query
// argument query is required, but not found") before our handler
// runs; Substreamer then tries to parse that as JSON and trips on the
// leading `Q`. We inject `""` as a sentinel value that passes the
// generator's non-empty check, and Search3 recognises the sentinel
// and returns empty result buckets.
func fillSearch3Query(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/rest/search3") {
			next.ServeHTTP(w, r)
			return
		}
		q := r.URL.Query()
		if q.Get("query") != "" {
			next.ServeHTTP(w, r)
			return
		}
		q.Set("query", `""`)
		newURL := *r.URL
		newURL.RawQuery = q.Encode()
		clone := r.Clone(r.Context())
		clone.URL = &newURL
		next.ServeHTTP(w, clone)
	})
}

// stripViewSuffix rewrites `/rest/<endpoint>.view` → `/rest/<endpoint>`
// so the same handlers serve both the modern and the legacy URL shape.
// Only paths under `/rest/` are rewritten.
func stripViewSuffix(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !(strings.HasPrefix(r.URL.Path, "/rest/") && strings.HasSuffix(r.URL.Path, ".view")) {
			next.ServeHTTP(w, r)
			return
		}
		newURL := *r.URL
		newURL.Path = strings.TrimSuffix(r.URL.Path, ".view")
		if r.URL.RawPath != "" {
			newURL.RawPath = strings.TrimSuffix(r.URL.RawPath, ".view")
		}
		clone := r.Clone(r.Context())
		clone.URL = &newURL
		next.ServeHTTP(w, clone)
	})
}
