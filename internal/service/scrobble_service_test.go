package service_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite driver untuk local in-memory test

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
	"github.com/mbagusaditya/scrobbles-cli/internal/service"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	// Gunakan driver name "sqlite" bawaan modernc.org/sqlite
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("gagal koneksi memory db: %v", err)
	}

	schema := `
    CREATE TABLE scrobble_logs (
        id TEXT PRIMARY KEY,
        track_id TEXT,
        raw_title TEXT NOT NULL,
        raw_artist TEXT NOT NULL,
        raw_album TEXT,
        played_at INTEGER NOT NULL,
        created_at INTEGER NOT NULL
    );
    `
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("gagal migrate schema: %v", err)
	}

	return db
}

func TestListScrobbles(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	svc := service.NewScrobbleService(db, nil)
	ctx := context.Background()

	// Seed data dummy
	baseTime := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	seeds := []*model.Scrobble{
		model.NewScrobble("Track 1", "Artist A", "Album X", baseTime),
		model.NewScrobble("Track 2", "Artist A", "Album Y", baseTime.Add(1*time.Hour)),
		model.NewScrobble("Track 3", "Artist B", "Album Z", baseTime.Add(2*time.Hour)),
	}

	for _, s := range seeds {
		query := `INSERT INTO scrobble_logs (id, raw_title, raw_artist, raw_album, played_at, created_at) VALUES (?, ?, ?, ?, ?, ?)`
		_, err := db.Exec(query, s.ID, s.RawTitle, s.RawArtist, s.RawAlbum, s.PlayedAt.Unix(), s.CreatedAt.Unix())
		if err != nil {
			t.Fatalf("gagal seed data: %v", err)
		}
	}

	// 1. Test Filter Artist
	t.Run("filter by artist", func(t *testing.T) {
		res, err := svc.ListScrobbles(ctx, service.ListFilter{Artist: "Artist A"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 2 {
			t.Errorf("ekspektasi 2 tracks, dapat %d", len(res))
		}
	})

	// 2. Test Pagination
	t.Run("pagination limit and offset", func(t *testing.T) {
		res, err := svc.ListScrobbles(ctx, service.ListFilter{Limit: 2, Page: 1})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 2 {
			t.Errorf("ekspektasi 2 tracks, dapat %d", len(res))
		}

		// Hasil harus urut DESC (Track 3 paling baru)
		if res[0].RawTitle != "Track 3" {
			t.Errorf("ekspektasi track pertama 'Track 3', dapat %s", res[0].RawTitle)
		}
	})
}
