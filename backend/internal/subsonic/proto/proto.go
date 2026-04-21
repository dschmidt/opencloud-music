// Package proto holds the Subsonic response-envelope writer and the set
// of protocol error codes. It is kept separate from the parent
// `subsonic` package so that non-handler code paths (notably the auth
// middleware) can emit a Subsonic-formatted failure without depending
// on the handler package — and it's kept separate from the `model`
// sibling so that changing the envelope emission doesn't force a
// regeneration of the spec.
package proto

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/opencloud-eu/opencloud-music/internal/subsonic/model"
)

// Subsonic protocol constants advertised in every response envelope.
// Handlers pass these into the generated `*SuccessResponse` structs;
// exposing them here (rather than tucking them into the handler
// package) lets non-handler code fill in the envelope too.
const (
	APIVersion    = "1.16.1"
	ServerType    = "opencloud-music"
	ServerVersion = "0.1.0"
)

// Subsonic error codes (see
// https://opensubsonic.netlify.app/docs/responses/error/). Every failure
// envelope must carry one of these codes.
const (
	ErrGeneric          = 0
	ErrMissingParam     = 10
	ErrClientVersionOld = 20
	ErrServerVersionOld = 30
	ErrBadCredentials   = 40
	ErrLDAPTokenAuth    = 41
	ErrAuthNotSupported = 42
	ErrConflictingAuth  = 43
	ErrInvalidAPIKey    = 44
	ErrNotAuthorized    = 50
	ErrTrialExpired     = 60
	ErrNotFound         = 70
)

// WriteResponse wraps resp in the Subsonic `{"subsonic-response": …}`
// envelope and emits it as HTTP 200 + application/json. resp should be
// one of the generated `Get<Endpoint>SuccessResponse`,
// `SubsonicSuccessResponse`, or `SubsonicFailureResponse` types —
// those already carry the envelope metadata (status / version / type
// / serverVersion / openSubsonic) alongside the payload field, so
// standard json.Marshal produces the exact shape Subsonic clients
// expect.
//
// Subsonic clients expect HTTP 200 even on protocol errors — WriteError
// funnels through here for the same reason.
func WriteResponse(w http.ResponseWriter, resp any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{"subsonic-response": resp})
}

// WriteError renders a failure envelope with the given Subsonic error
// code and message, using the generated SubsonicFailureResponse /
// SubsonicError types so the shape can't drift from the spec.
func WriteError(w http.ResponseWriter, code int, msg string) {
	WriteResponse(w, model.SubsonicFailureResponse{
		Status:        model.Failed,
		Version:       APIVersion,
		Type:          ServerType,
		ServerVersion: ServerVersion,
		OpenSubsonic:  true,
		Error: model.SubsonicError{
			Code:    model.SubsonicErrorCode(code),
			Message: &msg,
		},
	})
}

// WriteJSONError is used for non-Subsonic HTTP errors (bad Content-Type,
// malformed request) that never reach the envelope layer.
func WriteJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `{"error":%q}`, msg)
}
