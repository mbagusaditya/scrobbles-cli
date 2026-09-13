package service

import (
	"context"
	"fmt"
	"time"
)

type StatsFilter struct {
	From time.Time
	To   time.Time
}

type StatsResult struct {
	TotalScrobbles int
	TotalArtists   int
	TotalAlbums    int
	TotalTracks    int
}

// GetStats menghitung agregasi total scrobbles, artis unik, album unik,
// dan track unik dalam 1 kali roundtrip query ke database Turso.
// Jika filter.From atau filter.To bernilai zero-value (IsZero),
// batas tersebut tidak akan disertakan dalam klausa WHERE (all-time).
func (s *ScrobbleService) GetStats(ctx context.Context, filter StatsFilter) (*StatsResult, error) {
	query := `
		SELECT
			COUNT(*) AS total_scrobbles,
			COUNT(DISTINCT raw_artist) AS total_artists,
			COUNT(DISTINCT CASE
				WHEN raw_album IS NOT NULL AND raw_album != '' THEN raw_album
			END) AS total_albums,
			COUNT(DISTINCT raw_artist || ' - ' || raw_title) AS total_tracks
		FROM scrobble_logs
		WHERE 1=1
	`

	var args []any

	if !filter.From.IsZero() {
		query += " AND played_at >= ?"
		args = append(args, filter.From.Unix())
	}

	if !filter.To.IsZero() {
		query += " AND played_at <= ?"
		args = append(args, filter.To.Unix())
	}

	var res StatsResult
	row := s.db.QueryRowContext(ctx, query, args...)
	err := row.Scan(
		&res.TotalScrobbles,
		&res.TotalArtists,
		&res.TotalAlbums,
		&res.TotalTracks,
	)
	if err != nil {
		return nil, fmt.Errorf("gagal query statistik scrobble: %w", err)
	}

	return &res, nil
}
