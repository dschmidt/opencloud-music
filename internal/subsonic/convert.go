package subsonic

import (
	"encoding/base64"
	"errors"
	"net/url"
	"path"
	"strconv"
	"strings"

	libregraph "github.com/opencloud-eu/libre-graph-api-go"
)

// driveItemDownloadURL constructs the WebDAV URL that serves a
// DriveItem's content. OpenCloud's search endpoint does not (yet)
// populate `webDavUrl` on search hits, so we build the URL from the
// driveId and the parent path. If the item already carries a
// webDavUrl we use that instead.
//
// TODO: trace through the OpenCloud source (search / driveItem
// formatter) to pin down exactly which code paths populate
// webDavUrl — once that's available on search hits we can drop the
// manual reconstruction below.
//
// URL shape: https://<base>/dav/spaces/<driveId>/<path>/<name>
// with each path segment URL-encoded independently so spaces become
// %20 and slashes stay as separators.
func driveItemDownloadURL(base string, item *libregraph.DriveItem) string {
	if item == nil {
		return ""
	}
	if item.WebDavUrl != nil && *item.WebDavUrl != "" {
		return *item.WebDavUrl
	}
	if item.ParentReference == nil || item.ParentReference.DriveId == nil {
		return ""
	}
	parts := []string{url.PathEscape(*item.ParentReference.DriveId)}
	if item.ParentReference.Path != nil {
		for _, seg := range strings.Split(strings.Trim(*item.ParentReference.Path, "/"), "/") {
			if seg != "" {
				parts = append(parts, url.PathEscape(seg))
			}
		}
	}
	if item.Name != nil {
		parts = append(parts, url.PathEscape(*item.Name))
	}
	return strings.TrimRight(base, "/") + "/dav/spaces/" + strings.Join(parts, "/")
}

// driveItemPreviewURL returns the OpenCloud preview URL for an item at
// the requested pixel size. Returns "" when the item has no resolvable
// path (missing parentReference).
func driveItemPreviewURL(base string, item *libregraph.DriveItem, size int) string {
	dav := driveItemDownloadURL(base, item)
	if dav == "" {
		return ""
	}
	s := strconv.Itoa(size)
	return dav + "?preview=1&x=" + s + "&y=" + s
}

// ID prefixes. Songs keep the bare driveItem resource ID — OpenCloud
// returns an ID like "storageId$spaceId!opaqueId" which is already
// unique — while artists and albums get a reversible virtual ID since
// they aren't first-class entities in the search index. We base64-url
// encode the source strings so the ID decodes back into the same
// `(artist, album)` tuple on subsequent getArtist/getAlbum calls
// without needing a server-side lookup table.
const (
	artistIDPrefix = "ar-"
	albumIDPrefix  = "al-"
	idFieldSep     = "\x00" // NUL never appears inside an audio.album field
)

func encode(parts ...string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strings.Join(parts, idFieldSep)))
}

func decode(id, prefix string) ([]string, error) {
	if !strings.HasPrefix(id, prefix) {
		return nil, errors.New("subsonic: wrong id prefix")
	}
	raw, err := base64.RawURLEncoding.DecodeString(id[len(prefix):])
	if err != nil {
		return nil, err
	}
	return strings.Split(string(raw), idFieldSep), nil
}

func artistID(albumArtist string) string       { return artistIDPrefix + encode(albumArtist) }
func albumID(albumArtist, album string) string { return albumIDPrefix + encode(albumArtist, album) }

// decodeArtistID recovers the albumArtist string from a virtual ar-* ID.
func decodeArtistID(id string) (string, error) {
	parts, err := decode(id, artistIDPrefix)
	if err != nil || len(parts) != 1 {
		return "", errors.New("invalid artist id")
	}
	return parts[0], nil
}

// decodeAlbumID recovers the (albumArtist, album) tuple from a virtual al-* ID.
func decodeAlbumID(id string) (artist, album string, err error) {
	parts, err := decode(id, albumIDPrefix)
	if err != nil || len(parts) != 2 {
		return "", "", errors.New("invalid album id")
	}
	return parts[0], parts[1], nil
}

// albumKeyFromSong returns the (artist, album) tuple used to group
// DriveItems into albums. We rely exclusively on audio.artist — see
// GetArtists for why audio.albumArtist is intentionally ignored.
func albumKeyFromSong(item *libregraph.DriveItem) (artist, album string) {
	if item.Audio == nil {
		return "", ""
	}
	if item.Audio.Artist != nil {
		artist = *item.Audio.Artist
	}
	if item.Audio.Album != nil {
		album = *item.Audio.Album
	}
	return
}

// discTrack returns a sortable key for a DriveItem within an album
// (disc * 1000 + track). Missing disc/track numbers sort to the end.
func discTrack(item *libregraph.DriveItem) int {
	d, t := 0, 1_000_000
	if item.Audio != nil {
		if item.Audio.Disc != nil {
			d = int(*item.Audio.Disc)
		}
		if item.Audio.Track != nil {
			t = int(*item.Audio.Track)
		}
	}
	return d*1000 + t
}

// driveItemToSong projects a Graph DriveItem + its Audio facet into the
// Subsonic `song` payload shape consumed by clients. Missing optional
// fields are left off the map entirely so clients display their own
// fallbacks instead of empty strings.
func driveItemToSong(item *libregraph.DriveItem) map[string]any {
	id := deref(item.Id)
	out := map[string]any{
		"id":       id,
		"isDir":    false,
		"isVideo":  false,
		"type":     "music",
		"title":    audioTitle(item),
		"coverArt": id, // optimistic — getCoverArt proxies or 404s
	}
	if name := deref(item.Name); name != "" {
		ext := path.Ext(name)
		if len(ext) > 1 {
			out["suffix"] = ext[1:]
		}
	}
	if item.Size != nil {
		out["size"] = *item.Size
	}
	if a := item.Audio; a != nil {
		if a.Album != nil {
			out["album"] = *a.Album
			if a.Artist != nil {
				out["albumId"] = albumID(*a.Artist, *a.Album)
			}
		}
		if a.Artist != nil && *a.Artist != "" {
			out["artist"] = *a.Artist
			out["artistId"] = artistID(*a.Artist)
		}
		if a.Genre != nil {
			out["genre"] = *a.Genre
		}
		if a.Year != nil {
			out["year"] = *a.Year
		}
		if a.Track != nil {
			out["track"] = *a.Track
		}
		if a.Disc != nil {
			out["discNumber"] = *a.Disc
		}
		if a.Duration != nil {
			out["duration"] = *a.Duration / 1000 // Subsonic wants seconds
		}
		if a.Bitrate != nil {
			out["bitRate"] = *a.Bitrate
		}
	}
	return out
}

// audioTitle falls back to the filename (sans extension) if the audio
// facet has no title.
func audioTitle(item *libregraph.DriveItem) string {
	if item.Audio != nil && item.Audio.Title != nil && *item.Audio.Title != "" {
		return *item.Audio.Title
	}
	name := deref(item.Name)
	if name == "" {
		return "Unknown"
	}
	if ext := path.Ext(name); ext != "" {
		return name[:len(name)-len(ext)]
	}
	return name
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
