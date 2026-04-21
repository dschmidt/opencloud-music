// Package graph wraps the generated libre-graph-api-go client so music
// handlers can talk to OpenCloud's Graph API without knowing the
// underlying HTTP plumbing. The wrapper takes care of:
//
//   - Building a libregraph.APIClient pointed at the OpenCloud instance
//     (URL + TLS skip for self-signed dev certs).
//   - Attaching the per-request OpenCloud app token (coming from the
//     auth middleware) as the Bearer credential for each outgoing call
//     via libregraph.ContextAccessToken.
//
// Higher-level helpers (SearchAggregate, SearchHits, GetMe) live in
// their own files and build on Client.
package graph

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/url"
	"strings"

	libregraph "github.com/opencloud-eu/libre-graph-api-go"
	"github.com/opencloud-eu/opencloud/pkg/log"

	"github.com/opencloud-eu/opencloud-music/internal/auth"
)

// Client bundles a libregraph.APIClient with the base URL it talks to so
// handlers can run Graph calls without touching the underlying config.
type Client struct {
	api *libregraph.APIClient
	log log.Logger
}

// New constructs a Graph client that issues requests against baseURL
// (e.g. https://localhost:9200). The /graph suffix is appended so the
// libregraph routes (/graph/v1.0/me, /graph/v1beta1/search/query, ...)
// resolve correctly. insecure toggles TLS verification for self-signed
// dev certificates.
func New(baseURL string, insecure bool) (*Client, error) {
	if baseURL == "" {
		return nil, errors.New("graph: baseURL is required")
	}
	u, err := url.Parse(strings.TrimRight(baseURL, "/") + "/graph")
	if err != nil {
		return nil, err
	}

	cfg := libregraph.NewConfiguration()
	cfg.Servers = libregraph.ServerConfigurations{{URL: u.String()}}
	if insecure {
		cfg.HTTPClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // opt-in for dev self-signed
			},
		}
	}
	return &Client{api: libregraph.NewAPIClient(cfg)}, nil
}

// authCtx returns a context that carries the current request's
// (username, token) pair in the form libregraph expects. OpenCloud's
// Graph endpoints accept app tokens via HTTP Basic Auth exactly the
// same way they accept a regular password — the app token simply
// substitutes for the user's primary password.
func (c *Client) authCtx(ctx context.Context) (context.Context, error) {
	creds, ok := auth.FromContext(ctx)
	if !ok {
		return nil, errors.New("graph: no credentials on request context (auth middleware missing?)")
	}
	return context.WithValue(ctx, libregraph.ContextBasicAuth, libregraph.BasicAuth{
		UserName: creds.Username,
		Password: creds.Password,
	}), nil
}

// API exposes the underlying libregraph client for helpers that need
// direct access to a typed API service (e.g. graph.SearchAggregate).
func (c *Client) API() *libregraph.APIClient {
	return c.api
}

// SetLogger wires a zerolog logger into the graph client so its
// helpers can emit structured debug output (query strings, hit counts,
// aggregation bucket counts). Call once during server wiring.
func (c *Client) SetLogger(l log.Logger) { c.log = l }

func (c *Client) logHits(query string, n int) {
	c.log.Debug().Str("query", query).Int("hits", n).Msg("graph search")
}

func (c *Client) logAggs(query string, fields []string, aggs []libregraph.SearchAggregation) {
	total := 0
	for _, a := range aggs {
		total += len(a.Buckets)
	}
	c.log.Debug().
		Str("query", query).
		Str("fields", strings.Join(fields, ",")).
		Int("buckets", total).
		Msg("graph aggregate")
}
