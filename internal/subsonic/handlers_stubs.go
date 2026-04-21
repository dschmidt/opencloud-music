package subsonic

import (
	"net/http"

	"github.com/opencloud-eu/opencloud-music/internal/subsonic/proto"
)

// The Subsonic protocol surface is large; clients ping a lot of
// endpoints even when the corresponding features are disabled via
// `getUser`. To keep their logs clean we answer with empty or "ok"
// envelopes instead of letting the generated Unimplemented stubs
// return 501. None of this state is persisted — these are purely
// protocol stubs.

// okStub is the canonical no-op: empty success envelope. Used by
// scrobble, star, setRating, jukeboxControl, bookmark endpoints, etc.
func okStub(w http.ResponseWriter) { proto.WriteOK(w, nil) }

// --- Annotation (star / unstar / setRating / scrobble) ---

func (s *Server) Star(w http.ResponseWriter, r *http.Request, _ StarParams)     { okStub(w) }
func (s *Server) PostStar(w http.ResponseWriter, r *http.Request)               { okStub(w) }
func (s *Server) Unstar(w http.ResponseWriter, r *http.Request, _ UnstarParams) { okStub(w) }
func (s *Server) PostUnstar(w http.ResponseWriter, r *http.Request)             { okStub(w) }
func (s *Server) SetRating(w http.ResponseWriter, r *http.Request, _ SetRatingParams) {
	okStub(w)
}
func (s *Server) PostSetRating(w http.ResponseWriter, r *http.Request) { okStub(w) }
func (s *Server) Scrobble(w http.ResponseWriter, r *http.Request, _ ScrobbleParams) {
	okStub(w)
}
func (s *Server) PostScrobble(w http.ResponseWriter, r *http.Request) { okStub(w) }

// --- Now playing ---

func (s *Server) GetNowPlaying(w http.ResponseWriter, r *http.Request) {
	proto.WriteOK(w, map[string]any{"nowPlaying": map[string]any{"entry": []any{}}})
}
func (s *Server) PostGetNowPlaying(w http.ResponseWriter, r *http.Request) {
	s.GetNowPlaying(w, r)
}

// --- Playlists (deferred) ---

func (s *Server) GetPlaylists(w http.ResponseWriter, r *http.Request, _ GetPlaylistsParams) {
	proto.WriteOK(w, map[string]any{"playlists": map[string]any{"playlist": []any{}}})
}
func (s *Server) PostGetPlaylists(w http.ResponseWriter, r *http.Request) {
	s.GetPlaylists(w, r, GetPlaylistsParams{})
}

// --- Podcasts / Internet radio / Jukebox / Chat / Bookmarks ---

func (s *Server) GetPodcasts(w http.ResponseWriter, r *http.Request, _ GetPodcastsParams) {
	proto.WriteOK(w, map[string]any{"podcasts": map[string]any{"channel": []any{}}})
}
func (s *Server) PostGetPodcasts(w http.ResponseWriter, r *http.Request) {
	s.GetPodcasts(w, r, GetPodcastsParams{})
}
func (s *Server) GetNewestPodcasts(w http.ResponseWriter, r *http.Request, _ GetNewestPodcastsParams) {
	proto.WriteOK(w, map[string]any{"newestPodcasts": map[string]any{"episode": []any{}}})
}
func (s *Server) PostGetNewestPodcasts(w http.ResponseWriter, r *http.Request) {
	s.GetNewestPodcasts(w, r, GetNewestPodcastsParams{})
}
func (s *Server) GetInternetRadioStations(w http.ResponseWriter, r *http.Request) {
	proto.WriteOK(w, map[string]any{"internetRadioStations": map[string]any{"internetRadioStation": []any{}}})
}
func (s *Server) PostGetInternetRadioStations(w http.ResponseWriter, r *http.Request) {
	s.GetInternetRadioStations(w, r)
}
func (s *Server) GetChatMessages(w http.ResponseWriter, r *http.Request) {
	proto.WriteOK(w, map[string]any{"chatMessages": map[string]any{"chatMessage": []any{}}})
}
func (s *Server) PostGetChatMessages(w http.ResponseWriter, r *http.Request) {
	s.GetChatMessages(w, r)
}
func (s *Server) GetBookmarks(w http.ResponseWriter, r *http.Request) {
	proto.WriteOK(w, map[string]any{"bookmarks": map[string]any{"bookmark": []any{}}})
}
func (s *Server) PostGetBookmarks(w http.ResponseWriter, r *http.Request) {
	s.GetBookmarks(w, r)
}
