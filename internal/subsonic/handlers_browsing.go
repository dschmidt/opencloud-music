package subsonic

import (
	"net/http"
	"sort"
	"strings"
	"unicode"

	libregraph "github.com/opencloud-eu/libre-graph-api-go"

	"github.com/opencloud-eu/opencloud-music/internal/auth"
)

// Every audio query in the library scope is prefixed with this KQL
// snippet — the Graph search endpoint covers all mediatypes otherwise.
const kqlAudio = "mediatype:audio"

// buildAggregation constructs a libregraph.AggregationOption for a
// terms aggregation on `field`, sized `size`, with optional
// sub-aggregations attached. Keeps callers from having to wire the
// bucket-definition / pointer dance by hand every time.
func buildAggregation(field string, size int32, subs ...libregraph.AggregationOption) libregraph.AggregationOption {
	opt := libregraph.NewAggregationOption(field)
	opt.Size = &size
	bd := libregraph.NewBucketDefinition("keyAsString")
	desc := false
	bd.IsDescending = &desc
	minCount := int32(1)
	bd.MinimumCount = &minCount
	opt.BucketDefinition = bd
	if len(subs) > 0 {
		opt.SubAggregations = subs
	}
	return *opt
}

// buildMetric constructs a libregraph.AggregationOption for a scalar
// metric (sum/avg/min/max) over `field`. The result's `value` carries
// the reduced scalar instead of a bucket list.
func buildMetric(field, kind string) libregraph.AggregationOption {
	opt := libregraph.NewAggregationOption(field)
	opt.MetricKind = &kind
	return *opt
}

// requireAuth is the shared guard for every browsing endpoint. Returns
// true when the caller has credentials on context; otherwise emits
// error 10 and returns false.
func (s *Server) requireAuth(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := auth.FromContext(r.Context()); ok {
		return true
	}
	writeError(w, ErrMissingParam, "u (username) and p (app password) are required")
	return false
}

// GetMusicFolders exposes a single, merged logical folder covering every
// drive the user has access to. Subsonic clients treat `musicFolderId`
// as an optional filter; since we merge spaces anyway it is ignored by
// subsequent calls.
//
// (GET /rest/getMusicFolders)
func (s *Server) GetMusicFolders(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	writeOK(w, map[string]any{
		"musicFolders": map[string]any{
			"musicFolder": []map[string]any{
				{"id": 1, "name": "Music"},
			},
		},
	})
}

// PostGetMusicFolders mirrors GetMusicFolders.
//
// (POST /rest/getMusicFolders)
func (s *Server) PostGetMusicFolders(w http.ResponseWriter, r *http.Request) {
	s.GetMusicFolders(w, r)
}

// artistIndexKey returns the letter Subsonic clients group artists
// under. Non-letter leading characters fall into "#".
func artistIndexKey(name string) string {
	for _, r := range name {
		switch {
		case unicode.IsLetter(r):
			return strings.ToUpper(string(r))
		case unicode.IsDigit(r):
			return "#"
		}
	}
	return "#"
}

// GetArtists aggregates all distinct audio.artist values across the
// user's libraries. We intentionally ignore audio.artist: a lot
// of music files don't set it, and merging the two aggregations
// risked miscounts. The Subsonic response is keyed by first letter;
// we compute that index locally.
//
// (GET /rest/getArtists)
func (s *Server) GetArtists(w http.ResponseWriter, r *http.Request, _ GetArtistsParams) {
	if !s.requireAuth(w, r) {
		return
	}
	// One aggregation on audio.artist with nested sub-aggregations
	// for the distinct albums and the sum of track durations. Single
	// Graph round trip produces everything the Subsonic response
	// needs — no N+1, no per-artist follow-up query.
	artistOpt := buildAggregation("audio.artist", 500,
		buildAggregation("audio.album", 500),
		buildMetric("audio.duration", "sum"),
	)
	aggs, err := s.graph.SearchAggregateWithOptions(r.Context(), kqlAudio,
		[]libregraph.AggregationOption{artistOpt})
	if err != nil {
		s.logger.Warn().Err(err).Msg("getArtists: aggregate failed")
		writeError(w, ErrGeneric, "failed to list artists")
		return
	}
	grouped := map[string][]map[string]any{}
	for _, a := range aggs {
		if a.Field == nil || *a.Field != "audio.artist" {
			continue
		}
		for _, b := range a.Buckets {
			name := derefBucket(b.Key)
			if name == "" {
				continue
			}
			albumCount := 0
			durationSeconds := int64(0)
			for _, sub := range b.SubAggregations {
				if sub.Field == nil {
					continue
				}
				switch *sub.Field {
				case "audio.album":
					albumCount = len(sub.Buckets)
				case "audio.duration":
					if sub.Value != nil {
						durationSeconds = int64(*sub.Value / 1000)
					}
				}
			}
			letter := artistIndexKey(name)
			grouped[letter] = append(grouped[letter], map[string]any{
				"id":         artistID(name),
				"name":       name,
				"albumCount": albumCount,
				"coverArt":   artistID(name),
				"duration":   durationSeconds,
			})
		}
	}
	indexes := make([]map[string]any, 0, len(grouped))
	keys := make([]string, 0, len(grouped))
	for k := range grouped {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		artists := grouped[k]
		sort.Slice(artists, func(i, j int) bool {
			return strings.ToLower(artists[i]["name"].(string)) < strings.ToLower(artists[j]["name"].(string))
		})
		indexes = append(indexes, map[string]any{
			"name":   k,
			"artist": artists,
		})
	}
	writeOK(w, map[string]any{
		"artists": map[string]any{
			"ignoredArticles": "",
			"index":           indexes,
		},
	})
}

// PostGetArtists mirrors GetArtists.
//
// (POST /rest/getArtists)
func (s *Server) PostGetArtists(w http.ResponseWriter, r *http.Request) {
	s.GetArtists(w, r, GetArtistsParams{})
}

// GetArtist returns the albums by a single artist. The ID is the
// reversible `ar-<base64>` form produced by getArtists.
//
// (GET /rest/getArtist)
func (s *Server) GetArtist(w http.ResponseWriter, r *http.Request, params GetArtistParams) {
	if !s.requireAuth(w, r) {
		return
	}
	if params.Id == "" {
		writeError(w, ErrMissingParam, "id is required")
		return
	}
	name, err := decodeArtistID(params.Id)
	if err != nil {
		writeError(w, ErrNotFound, "artist not found")
		return
	}
	// Single aggregation: group tracks by audio.album, sum
	// audio.duration per album. No per-track data needed for the
	// artist-detail view — we only render the album list.
	albumOpt := buildAggregation("audio.album", 500,
		buildMetric("audio.duration", "sum"),
	)
	aggs, err := s.graph.SearchAggregateWithOptions(r.Context(),
		kqlAudio+" AND audio.artist:"+quote(name),
		[]libregraph.AggregationOption{albumOpt})
	if err != nil {
		s.logger.Warn().Err(err).Str("artist", name).Msg("getArtist: aggregate failed")
		writeError(w, ErrGeneric, "failed to list albums")
		return
	}
	albums := make([]map[string]any, 0)
	for _, a := range aggs {
		if a.Field == nil || *a.Field != "audio.album" {
			continue
		}
		for _, b := range a.Buckets {
			album := derefBucket(b.Key)
			if album == "" {
				continue
			}
			durationSeconds := int64(0)
			for _, sub := range b.SubAggregations {
				if sub.Field != nil && *sub.Field == "audio.duration" && sub.Value != nil {
					durationSeconds = int64(*sub.Value / 1000)
				}
			}
			albums = append(albums, map[string]any{
				"id":        albumID(name, album),
				"name":      album,
				"title":     album,
				"artist":    name,
				"artistId":  artistID(name),
				"songCount": derefInt64(b.Count),
				"duration":  durationSeconds,
				"coverArt":  albumID(name, album),
			})
		}
	}
	sort.Slice(albums, func(i, j int) bool {
		return strings.ToLower(albums[i]["name"].(string)) < strings.ToLower(albums[j]["name"].(string))
	})
	writeOK(w, map[string]any{
		"artist": map[string]any{
			"id":         params.Id,
			"name":       name,
			"albumCount": len(albums),
			"album":      albums,
			"coverArt":   params.Id,
		},
	})
}

// PostGetArtist mirrors GetArtist.
//
// (POST /rest/getArtist)
func (s *Server) PostGetArtist(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeError(w, ErrGeneric, "could not parse form body")
		return
	}
	s.GetArtist(w, r, GetArtistParams{Id: r.PostForm.Get("id")})
}

// GetAlbum returns every track for a specific (artist, album) pair.
// Duplicates that surface when the same album exists in multiple
// spaces are deduplicated by (disc, track), keeping the highest-
// bitrate copy.
//
// (GET /rest/getAlbum)
func (s *Server) GetAlbum(w http.ResponseWriter, r *http.Request, params GetAlbumParams) {
	if !s.requireAuth(w, r) {
		return
	}
	if params.Id == "" {
		writeError(w, ErrMissingParam, "id is required")
		return
	}
	artist, album, err := decodeAlbumID(params.Id)
	if err != nil {
		writeError(w, ErrNotFound, "album not found")
		return
	}
	query := kqlAudio + ` AND audio.artist:` + quote(artist) + ` AND audio.album:` + quote(album)
	hits, err := s.graph.SearchHits(r.Context(), query, 0, 500)
	if err != nil {
		s.logger.Warn().Err(err).Str("album", album).Msg("getAlbum: search failed")
		writeError(w, ErrGeneric, "failed to list album tracks")
		return
	}

	tracks := dedupeTracks(hits.Hits)
	sort.SliceStable(tracks, func(i, j int) bool { return discTrack(tracks[i]) < discTrack(tracks[j]) })

	songs := make([]map[string]any, 0, len(tracks))
	for _, t := range tracks {
		songs = append(songs, driveItemToSong(t))
	}
	totalSeconds := 0
	for _, t := range tracks {
		if t.Audio != nil && t.Audio.Duration != nil {
			totalSeconds += int(*t.Audio.Duration / 1000)
		}
	}
	payload := map[string]any{
		"id":        params.Id,
		"name":      album,
		"title":     album,
		"artist":    artist,
		"artistId":  artistID(artist),
		"songCount": len(songs),
		"duration":  totalSeconds,
		"song":      songs,
		"coverArt":  params.Id,
	}
	if len(tracks) > 0 && tracks[0].Audio != nil && tracks[0].Audio.Year != nil {
		payload["year"] = *tracks[0].Audio.Year
	}
	if len(tracks) > 0 && tracks[0].Audio != nil && tracks[0].Audio.Genre != nil {
		payload["genre"] = *tracks[0].Audio.Genre
	}
	writeOK(w, map[string]any{"album": payload})
}

// PostGetAlbum mirrors GetAlbum.
//
// (POST /rest/getAlbum)
func (s *Server) PostGetAlbum(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeError(w, ErrGeneric, "could not parse form body")
		return
	}
	s.GetAlbum(w, r, GetAlbumParams{Id: r.PostForm.Get("id")})
}

// GetGenres aggregates all distinct audio.genre values and surfaces
// per-genre track count, distinct album count, and total runtime in
// a single Graph round trip.
//
// (GET /rest/getGenres)
func (s *Server) GetGenres(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	genreOpt := buildAggregation("audio.genre", 500,
		buildAggregation("audio.album", 500),
		buildMetric("audio.duration", "sum"),
	)
	aggs, err := s.graph.SearchAggregateWithOptions(r.Context(), kqlAudio,
		[]libregraph.AggregationOption{genreOpt})
	if err != nil {
		s.logger.Warn().Err(err).Msg("getGenres: aggregate failed")
		writeError(w, ErrGeneric, "failed to list genres")
		return
	}
	var genres []map[string]any
	for _, a := range aggs {
		if a.Field == nil || *a.Field != "audio.genre" {
			continue
		}
		for _, b := range a.Buckets {
			name := derefBucket(b.Key)
			if name == "" {
				continue
			}
			albumCount := 0
			durationSeconds := int64(0)
			for _, sub := range b.SubAggregations {
				if sub.Field == nil {
					continue
				}
				switch *sub.Field {
				case "audio.album":
					albumCount = len(sub.Buckets)
				case "audio.duration":
					if sub.Value != nil {
						durationSeconds = int64(*sub.Value / 1000)
					}
				}
			}
			genres = append(genres, map[string]any{
				"value":      name,
				"songCount":  derefInt64(b.Count),
				"albumCount": albumCount,
				"duration":   durationSeconds,
			})
		}
	}
	writeOK(w, map[string]any{
		"genres": map[string]any{"genre": genres},
	})
}

// PostGetGenres mirrors GetGenres.
//
// (POST /rest/getGenres)
func (s *Server) PostGetGenres(w http.ResponseWriter, r *http.Request) {
	s.GetGenres(w, r)
}

// dedupeTracks removes duplicate driveItems with the same (disc,
// track) position, preferring higher bitrate — or, failing that, the
// first hit returned by the server.
func dedupeTracks(hits []libregraph.SearchHit) []*libregraph.DriveItem {
	type entry struct {
		item    *libregraph.DriveItem
		bitrate int64
	}
	best := map[int]entry{}
	for _, h := range hits {
		if h.Resource == nil {
			continue
		}
		key := discTrack(h.Resource)
		br := int64(0)
		if h.Resource.Audio != nil && h.Resource.Audio.Bitrate != nil {
			br = *h.Resource.Audio.Bitrate
		}
		prev, ok := best[key]
		if !ok || br > prev.bitrate {
			best[key] = entry{item: h.Resource, bitrate: br}
		}
	}
	out := make([]*libregraph.DriveItem, 0, len(best))
	for _, e := range best {
		out = append(out, e.item)
	}
	return out
}

func derefBucket(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// GetSong looks up a single track by its resource ID and returns the
// Subsonic song shape (see convert.go for field mapping).
//
// (GET /rest/getSong)
func (s *Server) GetSong(w http.ResponseWriter, r *http.Request, params GetSongParams) {
	if !s.requireAuth(w, r) {
		return
	}
	if params.Id == "" {
		writeError(w, ErrMissingParam, "id is required")
		return
	}
	item, err := s.resolveSong(r, params.Id)
	if err != nil {
		s.logger.Warn().Err(err).Str("id", params.Id).Msg("getSong: lookup failed")
		writeError(w, ErrGeneric, "failed to resolve song")
		return
	}
	if item == nil {
		writeError(w, ErrNotFound, "song not found")
		return
	}
	writeOK(w, map[string]any{"song": driveItemToSong(item)})
}

// PostGetSong mirrors GetSong for POST clients.
//
// (POST /rest/getSong)
func (s *Server) PostGetSong(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeError(w, ErrGeneric, "could not parse form body")
		return
	}
	s.GetSong(w, r, GetSongParams{Id: r.PostForm.Get("id")})
}
