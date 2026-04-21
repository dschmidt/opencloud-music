package subsonic

import (
	"context"
	"math/rand/v2"
	"net/http"
	"sort"
	"strconv"
	"strings"

	libregraph "github.com/opencloud-eu/libre-graph-api-go"

	"github.com/opencloud-eu/opencloud-music/internal/subsonic/model"
	"github.com/opencloud-eu/opencloud-music/internal/subsonic/proto"
)

// albumEntry builds the generated AlbumID3 shape from the fields we
// have available from a terms/aggregation response. Fields not
// derivable from aggregations (Created, DiscTitles, …) stay at their
// zero values — clients tolerate that.
func albumEntry(artist, album string, songCount int64, durationSeconds int64) model.AlbumID3 {
	id := albumID(artist, album)
	return model.AlbumID3{
		Id:        id,
		Name:      album,
		Artist:    ptr(artist),
		ArtistId:  ptr(artistID(artist)),
		SongCount: int(songCount),
		CoverArt:  ptr(id),
		Duration:  int(durationSeconds),
		// Created is required by the spec but aggregations don't carry
		// it; leave at zero-value (clients tolerate 0001-01-01).
	}
}

// GetAlbumList2 implements a subset of the `type` values:
//
//   - newest / recent: ordered by driveItem mtime. Uses a hits scan
//     because mtime isn't an aggregation dimension.
//   - random / alphabeticalByName / alphabeticalByArtist / byGenre /
//     byYear: computed from a single nested aggregation
//     (audio.artist → audio.album → sum(audio.duration)), no per-track
//     data fetched.
//
// Everything else returns an empty list rather than an error so
// clients don't flag the server as broken.
//
// (GET /rest/getAlbumList2)
func (s *Server) GetAlbumList2(w http.ResponseWriter, r *http.Request, params model.GetAlbumList2Params) {
	if !s.requireAuth(w, r) {
		return
	}
	size := 20
	if params.Size != nil && *params.Size > 0 {
		size = *params.Size
	}
	if size > 500 {
		size = 500
	}
	offset := 0
	if params.Offset != nil && *params.Offset > 0 {
		offset = *params.Offset
	}

	query := kqlAudio
	switch string(params.Type) {
	case "byGenre":
		if params.Genre == nil || *params.Genre == "" {
			proto.WriteError(w, proto.ErrMissingParam, "genre is required for byGenre")
			return
		}
		query += " AND audio.genre:" + quote(*params.Genre)
	case "byYear":
		if params.FromYear == nil || params.ToYear == nil {
			proto.WriteError(w, proto.ErrMissingParam, "fromYear and toYear are required for byYear")
			return
		}
		// OpenCloud's KQL grammar only supports equality right now;
		// narrow via the range once upstream land the numeric range
		// operators. For now, fall back to the first year.
		query += " AND audio.year:" + quote(strconv.Itoa(*params.FromYear))
	}

	out, err := s.aggregateAlbumList(r.Context(), query, string(params.Type), offset, size)
	if err != nil {
		// newest/recent fall back to a hits scan because mtime isn't
		// a bucket property.
		if err == errNeedsHitsScan {
			out, err = s.hitsAlbumList(r.Context(), query, offset, size)
		}
	}
	if err != nil {
		s.logger.Warn().Err(err).Str("type", string(params.Type)).Msg("getAlbumList2: failed")
		proto.WriteError(w, proto.ErrGeneric, "failed to list albums")
		return
	}

	proto.WriteResponse(w, model.GetAlbumList2SuccessResponse{
		Status:        model.GetAlbumList2SuccessResponseStatusOk,
		Version:       proto.APIVersion,
		Type:          proto.ServerType,
		ServerVersion: proto.ServerVersion,
		OpenSubsonic:  true,
		AlbumList2:    model.AlbumList2{Album: ptr(out)},
	})
}

// errNeedsHitsScan signals that the requested list type needs the
// mtime-sorted hit fallback rather than the aggregation path.
var errNeedsHitsScan = &needsHitsScanErr{}

type needsHitsScanErr struct{}

func (*needsHitsScanErr) Error() string { return "album list requires hits scan" }

// aggregateAlbumList returns album entries for `type` values that can
// be expressed as a terms aggregation. Returns errNeedsHitsScan for
// newest/recent/unknown, so the caller can fall back.
func (s *Server) aggregateAlbumList(ctx context.Context, query, listType string, offset, size int) ([]model.AlbumID3, error) {
	switch listType {
	case "newest", "recent":
		return nil, errNeedsHitsScan
	}

	// Aggregate: terms(audio.artist) → terms(audio.album) → sum(duration).
	artistOpt := buildAggregation("audio.artist", 500,
		buildAggregation("audio.album", 500,
			buildMetric("audio.duration", "sum"),
		),
	)
	aggs, err := s.graph.SearchAggregateWithOptions(ctx, query,
		[]libregraph.AggregationOption{artistOpt})
	if err != nil {
		return nil, err
	}

	type flatAlbum struct {
		artist, album   string
		songCount       int64
		durationSeconds int64
	}
	var all []flatAlbum
	for _, a := range aggs {
		if a.Field == nil || *a.Field != "audio.artist" {
			continue
		}
		for _, artistBucket := range a.Buckets {
			artist := derefBucket(artistBucket.Key)
			if artist == "" {
				continue
			}
			for _, sub := range artistBucket.SubAggregations {
				if sub.Field == nil || *sub.Field != "audio.album" {
					continue
				}
				for _, albumBucket := range sub.Buckets {
					album := derefBucket(albumBucket.Key)
					if album == "" {
						continue
					}
					d := int64(0)
					for _, m := range albumBucket.SubAggregations {
						if m.Field != nil && *m.Field == "audio.duration" && m.Value != nil {
							d = int64(*m.Value / 1000)
						}
					}
					all = append(all, flatAlbum{
						artist:          artist,
						album:           album,
						songCount:       derefInt64(albumBucket.Count),
						durationSeconds: d,
					})
				}
			}
		}
	}

	switch listType {
	case "random":
		rand.Shuffle(len(all), func(i, j int) { all[i], all[j] = all[j], all[i] })
	case "alphabeticalByName":
		sort.SliceStable(all, func(i, j int) bool {
			return strings.ToLower(all[i].album) < strings.ToLower(all[j].album)
		})
	case "alphabeticalByArtist":
		sort.SliceStable(all, func(i, j int) bool {
			return strings.ToLower(all[i].artist) < strings.ToLower(all[j].artist)
		})
	}

	if offset < len(all) {
		all = all[offset:]
	} else {
		all = nil
	}
	if size < len(all) {
		all = all[:size]
	}

	out := make([]model.AlbumID3, 0, len(all))
	for _, a := range all {
		out = append(out, albumEntry(a.artist, a.album, a.songCount, a.durationSeconds))
	}
	return out, nil
}

// hitsAlbumList implements the newest/recent code path: scan every
// matching audio hit, group by (artist, album), then sort by newest
// lastModified date across each album's tracks.
func (s *Server) hitsAlbumList(ctx context.Context, query string, offset, size int) ([]model.AlbumID3, error) {
	hits, err := s.graph.SearchHits(ctx, query, 0, 500)
	if err != nil {
		return nil, err
	}
	type albumAgg struct {
		artist, album string
		count         int64
		duration      int64
		youngest      string
	}
	by := map[string]*albumAgg{}
	order := []string{}
	for _, h := range hits.Hits {
		if h.Resource == nil {
			continue
		}
		a, t := albumKeyFromSong(h.Resource)
		if a == "" || t == "" {
			continue
		}
		key := a + "\x00" + t
		agg, seen := by[key]
		if !seen {
			agg = &albumAgg{artist: a, album: t}
			by[key] = agg
			order = append(order, key)
		}
		agg.count++
		if h.Resource.Audio != nil && h.Resource.Audio.Duration != nil {
			agg.duration += *h.Resource.Audio.Duration / 1000
		}
		if h.Resource.LastModifiedDateTime != nil {
			ts := h.Resource.LastModifiedDateTime.UTC().Format("2006-01-02T15:04:05Z")
			if ts > agg.youngest {
				agg.youngest = ts
			}
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		return by[order[i]].youngest > by[order[j]].youngest
	})
	if offset < len(order) {
		order = order[offset:]
	} else {
		order = nil
	}
	if size < len(order) {
		order = order[:size]
	}
	out := make([]model.AlbumID3, 0, len(order))
	for _, k := range order {
		a := by[k]
		out = append(out, albumEntry(a.artist, a.album, a.count, a.duration))
	}
	return out, nil
}

// PostGetAlbumList2 mirrors GetAlbumList2.
//
// (POST /rest/getAlbumList2)
func (s *Server) PostGetAlbumList2(w http.ResponseWriter, r *http.Request) {
	// Read required params out of the form body and delegate.
	if err := r.ParseForm(); err != nil {
		proto.WriteError(w, proto.ErrGeneric, "could not parse form body")
		return
	}
	p := model.GetAlbumList2Params{Type: model.AlbumListType(r.PostForm.Get("type"))}
	if v := r.PostForm.Get("size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			p.Size = &n
		}
	}
	if v := r.PostForm.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			p.Offset = &n
		}
	}
	if v := r.PostForm.Get("genre"); v != "" {
		p.Genre = &v
	}
	s.GetAlbumList2(w, r, p)
}

// GetRandomSongs fetches audio hits and returns a random subset.
//
// (GET /rest/getRandomSongs)
func (s *Server) GetRandomSongs(w http.ResponseWriter, r *http.Request, params model.GetRandomSongsParams) {
	if !s.requireAuth(w, r) {
		return
	}
	size := 10
	if params.Size != nil && *params.Size > 0 {
		size = *params.Size
	}
	if size > 500 {
		size = 500
	}
	query := kqlAudio
	if params.Genre != nil && *params.Genre != "" {
		query += " AND audio.genre:" + quote(*params.Genre)
	}
	hits, err := s.graph.SearchHits(r.Context(), query, 0, 500)
	if err != nil {
		s.logger.Warn().Err(err).Msg("getRandomSongs: search failed")
		proto.WriteError(w, proto.ErrGeneric, "failed to list random songs")
		return
	}
	items := make([]*libregraph.DriveItem, 0, len(hits.Hits))
	for _, h := range hits.Hits {
		if h.Resource != nil {
			items = append(items, h.Resource)
		}
	}
	rand.Shuffle(len(items), func(i, j int) { items[i], items[j] = items[j], items[i] })
	if size < len(items) {
		items = items[:size]
	}
	songs := make([]model.Child, 0, len(items))
	for _, it := range items {
		songs = append(songs, driveItemToChild(it))
	}
	proto.WriteResponse(w, model.GetRandomSongsSuccessResponse{
		Status:        model.GetRandomSongsSuccessResponseStatusOk,
		Version:       proto.APIVersion,
		Type:          proto.ServerType,
		ServerVersion: proto.ServerVersion,
		OpenSubsonic:  true,
		RandomSongs:   model.Songs{Song: ptr(songs)},
	})
}

// PostGetRandomSongs mirrors GetRandomSongs.
//
// (POST /rest/getRandomSongs)
func (s *Server) PostGetRandomSongs(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		proto.WriteError(w, proto.ErrGeneric, "could not parse form body")
		return
	}
	p := model.GetRandomSongsParams{}
	if v := r.PostForm.Get("size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			p.Size = &n
		}
	}
	if v := r.PostForm.Get("genre"); v != "" {
		p.Genre = &v
	}
	s.GetRandomSongs(w, r, p)
}

// GetSongsByGenre runs a genre-filtered audio search and returns the
// hits as songs (order follows server defaults).
//
// (GET /rest/getSongsByGenre)
func (s *Server) GetSongsByGenre(w http.ResponseWriter, r *http.Request, params model.GetSongsByGenreParams) {
	if !s.requireAuth(w, r) {
		return
	}
	if params.Genre == "" {
		proto.WriteError(w, proto.ErrMissingParam, "genre is required")
		return
	}
	size := 10
	if params.Count != nil && *params.Count > 0 {
		size = *params.Count
	}
	if size > 500 {
		size = 500
	}
	offset := int32(0)
	if params.Offset != nil {
		offset = int32(*params.Offset)
	}
	hits, err := s.graph.SearchHits(r.Context(), kqlAudio+" AND audio.genre:"+quote(params.Genre), offset, int32(size))
	if err != nil {
		s.logger.Warn().Err(err).Str("genre", params.Genre).Msg("getSongsByGenre: search failed")
		proto.WriteError(w, proto.ErrGeneric, "failed to list songs")
		return
	}
	songs := make([]model.Child, 0, len(hits.Hits))
	for _, h := range hits.Hits {
		if h.Resource == nil {
			continue
		}
		songs = append(songs, driveItemToChild(h.Resource))
	}
	proto.WriteResponse(w, model.GetSongsByGenreSuccessResponse{
		Status:        model.GetSongsByGenreSuccessResponseStatusOk,
		Version:       proto.APIVersion,
		Type:          proto.ServerType,
		ServerVersion: proto.ServerVersion,
		OpenSubsonic:  true,
		SongsByGenre:  model.Songs{Song: ptr(songs)},
	})
}

// PostGetSongsByGenre mirrors GetSongsByGenre.
//
// (POST /rest/getSongsByGenre)
func (s *Server) PostGetSongsByGenre(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		proto.WriteError(w, proto.ErrGeneric, "could not parse form body")
		return
	}
	p := model.GetSongsByGenreParams{Genre: r.PostForm.Get("genre")}
	if v := r.PostForm.Get("count"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			p.Count = &n
		}
	}
	if v := r.PostForm.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			p.Offset = &n
		}
	}
	s.GetSongsByGenre(w, r, p)
}

// Search3 issues two queries concurrently: one aggregation that
// delivers the artist/album dimensions (with real runtimes) and a
// hits query that produces the song-list. Artist and album dimensions
// now come from a nested aggregation — accurate across the full
// matched corpus rather than capped at whatever fits in the scan
// window.
//
// (GET /rest/search3)
func (s *Server) Search3(w http.ResponseWriter, r *http.Request, params model.Search3Params) {
	if !s.requireAuth(w, r) {
		return
	}
	artistCount, albumCount, songCount := 20, 20, 20
	if params.ArtistCount != nil {
		artistCount = *params.ArtistCount
	}
	if params.AlbumCount != nil {
		albumCount = *params.AlbumCount
	}
	if params.SongCount != nil {
		songCount = *params.SongCount
	}

	q := strings.TrimSpace(params.Query)
	query := kqlAudio
	if q != "" && q != `""` {
		// content: matches Tika's extracted text body (tokenised,
		// lowercased, stemmed) — forgiving for codecs where Tika
		// embeds tags into the content stream. The audio.* branches
		// are the exact-match fallback for keyword-analysed fields.
		query += " AND (content:" + quote(q) +
			" OR audio.title:" + quote(q) +
			" OR audio.artist:" + quote(q) +
			" OR audio.album:" + quote(q) + ")"
	}

	// Nested aggregation: terms(audio.artist) -> terms(audio.album)
	// -> sum(audio.duration). Gives us correct albumCount per artist
	// and total runtime per album in one round trip.
	artistOpt := buildAggregation("audio.artist", int32(artistCount),
		buildAggregation("audio.album", int32(albumCount),
			buildMetric("audio.duration", "sum"),
		),
	)
	aggs, aggErr := s.graph.SearchAggregateWithOptions(r.Context(), query,
		[]libregraph.AggregationOption{artistOpt})
	if aggErr != nil {
		s.logger.Warn().Err(aggErr).Msg("search3: aggregate failed")
		proto.WriteError(w, proto.ErrGeneric, "search failed")
		return
	}

	hits, hitsErr := s.graph.SearchHits(r.Context(), query, 0, int32(songCount))
	if hitsErr != nil {
		s.logger.Warn().Err(hitsErr).Msg("search3: hits failed")
		proto.WriteError(w, proto.ErrGeneric, "search failed")
		return
	}

	artists := make([]model.ArtistID3, 0)
	albums := make([]model.AlbumID3, 0)
	for _, a := range aggs {
		if a.Field == nil || *a.Field != "audio.artist" {
			continue
		}
		for _, artistBucket := range a.Buckets {
			if len(artists) >= artistCount && len(albums) >= albumCount {
				break
			}
			artist := derefBucket(artistBucket.Key)
			if artist == "" {
				continue
			}
			artistAlbums := 0
			for _, sub := range artistBucket.SubAggregations {
				if sub.Field != nil && *sub.Field == "audio.album" {
					artistAlbums = len(sub.Buckets)
					for _, albumBucket := range sub.Buckets {
						if len(albums) >= albumCount {
							break
						}
						album := derefBucket(albumBucket.Key)
						if album == "" {
							continue
						}
						duration := int64(0)
						for _, m := range albumBucket.SubAggregations {
							if m.Field != nil && *m.Field == "audio.duration" && m.Value != nil {
								duration = int64(*m.Value / 1000)
							}
						}
						albums = append(albums, albumEntry(artist, album,
							derefInt64(albumBucket.Count), duration))
					}
				}
			}
			if len(artists) < artistCount {
				artists = append(artists, model.ArtistID3{
					Id:         artistID(artist),
					Name:       artist,
					AlbumCount: ptr(artistAlbums),
					CoverArt:   ptr(artistID(artist)),
				})
			}
		}
	}

	songs := make([]model.Child, 0, songCount)
	for _, h := range hits.Hits {
		if h.Resource == nil {
			continue
		}
		if len(songs) >= songCount {
			break
		}
		songs = append(songs, driveItemToChild(h.Resource))
	}

	proto.WriteResponse(w, model.Search3SuccessResponse{
		Status:        model.Search3SuccessResponseStatusOk,
		Version:       proto.APIVersion,
		Type:          proto.ServerType,
		ServerVersion: proto.ServerVersion,
		OpenSubsonic:  true,
		SearchResult3: model.SearchResult3{
			Artist: ptr(artists),
			Album:  ptr(albums),
			Song:   ptr(songs),
		},
	})
}

// PostSearch3 mirrors Search3.
//
// (POST /rest/search3)
func (s *Server) PostSearch3(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		proto.WriteError(w, proto.ErrGeneric, "could not parse form body")
		return
	}
	p := model.Search3Params{Query: r.PostForm.Get("query")}
	for _, spec := range []struct {
		key string
		dst **int
	}{
		{"artistCount", &p.ArtistCount},
		{"albumCount", &p.AlbumCount},
		{"songCount", &p.SongCount},
	} {
		if v := r.PostForm.Get(spec.key); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				*spec.dst = &n
			}
		}
	}
	s.Search3(w, r, p)
}
