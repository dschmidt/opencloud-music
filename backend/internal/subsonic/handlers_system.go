package subsonic

import (
	"net/http"

	"github.com/opencloud-eu/opencloud-music/internal/subsonic/model"
	"github.com/opencloud-eu/opencloud-music/internal/subsonic/proto"
)

// okEnvelope populates the five envelope metadata fields that every
// OpenSubsonic success response carries. Endpoints with no payload
// (ping, scrobble, star, …) return this directly; endpoints with a
// payload embed these fields into their per-endpoint *SuccessResponse
// type.
func okEnvelope() model.SubsonicSuccessResponse {
	return model.SubsonicSuccessResponse{
		Status:        model.SubsonicSuccessResponseStatusOk,
		Version:       proto.APIVersion,
		Type:          proto.ServerType,
		ServerVersion: proto.ServerVersion,
		OpenSubsonic:  true,
	}
}

// Ping is the canonical Subsonic health/auth probe.
//
// (GET /rest/ping)
func (s *Server) Ping(w http.ResponseWriter, _ *http.Request) {
	proto.WriteResponse(w, okEnvelope())
}

// PostPing mirrors Ping for POST clients.
//
// (POST /rest/ping)
func (s *Server) PostPing(w http.ResponseWriter, _ *http.Request) {
	proto.WriteResponse(w, okEnvelope())
}

// GetLicense always reports a valid license — OpenCloud is free software,
// there is no trial period to honour. Navidrome and gonic behave the
// same way.
//
// (GET /rest/getLicense)
func (s *Server) GetLicense(w http.ResponseWriter, _ *http.Request) {
	proto.WriteResponse(w, model.GetLicenseSuccessResponse{
		Status:        model.GetLicenseSuccessResponseStatusOk,
		Version:       proto.APIVersion,
		Type:          proto.ServerType,
		ServerVersion: proto.ServerVersion,
		OpenSubsonic:  true,
		License:       model.License{Valid: true},
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
var openSubsonicExtensions = []model.OpenSubsonicExtension{}

// GetOpenSubsonicExtensions advertises OpenSubsonic API v1 support and
// the extension set above.
//
// (GET /rest/getOpenSubsonicExtensions)
func (s *Server) GetOpenSubsonicExtensions(w http.ResponseWriter, _ *http.Request) {
	proto.WriteResponse(w, model.GetOpenSubsonicExtensionsSuccessResponse{
		Status:                 model.GetOpenSubsonicExtensionsSuccessResponseStatusOk,
		Version:                proto.APIVersion,
		Type:                   proto.ServerType,
		ServerVersion:          proto.ServerVersion,
		OpenSubsonic:           true,
		OpenSubsonicExtensions: openSubsonicExtensions,
	})
}

// PostGetOpenSubsonicExtensions mirrors GetOpenSubsonicExtensions.
//
// (POST /rest/getOpenSubsonicExtensions)
func (s *Server) PostGetOpenSubsonicExtensions(w http.ResponseWriter, _ *http.Request) {
	s.GetOpenSubsonicExtensions(w, nil)
}
