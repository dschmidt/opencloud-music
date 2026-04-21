package subsonic

import (
	"net/http"

	"github.com/opencloud-eu/opencloud-music/internal/subsonic/model"
	"github.com/opencloud-eu/opencloud-music/internal/subsonic/proto"
)

// The Subsonic protocol surface is large; clients ping a lot of
// endpoints even when the corresponding features are disabled via
// `getUser`. To keep their logs clean we answer with empty or "ok"
// envelopes instead of letting the generated Unimplemented stubs
// return 501. None of this state is persisted — these are purely
// protocol stubs.

// okStub writes the empty success envelope shared by scrobble / star /
// setRating / jukeboxControl — endpoints the spec defines as returning
// only the envelope with no payload. The spec's response type for
// these endpoints is a union wrapper that isn't useful for building
// responses, so we reuse the base SubsonicSuccessResponse.
func okStub(w http.ResponseWriter) { proto.WriteResponse(w, okEnvelope()) }

// --- Annotation (star / unstar / setRating / scrobble) ---

func (s *Server) Star(w http.ResponseWriter, r *http.Request, _ model.StarParams) { okStub(w) }
func (s *Server) PostStar(w http.ResponseWriter, r *http.Request)                 { okStub(w) }
func (s *Server) Unstar(w http.ResponseWriter, r *http.Request, _ model.UnstarParams) {
	okStub(w)
}
func (s *Server) PostUnstar(w http.ResponseWriter, r *http.Request) { okStub(w) }
func (s *Server) SetRating(w http.ResponseWriter, r *http.Request, _ model.SetRatingParams) {
	okStub(w)
}
func (s *Server) PostSetRating(w http.ResponseWriter, r *http.Request) { okStub(w) }
func (s *Server) Scrobble(w http.ResponseWriter, r *http.Request, _ model.ScrobbleParams) {
	okStub(w)
}
func (s *Server) PostScrobble(w http.ResponseWriter, r *http.Request) { okStub(w) }

// --- Now playing ---

func (s *Server) GetNowPlaying(w http.ResponseWriter, r *http.Request) {
	proto.WriteResponse(w, model.GetNowPlayingSuccessResponse{
		Status:        model.GetNowPlayingSuccessResponseStatusOk,
		Version:       proto.APIVersion,
		Type:          proto.ServerType,
		ServerVersion: proto.ServerVersion,
		OpenSubsonic:  true,
		NowPlaying:    model.NowPlaying{Entry: []model.NowPlayingEntry{}},
	})
}
func (s *Server) PostGetNowPlaying(w http.ResponseWriter, r *http.Request) {
	s.GetNowPlaying(w, r)
}

// --- Playlists (deferred) ---

func (s *Server) GetPlaylists(w http.ResponseWriter, r *http.Request, _ model.GetPlaylistsParams) {
	proto.WriteResponse(w, model.GetPlaylistsSuccessResponse{
		Status:        model.GetPlaylistsSuccessResponseStatusOk,
		Version:       proto.APIVersion,
		Type:          proto.ServerType,
		ServerVersion: proto.ServerVersion,
		OpenSubsonic:  true,
		Playlists:     model.Playlists{Playlist: ptr([]model.Playlist{})},
	})
}
func (s *Server) PostGetPlaylists(w http.ResponseWriter, r *http.Request) {
	s.GetPlaylists(w, r, model.GetPlaylistsParams{})
}

// --- Podcasts / Internet radio / Chat / Bookmarks ---

func (s *Server) GetPodcasts(w http.ResponseWriter, r *http.Request, _ model.GetPodcastsParams) {
	proto.WriteResponse(w, model.GetPodcastsSuccessResponse{
		Status:        model.GetPodcastsSuccessResponseStatusOk,
		Version:       proto.APIVersion,
		Type:          proto.ServerType,
		ServerVersion: proto.ServerVersion,
		OpenSubsonic:  true,
		Podcasts:      model.Podcasts{Channel: ptr([]model.PodcastChannel{})},
	})
}
func (s *Server) PostGetPodcasts(w http.ResponseWriter, r *http.Request) {
	s.GetPodcasts(w, r, model.GetPodcastsParams{})
}
func (s *Server) GetNewestPodcasts(w http.ResponseWriter, r *http.Request, _ model.GetNewestPodcastsParams) {
	proto.WriteResponse(w, model.GetNewestPodcastsSuccessResponse{
		Status:         model.GetNewestPodcastsSuccessResponseStatusOk,
		Version:        proto.APIVersion,
		Type:           proto.ServerType,
		ServerVersion:  proto.ServerVersion,
		OpenSubsonic:   true,
		NewestPodcasts: model.NewestPodcasts{Episode: ptr([]model.PodcastEpisode{})},
	})
}
func (s *Server) PostGetNewestPodcasts(w http.ResponseWriter, r *http.Request) {
	s.GetNewestPodcasts(w, r, model.GetNewestPodcastsParams{})
}
func (s *Server) GetInternetRadioStations(w http.ResponseWriter, r *http.Request) {
	proto.WriteResponse(w, model.GetInternetRadioStationsSuccessResponse{
		Status:                model.GetInternetRadioStationsSuccessResponseStatusOk,
		Version:               proto.APIVersion,
		Type:                  proto.ServerType,
		ServerVersion:         proto.ServerVersion,
		OpenSubsonic:          true,
		InternetRadioStations: model.InternetRadioStations{InternetRadioStation: ptr([]model.InternetRadioStation{})},
	})
}
func (s *Server) PostGetInternetRadioStations(w http.ResponseWriter, r *http.Request) {
	s.GetInternetRadioStations(w, r)
}
func (s *Server) GetChatMessages(w http.ResponseWriter, r *http.Request) {
	proto.WriteResponse(w, model.GetChatMessagesSuccessResponse{
		Status:        model.GetChatMessagesSuccessResponseStatusOk,
		Version:       proto.APIVersion,
		Type:          proto.ServerType,
		ServerVersion: proto.ServerVersion,
		OpenSubsonic:  true,
		ChatMessages:  model.ChatMessages{ChatMessage: ptr([]model.ChatMessage{})},
	})
}
func (s *Server) PostGetChatMessages(w http.ResponseWriter, r *http.Request) {
	s.GetChatMessages(w, r)
}
func (s *Server) GetBookmarks(w http.ResponseWriter, r *http.Request) {
	proto.WriteResponse(w, model.GetBookmarksSuccessResponse{
		Status:        model.GetBookmarksSuccessResponseStatusOk,
		Version:       proto.APIVersion,
		Type:          proto.ServerType,
		ServerVersion: proto.ServerVersion,
		OpenSubsonic:  true,
		Bookmarks:     model.Bookmarks{Bookmark: ptr([]model.Bookmark{})},
	})
}
func (s *Server) PostGetBookmarks(w http.ResponseWriter, r *http.Request) {
	s.GetBookmarks(w, r)
}
