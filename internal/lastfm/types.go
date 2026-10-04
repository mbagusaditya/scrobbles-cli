package lastfm

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// RecentTracksResponse adalah root response dari endpoint user.getrecenttracks.
type RecentTracksResponse struct {
	RecentTracks RecentTracksPayload `json:"recenttracks"`
}

type RecentTracksPayload struct {
	Tracks TrackList        `json:"track"`
	Attr   RecentTracksAttr `json:"@attr"`
}

// RecentTracksAttr berisi metadata pagination. Semua field string karena
// Last.fm API mengembalikannya sebagai string, bukan number, di JSON.
type RecentTracksAttr struct {
	User       string `json:"user"`
	TotalPages string `json:"totalPages"`
	Page       string `json:"page"`
	Total      string `json:"total"`
	PerPage    string `json:"perPage"`
}

// TotalPagesInt mengonversi TotalPages ke int, dengan fallback 0 kalau parse gagal.
func (a RecentTracksAttr) TotalPagesInt() int {
	n, err := strconv.Atoi(a.TotalPages)
	if err != nil {
		return 0
	}
	return n
}

// Track merepresentasikan satu entry track dari response API.
// Attr bernilai non-nil HANYA kalau track ini sedang "now playing"
// (belum final scrobble, Date akan kosong).
type Track struct {
	Artist TrackArtist `json:"artist"`
	Name   string      `json:"name"`
	Album  TrackAlbum  `json:"album"`
	Date   *TrackDate  `json:"date,omitempty"`
	Attr   *TrackAttr  `json:"@attr,omitempty"`
}

type TrackArtist struct {
	Text string `json:"#text"`
	Mbid string `json:"mbid"`
}

type TrackAlbum struct {
	Text string `json:"#text"`
	Mbid string `json:"mbid"`
}

type TrackDate struct {
	Uts  string `json:"uts"`
	Text string `json:"#text"`
}

type TrackAttr struct {
	NowPlaying string `json:"nowplaying"`
}

// IsNowPlaying mengembalikan true kalau track ini adalah track yang sedang
// diputar saat ini (belum final scrobble). Track semacam ini harus DI-SKIP
// saat sync, karena belum punya PlayedAt dan bisa berubah statusnya.
func (t Track) IsNowPlaying() bool {
	return t.Attr != nil && t.Attr.NowPlaying == "true"
}

// PlayedAt mengonversi field Date.Uts (unix timestamp string) menjadi time.Time.
// Mengembalikan error kalau Date nil (ini terjadi pada track now-playing)
// atau kalau Uts tidak valid.
func (t Track) PlayedAt() (time.Time, error) {
	if t.Date == nil {
		return time.Time{}, fmt.Errorf("track %q tidak punya field date (kemungkinan now-playing)", t.Name)
	}
	uts, err := strconv.ParseInt(t.Date.Uts, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("gagal parse uts %q: %w", t.Date.Uts, err)
	}
	return time.Unix(uts, 0).UTC(), nil
}

// TrackList menangani gotcha Last.fm API: field "track" biasanya array,
// tapi kalau hasilnya cuma 1 item, API kadang mengembalikan single object
// alih-alih array berisi 1 elemen. UnmarshalJSON custom ini menormalisasi
// kedua bentuk tersebut jadi []Track yang konsisten.
type TrackList []Track

func (tl *TrackList) UnmarshalJSON(data []byte) error {
	// Coba sebagai array dulu (kasus normal).
	var arr []Track
	if err := json.Unmarshal(data, &arr); err == nil {
		*tl = arr
		return nil
	}

	// Fallback: single object.
	var single Track
	if err := json.Unmarshal(data, &single); err != nil {
		return fmt.Errorf("gagal unmarshal track list (bukan array maupun object): %w", err)
	}
	*tl = TrackList{single}
	return nil
}

// errorResponse merepresentasikan bentuk error dari Last.fm API,
// contoh: {"error": 10, "message": "Invalid API key"}.
type errorResponse struct {
	Error   int    `json:"error"`
	Message string `json:"message"`
}

type TrackInfoResponse struct {
	Track struct {
		Name      string `json:"name"`
		Duration  string `json:"duration"` // ms dalam string
		Listeners string `json:"listeners"`
		Playcount string `json:"playcount"`
		Artist    struct {
			Name string `json:"name"`
		} `json:"artist"`
		Album struct {
			Title string `json:"title"`
		} `json:"album"`
		TopTags struct {
			Tag []struct {
				Name string `json:"name"`
			} `json:"tag"`
		} `json:"toptags"`
		Wiki struct {
			Summary string `json:"summary"`
		} `json:"wiki"`
	} `json:"track"`
}

type ArtistInfoResponse struct {
	Artist struct {
		Name  string `json:"name"`
		Stats struct {
			Listeners string `json:"listeners"`
			Playcount string `json:"playcount"`
		} `json:"stats"`
		Tags struct {
			Tag []struct {
				Name string `json:"name"`
			} `json:"tag"`
		} `json:"tags"`
		Bio struct {
			Summary string `json:"summary"`
		} `json:"bio"`
	} `json:"artist"`
}

type AlbumInfoResponse struct {
	Album struct {
		Name      string `json:"name"`
		Artist    string `json:"artist"`
		Listeners string `json:"listeners"`
		Playcount string `json:"playcount"`
		Tags      struct {
			Tag []struct {
				Name string `json:"name"`
			} `json:"tag"`
		} `json:"tags"`
		Wiki struct {
			Summary string `json:"summary"`
		} `json:"wiki"`
	} `json:"album"`
}
