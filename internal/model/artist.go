package model

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// Artist merepresentasikan satu baris di tabel artists.
type Artist struct {
	ID        string         // primary key, UUID
	Name      string         // notNull, unique
	Mbid      sql.NullString // nullable, MusicBrainz ID
	ImageURL  sql.NullString // nullable
	CreatedAt time.Time      // notNull
}

// NewArtist membuat instance Artist baru dengan ID dan CreatedAt otomatis.
// mbid dan imageURL boleh dikirim string kosong "" -> otomatis NULL.
func NewArtist(name, mbid, imageURL string) *Artist {
	return &Artist{
		ID:        uuid.NewString(),
		Name:      name,
		Mbid:      toNullString(mbid),
		ImageURL:  toNullString(imageURL),
		CreatedAt: time.Now(),
	}
}
