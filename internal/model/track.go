package model

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// Track merepresentasikan satu baris di tabel tracks.
// Perhatikan: "album" bukan entity/tabel terpisah di schema,
// melainkan kolom albumName di sini.
type Track struct {
	ID         string         // primary key, UUID
	ArtistID   string         // notNull, FK ke artists.id
	Title      string         // notNull
	AlbumName  sql.NullString // nullable
	DurationMs sql.NullInt64  // nullable
	CoverURL   sql.NullString // nullable
	CreatedAt  time.Time      // notNull
}

// NewTrack membuat instance Track baru dengan ID dan CreatedAt otomatis.
// albumName dan coverURL boleh dikirim string kosong "" -> otomatis NULL.
// durationMs boleh dikirim 0 -> otomatis NULL (0 ms dianggap "tidak diketahui").
func NewTrack(artistID, title, albumName, coverURL string, durationMs int64) *Track {
	duration := sql.NullInt64{}
	if durationMs > 0 {
		duration = sql.NullInt64{Int64: durationMs, Valid: true}
	}

	return &Track{
		ID:         uuid.NewString(),
		ArtistID:   artistID,
		Title:      title,
		AlbumName:  toNullString(albumName),
		DurationMs: duration,
		CoverURL:   toNullString(coverURL),
		CreatedAt:  time.Now(),
	}
}
