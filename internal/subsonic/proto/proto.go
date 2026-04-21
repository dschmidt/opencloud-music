// Package proto holds the Subsonic response-envelope writer and the set
// of protocol error codes. It is kept separate from the parent
// `subsonic` package so that non-handler code paths (notably the auth
// middleware) can emit a Subsonic-formatted failure without taking a
// dependency on the generated types that live alongside the handlers.
//
// We deliberately redeclare the envelope shape here rather than
// importing the oapi-codegen-produced types: those live in the parent
// `subsonic` package (so importing them would re-create the cycle this
// split was designed to avoid), and the envelope we write is
// intentionally a minimal subset of the generated SubsonicResponse
// anyway.
package proto

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
)

// Subsonic protocol constants advertised in every response envelope.
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

// base is the set of metadata fields every Subsonic response carries. It
// is composed into success and failure envelopes via JSON inlining.
type base struct {
	Status        string `json:"status"`
	Version       string `json:"version"`
	Type          string `json:"type"`
	ServerVersion string `json:"serverVersion"`
	OpenSubsonic  bool   `json:"openSubsonic"`
}

func baseOK() base {
	return base{
		Status:        "ok",
		Version:       APIVersion,
		Type:          ServerType,
		ServerVersion: ServerVersion,
		OpenSubsonic:  true,
	}
}

func baseFailed() base {
	b := baseOK()
	b.Status = "failed"
	return b
}

// envelope wraps an OK response payload under the `subsonic-response` key
// and inlines the payload so its keys sit as siblings of the metadata
// fields (status, version, type, …).
type envelope struct {
	Body baseBody `json:"subsonic-response"`
}

type baseBody struct {
	base
	payload map[string]any
}

// MarshalJSON merges the base fields and the opaque payload into a single
// JSON object. The resulting shape is
//
//	{"status":"ok","version":"1.16.1",…,"<payloadKey>":{…}}
//
// so individual endpoints only have to hand us a map of their
// endpoint-specific keys.
func (b baseBody) MarshalJSON() ([]byte, error) {
	merged := map[string]any{
		"status":        b.Status,
		"version":       b.Version,
		"type":          b.Type,
		"serverVersion": b.ServerVersion,
		"openSubsonic":  b.OpenSubsonic,
	}
	maps.Copy(merged, b.payload)
	return json.Marshal(merged)
}

// WriteOK renders a success envelope. payload may be nil for endpoints
// that only need to signal success (e.g. ping, scrobble).
func WriteOK(w http.ResponseWriter, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(envelope{
		Body: baseBody{base: baseOK(), payload: payload},
	})
}

// WriteError renders a failure envelope. Subsonic clients expect HTTP
// 200 even on protocol errors (they read the `status` field).
func WriteError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(envelope{
		Body: baseBody{
			base: baseFailed(),
			payload: map[string]any{
				"error": map[string]any{
					"code":    code,
					"message": msg,
				},
			},
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
