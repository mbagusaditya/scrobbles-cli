package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
)

// TrackRepository menangani kueri dan pembaruan metadata pada tabel master tracks.
type TrackRepository interface {
	FetchMissingDurations(ctx context.Context) ([]model.MissingDurationTrack, error)
	FetchKnownDurations(ctx context.Context) (map[string]int64, error)
	UpdateDuration(ctx context.Context, artist, title string, durationMs int64) error
}

var _ TrackRepository = (*sqlTrackRepository)(nil)

type sqlTrackRepository struct {
	db *sql.DB
}

func NewTrackRepository(db *sql.DB) TrackRepository {
	return &sqlTrackRepository{db: db}
}

func (r *sqlTrackRepository) FetchMissingDurations(ctx context.Context) ([]model.MissingDurationTrack, error) {
	const query = `
		SELECT DISTINCT a.name, t.title
		FROM tracks t
		JOIN artists a ON t.artist_id = a.id
		WHERE t.duration_ms IS NULL OR t.duration_ms <= 0
		ORDER BY a.name ASC, t.title ASC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("gagal query tracks tanpa durasi: %w", err)
	}
	defer rows.Close()

	var targets []model.MissingDurationTrack
	for rows.Next() {
		var item model.MissingDurationTrack
		if err := rows.Scan(&item.Artist, &item.Title); err != nil {
			return nil, fmt.Errorf("gagal scan baris track: %w", err)
		}
		targets = append(targets, item)
	}
	return targets, rows.Err()
}

func (r *sqlTrackRepository) FetchKnownDurations(ctx context.Context) (map[string]int64, error) {
	const query = `
		SELECT a.name, t.title, MAX(t.duration_ms)
		FROM tracks t
		JOIN artists a ON t.artist_id = a.id
		WHERE t.duration_ms IS NOT NULL AND t.duration_ms > 0
		GROUP BY LOWER(TRIM(a.name)), LOWER(TRIM(t.title))
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("gagal query durasi tracks yang sudah ada: %w", err)
	}
	defer rows.Close()

	known := make(map[string]int64)
	for rows.Next() {
		var artist, title string
		var durationMs int64
		if err := rows.Scan(&artist, &title, &durationMs); err == nil && durationMs > 0 {
			key := strings.ToLower(strings.TrimSpace(artist)) + "\x00" + strings.ToLower(strings.TrimSpace(title))
			known[key] = durationMs
		}
	}
	return known, rows.Err()
}

func (r *sqlTrackRepository) UpdateDuration(ctx context.Context, artist, title string, durationMs int64) error {
	const query = `
		UPDATE tracks
		SET duration_ms = ?
		WHERE title = ?
		  AND artist_id IN (SELECT id FROM artists WHERE name = ?)
	`
	if _, err := r.db.ExecContext(ctx, query, durationMs, title, artist); err != nil {
		return fmt.Errorf("gagal menyimpan durasi untuk %q - %q: %w", artist, title, err)
	}
	return nil
}
