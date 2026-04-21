package subsonic

import (
	"net/http"

	"github.com/opencloud-eu/opencloud-music/internal/subsonic/proto"
)

// Ping is the canonical Subsonic health/auth probe.
//
// (GET /rest/ping)
func (s *Server) Ping(w http.ResponseWriter, _ *http.Request) {
	proto.WriteOK(w, nil)
}

// PostPing mirrors Ping for POST clients.
//
// (POST /rest/ping)
func (s *Server) PostPing(w http.ResponseWriter, _ *http.Request) {
	proto.WriteOK(w, nil)
}

// GetLicense always reports a valid license — OpenCloud is free software,
// there is no trial period to honour. Navidrome and gonic behave the
// same way.
//
// (GET /rest/getLicense)
func (s *Server) GetLicense(w http.ResponseWriter, _ *http.Request) {
	proto.WriteOK(w, map[string]any{
		"license": License{Valid: true},
	})
}

// PostGetLicense mirrors GetLicense for POST clients.
//
// (POST /rest/getLicense)
func (s *Server) PostGetLicense(w http.ResponseWriter, _ *http.Request) {
	s.GetLicense(w, nil)
}

// openSubsonicExtensions is the static list of OpenSubsonic extensions
// this server supports.
//
// We intentionally DO NOT advertise `apiKeyAuthentication`: that
// extension promises a single opaque token that identifies both the
// user and the credential, but OpenCloud's app tokens are HTTP
// Basic-Auth passwords — they always need a companion username. The
// required auth flow is therefore classic Subsonic `u` + `p`, where
// `p` carries the OpenCloud app token (encoded with `enc:<hex>`
// if your client defaults to that form).
//
// See https://opensubsonic.netlify.app/docs/extensions/
var openSubsonicExtensions = []OpenSubsonicExtension{}

// GetOpenSubsonicExtensions advertises OpenSubsonic API v1 support and
// the extension set above.
//
// (GET /rest/getOpenSubsonicExtensions)
func (s *Server) GetOpenSubsonicExtensions(w http.ResponseWriter, _ *http.Request) {
	proto.WriteOK(w, map[string]any{
		"openSubsonicExtensions": openSubsonicExtensions,
	})
}

// PostGetOpenSubsonicExtensions mirrors GetOpenSubsonicExtensions.
//
// (POST /rest/getOpenSubsonicExtensions)
func (s *Server) PostGetOpenSubsonicExtensions(w http.ResponseWriter, _ *http.Request) {
	s.GetOpenSubsonicExtensions(w, nil)
}
