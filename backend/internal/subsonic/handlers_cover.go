package subsonic

import (
	"context"
	"net/http"
	"strings"

	libregraph "github.com/opencloud-eu/libre-graph-api-go"

	"github.com/opencloud-eu/opencloud-music/internal/auth"
	"github.com/opencloud-eu/opencloud-music/internal/subsonic/model"
	"github.com/opencloud-eu/opencloud-music/internal/subsonic/proto"
)

// coverPreviewDefaultSize is the thumbnail edge we request from
// OpenCloud when the client doesn't supply one. 256px lines up with
// most Subsonic clients' default list-row heights at @2x.
const coverPreviewDefaultSize = 256

// GetCoverArt proxies an embedded cover-art JPEG from OpenCloud's
// preview service. The given ID can be:
//
//   - a raw driveItem resource ID (songs) — used directly,
//   - `ar-<base64>` (artist)             — resolved via a single
//     audio.artist search,
//   - `al-<base64>` (album)              — resolved via an
//     audio.artist + audio.album search (with the same scan
//     fallback as getAlbum).
//
// When no source item can be found (or it has no embedded art), the
// handler returns a Subsonic 70 ("not found"). Subsonic clients treat
// that as "no cover" and render their own placeholder.
//
// (GET /rest/getCoverArt)
func (s *Server) GetCoverArt(w http.ResponseWriter, r *http.Request, params model.GetCoverArtParams) {
	creds, ok := auth.FromContext(r.Context())
	if !ok {
		proto.WriteError(w, proto.ErrMissingParam, "u (username) and p (app token) are required")
		return
	}
	if params.Id == "" {
		proto.WriteError(w, proto.ErrMissingParam, "id is required")
		return
	}
	size := coverPreviewDefaultSize
	if params.Size != nil && *params.Size > 0 {
		size = *params.Size
	}

	item, err := s.resolveCoverSource(r.Context(), params.Id)
	if err != nil {
		s.logger.Warn().Err(err).Str("id", params.Id).Msg("getCoverArt: resolution failed")
		proto.WriteError(w, proto.ErrGeneric, "failed to resolve cover art")
		return
	}
	if item == nil {
		proto.WriteError(w, proto.ErrNotFound, "no cover art for id")
		return
	}

	previewURL := driveItemPreviewURL(s.publicBaseURL, item, size)
	if previewURL == "" {
		proto.WriteError(w, proto.ErrNotFound, "no cover art for id")
		return
	}
	if err := s.proxy.Serve(r.Context(), previewURL, creds.Username, creds.Password, w, r); err != nil {
		s.logger.Debug().Err(err).Str("id", params.Id).Msg("getCoverArt: proxy ended with error")
	}
}

// PostGetCoverArt mirrors GetCoverArt.
//
// (POST /rest/getCoverArt)
func (s *Server) PostGetCoverArt(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		proto.WriteError(w, proto.ErrGeneric, "could not parse form body")
		return
	}
	p := model.GetCoverArtParams{Id: r.PostForm.Get("id")}
	s.GetCoverArt(w, r, p)
}

// resolveCoverSource finds a DriveItem whose embedded cover art can
// stand in for the given Subsonic ID.
func (s *Server) resolveCoverSource(ctx context.Context, id string) (*libregraph.DriveItem, error) {
	switch {
	case strings.HasPrefix(id, artistIDPrefix):
		name, err := decodeArtistID(id)
		if err != nil {
			return nil, err
		}
		hits, err := s.graph.SearchHits(ctx, kqlAudio+" AND audio.artist:"+quote(name), 0, 1)
		if err != nil {
			return nil, err
		}
		return firstResource(hits), nil

	case strings.HasPrefix(id, albumIDPrefix):
		artist, album, err := decodeAlbumID(id)
		if err != nil {
			return nil, err
		}
		hits, err := s.graph.SearchHits(ctx,
			kqlAudio+" AND audio.artist:"+quote(artist)+" AND audio.album:"+quote(album), 0, 1)
		if err != nil {
			return nil, err
		}
		if res := firstResource(hits); res != nil {
			return res, nil
		}
		// Fall back to a wider scan if the compound query misbehaves
		// — same pattern as getAlbum.
		scan, err := s.graph.SearchHits(ctx, kqlAudio+" AND audio.artist:"+quote(artist), 0, 500)
		if err != nil {
			return nil, err
		}
		for _, h := range scan.Hits {
			if h.Resource == nil || h.Resource.Audio == nil {
				continue
			}
			if h.Resource.Audio.Album != nil && *h.Resource.Audio.Album == album {
				return h.Resource, nil
			}
		}
		return nil, nil

	default:
		// Assume a raw driveItem ID (song).
		return s.resolveSongByID(ctx, id)
	}
}

// resolveSongByID fetches the driveItem for a Subsonic song ID.
// Shared between stream/download and cover art.
func (s *Server) resolveSongByID(ctx context.Context, id string) (*libregraph.DriveItem, error) {
	return s.graph.GetDriveItem(ctx, id)
}

func firstResource(c *libregraph.SearchHitsContainer) *libregraph.DriveItem {
	if c == nil || len(c.Hits) == 0 {
		return nil
	}
	return c.Hits[0].Resource
}
