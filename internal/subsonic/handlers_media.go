package subsonic

import (
	"net/http"

	libregraph "github.com/opencloud-eu/libre-graph-api-go"

	"github.com/opencloud-eu/opencloud-music/internal/auth"
)

// resolveSong looks up a single driveItem by its ID via a minimal
// Graph search. Returns (nil, nil) if no item matched.
//
// OpenCloud's KQL grammar doesn't (yet) expose driveItem IDs as a
// searchable field — the `id:` syntax is parsed but hits zero
// results. We therefore accept a fallback: when `id:<x>` comes back
// empty but x looks like a driveItem resource ID, scan a broader
// audio page and match client-side. That's not scalable but gets us
// playback working until the upstream KQL grammar lands a proper
// `id:` predicate.
func (s *Server) resolveSong(r *http.Request, id string) (*libregraph.DriveItem, error) {
	hits, err := s.graph.SearchHits(r.Context(), "id:"+quote(id), 0, 1)
	if err != nil {
		return nil, err
	}
	if hits != nil && len(hits.Hits) > 0 && hits.Hits[0].Resource != nil {
		return hits.Hits[0].Resource, nil
	}

	// Fallback: scan recent audio hits and match by ID in memory.
	s.logger.Debug().Str("id", id).Msg("resolveSong: id: query returned no hits, falling back to scan")
	scan, err := s.graph.SearchHits(r.Context(), "mediatype:audio", 0, 500)
	if err != nil {
		return nil, err
	}
	for _, h := range scan.Hits {
		if h.Resource != nil && h.Resource.Id != nil && *h.Resource.Id == id {
			return h.Resource, nil
		}
	}
	return nil, nil
}

// Stream fetches the driveItem for the given song ID and proxies its
// WebDAV bytes (with Range passthrough) to the client.
//
// (GET /rest/stream)
func (s *Server) Stream(w http.ResponseWriter, r *http.Request, params StreamParams) {
	creds, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, ErrMissingParam, "u (username) and p (app password) are required")
		return
	}
	if params.Id == "" {
		writeError(w, ErrMissingParam, "id is required")
		return
	}
	item, err := s.resolveSong(r, params.Id)
	if err != nil {
		s.logger.Warn().Err(err).Str("id", params.Id).Msg("stream: lookup failed")
		writeError(w, ErrGeneric, "failed to resolve song")
		return
	}
	if item == nil {
		writeError(w, ErrNotFound, "song not found")
		return
	}
	webDav := driveItemDownloadURL(s.publicBaseURL, item)
	if webDav == "" {
		s.logger.Warn().Str("id", params.Id).Msg("stream: could not derive WebDAV URL from driveItem")
		writeError(w, ErrGeneric, "song has no download URL")
		return
	}
	if err := s.proxy.Serve(r.Context(), webDav, creds.Username, creds.Password, w, r); err != nil {
		s.logger.Debug().Err(err).Str("id", params.Id).Msg("stream: proxy ended with error")
	}
}

// PostStream mirrors Stream for POST clients.
//
// (POST /rest/stream)
func (s *Server) PostStream(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeError(w, ErrGeneric, "could not parse form body")
		return
	}
	s.Stream(w, r, StreamParams{Id: r.PostForm.Get("id")})
}

// Download is the non-transcoded equivalent of Stream. Since we don't
// transcode anyway they share an implementation.
//
// (GET /rest/download)
func (s *Server) Download(w http.ResponseWriter, r *http.Request, params DownloadParams) {
	s.Stream(w, r, StreamParams{Id: params.Id})
}

// PostDownload mirrors Download for POST clients.
//
// (POST /rest/download)
func (s *Server) PostDownload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeError(w, ErrGeneric, "could not parse form body")
		return
	}
	s.Stream(w, r, StreamParams{Id: r.PostForm.Get("id")})
}
