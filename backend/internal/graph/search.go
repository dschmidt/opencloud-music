package graph

import (
	"context"
	"fmt"

	libregraph "github.com/opencloud-eu/libre-graph-api-go"
)

// p32 returns a pointer to an int32 literal — libregraph's generated
// API uses *int32 for optional paging fields.
func p32(v int32) *int32 { return &v }

// SearchHits runs a KQL search query against OpenCloud and returns the
// underlying hits container (if any). Callers pass the full query
// string (e.g. `mediatype:audio AND audio.albumArtist:"Pink Floyd"`);
// paging is controlled through `from` and `size`.
func (c *Client) SearchHits(ctx context.Context, query string, from, size int32) (*libregraph.SearchHitsContainer, error) {
	authed, err := c.authCtx(ctx)
	if err != nil {
		return nil, err
	}
	req := libregraph.NewSearchRequest([]string{"driveItem"}, *libregraph.NewSearchQuery(query))
	req.From = p32(from)
	req.Size = p32(size)
	body := libregraph.NewSearchQueryRequest([]libregraph.SearchRequest{*req})

	resp, httpResp, err := c.api.SearchApi.SearchQuery(authed).SearchQueryRequest(*body).Execute()
	if err != nil {
		status := 0
		if httpResp != nil {
			status = httpResp.StatusCode
		}
		return nil, fmt.Errorf("graph: search query %q (http %d): %w", query, status, err)
	}
	if len(resp.Value) == 0 || len(resp.Value[0].HitsContainers) == 0 {
		c.logHits(query, 0)
		return &libregraph.SearchHitsContainer{}, nil
	}
	hc := resp.Value[0].HitsContainers[0]
	c.logHits(query, len(hc.Hits))
	return &hc, nil
}

// SearchAggregateWithOptions is the low-level aggregation entry point.
// Callers pass pre-built libregraph.AggregationOption values, so it's
// the only helper that can express sub-aggregations or custom bucket
// definitions. SearchAggregate (flat terms aggregation) is a thin
// wrapper.
func (c *Client) SearchAggregateWithOptions(ctx context.Context, query string, opts []libregraph.AggregationOption) ([]libregraph.SearchAggregation, error) {
	authed, err := c.authCtx(ctx)
	if err != nil {
		return nil, err
	}
	req := libregraph.NewSearchRequest([]string{"driveItem"}, *libregraph.NewSearchQuery(query))
	req.From = p32(0)
	req.Size = p32(0)
	req.Aggregations = opts
	body := libregraph.NewSearchQueryRequest([]libregraph.SearchRequest{*req})

	resp, httpResp, err := c.api.SearchApi.SearchQuery(authed).SearchQueryRequest(*body).Execute()
	if err != nil {
		status := 0
		if httpResp != nil {
			status = httpResp.StatusCode
		}
		return nil, fmt.Errorf("graph: aggregate %q (http %d): %w", query, status, err)
	}
	if len(resp.Value) == 0 || len(resp.Value[0].HitsContainers) == 0 {
		return nil, nil
	}
	return resp.Value[0].HitsContainers[0].Aggregations, nil
}

// SearchAggregate runs a zero-hit search purely for its aggregation
// buckets. Useful for `getArtists`, `getGenres`, etc. The query is
// typically `mediatype:audio` plus optional filters; callers pass the
// list of fields to aggregate over (e.g. "audio.albumArtist",
// "audio.genre").
func (c *Client) SearchAggregate(ctx context.Context, query string, fields []string, size int32) ([]libregraph.SearchAggregation, error) {
	authed, err := c.authCtx(ctx)
	if err != nil {
		return nil, err
	}
	aggs := make([]libregraph.AggregationOption, 0, len(fields))
	for _, f := range fields {
		opt := libregraph.NewAggregationOption(f)
		opt.Size = p32(size)
		bd := libregraph.NewBucketDefinition("keyAsString")
		desc := false
		bd.IsDescending = &desc
		minCount := int32(1)
		bd.MinimumCount = &minCount
		opt.BucketDefinition = bd
		aggs = append(aggs, *opt)
	}
	req := libregraph.NewSearchRequest([]string{"driveItem"}, *libregraph.NewSearchQuery(query))
	req.From = p32(0)
	req.Size = p32(0)
	req.Aggregations = aggs
	body := libregraph.NewSearchQueryRequest([]libregraph.SearchRequest{*req})

	resp, httpResp, err := c.api.SearchApi.SearchQuery(authed).SearchQueryRequest(*body).Execute()
	if err != nil {
		status := 0
		if httpResp != nil {
			status = httpResp.StatusCode
		}
		return nil, fmt.Errorf("graph: aggregate %q (http %d): %w", query, status, err)
	}
	if len(resp.Value) == 0 || len(resp.Value[0].HitsContainers) == 0 {
		return nil, nil
	}
	out := resp.Value[0].HitsContainers[0].Aggregations
	c.logAggs(query, fields, out)
	return out, nil
}
