package subsonic

import (
	"strings"

	"github.com/opencloud-eu/opencloud/pkg/log"

	"github.com/opencloud-eu/opencloud-music/internal/graph"
	"github.com/opencloud-eu/opencloud-music/internal/stream"
	"github.com/opencloud-eu/opencloud-music/internal/subsonic/model"
)

// Server implements the generated ServerInterface. It embeds
// model.Unimplemented so any endpoint the music service does not
// explicitly handle returns 501 via oapi-codegen's default stubs. As
// MVP coverage grows, endpoints are promoted into their own methods
// (handlers_*.go) which shadow the embedded stubs.
type Server struct {
	model.Unimplemented

	logger        log.Logger
	graph         *graph.Client
	proxy         *stream.Proxier
	publicBaseURL string
}

// NewServer constructs a Subsonic server handler.
func NewServer(logger log.Logger, g *graph.Client, p *stream.Proxier) *Server {
	return &Server{logger: logger, graph: g, proxy: p}
}

// quote escapes a value for inclusion in a KQL phrase query. OpenCloud's
// KQL parser expects double-quoted strings with `"` and `\` doubled.
func quote(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + r.Replace(s) + `"`
}

// publicBaseURL is the URL prefix we build WebDAV URLs against. Stored
// on the Server so handlers can reach it without reaching into the
// config.
func (s *Server) SetPublicBaseURL(u string) {
	s.publicBaseURL = strings.TrimRight(u, "/")
}
