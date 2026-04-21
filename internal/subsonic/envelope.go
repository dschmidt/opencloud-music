package subsonic

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
)

// Subsonic protocol constants advertised in every response envelope.
const (
	SubsonicAPIVersion = "1.16.1"
	ServerType         = "opencloud-music"
	ServerVersion      = "0.1.0"
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
		Version:       SubsonicAPIVersion,
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

// writeOK renders a success envelope. payload may be nil for endpoints
// that only need to signal success (e.g. ping, scrobble).
func writeOK(w http.ResponseWriter, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(envelope{
		Body: baseBody{base: baseOK(), payload: payload},
	})
}

// writeError renders a failure envelope. Subsonic clients expect HTTP
// 200 even on protocol errors (they read the `status` field).
func writeError(w http.ResponseWriter, code int, msg string) {
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

// writeJSONError is used for non-Subsonic HTTP errors (bad Content-Type,
// malformed request) that never reach the envelope layer.
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `{"error":%q}`, msg)
}
