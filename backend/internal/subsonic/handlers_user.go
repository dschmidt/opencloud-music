package subsonic

import (
	"net/http"

	"github.com/opencloud-eu/opencloud-music/internal/auth"
	"github.com/opencloud-eu/opencloud-music/internal/subsonic/proto"
)

// TokenInfo is part of the OpenSubsonic apiKeyAuthentication extension,
// which we DO NOT advertise (see handlers_system.go). It is kept here
// so existing clients that still probe it get a well-formed envelope
// confirming their username rather than a 501 from the generated
// Unimplemented stub.
//
// (GET /rest/tokenInfo)
func (s *Server) TokenInfo(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.FromContext(r.Context()); !ok {
		proto.WriteError(w, proto.ErrMissingParam, "u (username) and p (app token) are required")
		return
	}
	user, err := s.graph.GetMe(r.Context())
	if err != nil {
		s.logger.Warn().Err(err).Msg("tokenInfo: GetMe failed")
		proto.WriteError(w, proto.ErrBadCredentials, "invalid credentials")
		return
	}
	proto.WriteOK(w, map[string]any{
		"tokenInfo": TokenInfo{Username: user.OnPremisesSamAccountName},
	})
}

// PostTokenInfo mirrors TokenInfo for POST clients.
//
// (POST /rest/tokenInfo)
func (s *Server) PostTokenInfo(w http.ResponseWriter, r *http.Request) {
	s.TokenInfo(w, r)
}

// GetUser returns a minimal user descriptor. Subsonic clients use this
// to decide which features to surface (stream, download, share, …).
// Flags that depend on server state we don't manage yet (coverArtRole,
// playlistRole, scrobblingEnabled, …) are kept conservative for the
// MVP; tighten or loosen them as features land.
//
// (GET /rest/getUser)
func (s *Server) GetUser(w http.ResponseWriter, r *http.Request, params GetUserParams) {
	if _, ok := auth.FromContext(r.Context()); !ok {
		proto.WriteError(w, proto.ErrMissingParam, "u (username) and p (app token) are required")
		return
	}
	user, err := s.graph.GetMe(r.Context())
	if err != nil {
		s.logger.Warn().Err(err).Msg("getUser: GetMe failed")
		proto.WriteError(w, proto.ErrBadCredentials, "invalid credentials")
		return
	}
	// Subsonic's getUser lets admins look up other users. We only
	// honour lookups of the caller themselves — anything else is
	// answered with 50 ("not authorized") rather than trying to map
	// Subsonic-admin onto OpenCloud ACLs.
	if params.Username != "" && params.Username != user.OnPremisesSamAccountName {
		proto.WriteError(w, proto.ErrNotAuthorized, "only self-lookup supported")
		return
	}
	proto.WriteOK(w, map[string]any{
		"user": User{
			Username:          user.OnPremisesSamAccountName,
			AdminRole:         false,
			SettingsRole:      true,
			StreamRole:        true,
			DownloadRole:      true,
			ShareRole:         false,
			PlaylistRole:      false,
			CoverArtRole:      false,
			CommentRole:       false,
			PodcastRole:       false,
			JukeboxRole:       false,
			UploadRole:        false,
			ScrobblingEnabled: false,
		},
	})
}

// PostGetUser mirrors GetUser for POST clients.
//
// (POST /rest/getUser)
func (s *Server) PostGetUser(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err == nil {
		s.GetUser(w, r, GetUserParams{Username: r.PostForm.Get("username")})
		return
	}
	s.GetUser(w, r, GetUserParams{})
}
