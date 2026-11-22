package model

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// Scrobble merepresentasikan satu baris di tabel scrobble_logs.
type Scrobble struct {
	ID        string         // primary key, UUID
	TrackID   sql.NullString // nullable, FK ke tracks.id (resolver belum jalan -> selalu NULL untuk sekarang)
	RawTitle  string         // notNull
	RawArtist string         // notNull
	RawAlbum  sql.NullString // nullable
	PlayedAt  time.Time      // notNull, dari field "uts" Last.fm
	CreatedAt time.Time      // notNull
}

// NewScrobble membuat instance Scrobble baru dengan ID dan CreatedAt otomatis.
// rawAlbum boleh dikirim string kosong "" -> otomatis NULL.
func NewScrobble(rawTitle, rawArtist, rawAlbum string, playedAt time.Time) *Scrobble {
	return &Scrobble{
		ID:        uuid.NewString(),
		TrackID:   sql.NullString{}, // NULL, resolver matching belum diimplementasikan
		RawTitle:  rawTitle,
		RawArtist: rawArtist,
		RawAlbum:  toNullString(rawAlbum),
		PlayedAt:  playedAt,
		CreatedAt: time.Now(),
	}
}

// ScrobbleListFilter menampung parameter filter dan paginasi untuk daftar log scrobble.
type ScrobbleListFilter struct {
	Artist string
	Album  string
	From   time.Time
	To     time.Time
	Limit  int
	Offset int
}

// SyncResult merangkum hasil keseluruhan proses sinkronisasi dari Last.fm.
type SyncResult struct {
	Inserted int
	Skipped  int
	Fetched  int
}

// PageProgress melaporkan progres pengambilan per halaman dari API Last.fm.
type PageProgress struct {
	Page       int
	TotalPages int
	Fetched    int
	Inserted   int
	Skipped    int
}
