package graph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	libregraph "github.com/opencloud-eu/libre-graph-api-go"

	"github.com/opencloud-eu/opencloud-music/internal/auth"
)

// GetDriveItem looks up a driveItem by its OpenCloud resource ID
// against /graph/v1.0/drives/{driveID}/items/{resourceID}.
//
// The Subsonic song ID we hand to clients is reva's
// storagespace.FormatResourceID output — `<storageID>$<spaceID>!<opaqueID>`.
// The v1.0 driveItem endpoint expects the `driveID` path segment to
// be just the `<storageID>$<spaceID>` prefix, but the `itemID` segment
// must be the full composite resource ID.
//
// libregraph exposes the v1beta1 driveItem endpoint under
// DriveItemApiService, but that endpoint rejects the storageID$spaceID
// composite with "invalid driveID or itemID" — the v1.0 route is the
// one OpenCloud's user-drive service answers, so we issue the request
// by hand rather than through the generated client.
func (c *Client) GetDriveItem(ctx context.Context, resourceID string) (*libregraph.DriveItem, error) {
	driveID, _, ok := splitResourceID(resourceID)
	if !ok {
		return nil, fmt.Errorf("graph: malformed driveItem id: %q", resourceID)
	}
	creds, ok := auth.FromContext(ctx)
	if !ok {
		return nil, errors.New("graph: no credentials on request context")
	}

	path := "/v1.0/drives/" + url.PathEscape(driveID) + "/items/" + url.PathEscape(resourceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	applyAuth(req, creds)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("graph: GET %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("graph: GET %s: %s", path, resp.Status)
	}
	var item libregraph.DriveItem
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, fmt.Errorf("graph: decode %s: %w", path, err)
	}
	return &item, nil
}

func splitResourceID(id string) (drive, item string, ok bool) {
	drive, item, found := strings.Cut(id, "!")
	if !found || drive == "" || item == "" {
		return "", "", false
	}
	return drive, item, true
}

// applyAuth sets the outbound Authorization header based on the
// credential flavour the auth middleware captured — Bearer for OIDC
// access tokens forwarded from the web UI, Basic for app-token auth.
func applyAuth(req *http.Request, creds auth.Credentials) {
	if creds.IsBearer() {
		req.Header.Set("Authorization", "Bearer "+creds.BearerToken)
		return
	}
	req.SetBasicAuth(creds.Username, creds.Password)
}
