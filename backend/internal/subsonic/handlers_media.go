package subsonic

import (
	"net/http"

	libregraph "github.com/opencloud-eu/libre-graph-api-go"

	"github.com/opencloud-eu/opencloud-music/internal/auth"
	"github.com/opencloud-eu/opencloud-music/internal/subsonic/model"
	"github.com/opencloud-eu/opencloud-music/internal/subsonic/proto"
)

// resolveSong fetches the driveItem for a Subsonic song ID.
func (s *Server) resolveSong(r *http.Request, id string) (*libregraph.DriveItem, error) {
	return s.graph.GetDriveItem(r.Context(), id)
}

// Stream fetches the driveItem for the given song ID and proxies its
// WebDAV bytes (with Range passthrough) to the client.
//
// (GET /rest/stream)
func (s *Server) Stream(w http.ResponseWriter, r *http.Request, params model.StreamParams) {
	creds, ok := auth.FromContext(r.Context())
	if !ok {
		proto.WriteError(w, proto.ErrMissingParam, "u (username) and p (app token) are required")
		return
	}
	if params.Id == "" {
		proto.WriteError(w, proto.ErrMissingParam, "id is required")
		return
	}
	item, err := s.resolveSong(r, params.Id)
	if err != nil {
		s.logger.Warn().Err(err).Str("id", params.Id).Msg("stream: lookup failed")
		proto.WriteError(w, proto.ErrGeneric, "failed to resolve song")
		return
	}
	if item == nil {
		proto.WriteError(w, proto.ErrNotFound, "song not found")
		return
	}
	webDav := driveItemDownloadURL(s.publicBaseURL, item)
	if webDav == "" {
		s.logger.Warn().Str("id", params.Id).Msg("stream: could not derive WebDAV URL from driveItem")
		proto.WriteError(w, proto.ErrGeneric, "song has no download URL")
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
		proto.WriteError(w, proto.ErrGeneric, "could not parse form body")
		return
	}
	s.Stream(w, r, model.StreamParams{Id: r.PostForm.Get("id")})
}

// Download is the non-transcoded equivalent of Stream. Since we don't
// transcode anyway they share an implementation.
//
// (GET /rest/download)
func (s *Server) Download(w http.ResponseWriter, r *http.Request, params model.DownloadParams) {
	s.Stream(w, r, model.StreamParams{Id: params.Id})
}

// PostDownload mirrors Download for POST clients.
//
// (POST /rest/download)
func (s *Server) PostDownload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		proto.WriteError(w, proto.ErrGeneric, "could not parse form body")
		return
	}
	s.Stream(w, r, model.StreamParams{Id: r.PostForm.Get("id")})
}
