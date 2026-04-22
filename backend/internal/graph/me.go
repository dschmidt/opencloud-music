package graph

import (
	"context"
	"fmt"

	libregraph "github.com/opencloud-eu/libre-graph-api-go"
)

// GetMe resolves the current user via GET /graph/v1.0/me. The app
// token from the request context is used as the Bearer credential.
func (c *Client) GetMe(ctx context.Context) (*libregraph.User, error) {
	authed, err := c.authCtx(ctx)
	if err != nil {
		return nil, err
	}
	user, _, err := c.api.MeUserAPI.GetOwnUser(authed).Execute()
	if err != nil {
		return nil, fmt.Errorf("graph: GET /me: %w", err)
	}
	return user, nil
}
