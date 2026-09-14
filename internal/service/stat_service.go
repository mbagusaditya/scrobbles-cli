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

func (s *ScrobbleService) GetStats(ctx context.Context, filter StatsFilter) (*StatsResult, error) {
	// Skenario 1: All-Time Stats (Memanfaatkan master tabel artists & tracks secara langsung)
	if filter.From.IsZero() && filter.To.IsZero() {
		query := `
			SELECT
				(SELECT COUNT(*) FROM scrobble_logs) AS total_scrobbles,
				(SELECT COUNT(*) FROM artists) AS total_artists,
				(SELECT COUNT(DISTINCT raw_album) FROM scrobble_logs WHERE raw_album IS NOT NULL AND raw_album != '') AS total_albums,
				(SELECT COUNT(*) FROM tracks) AS total_tracks;
		`

		var res StatsResult
		row := s.db.QueryRowContext(ctx, query)
		err := row.Scan(
			&res.TotalScrobbles,
			&res.TotalArtists,
			&res.TotalAlbums,
			&res.TotalTracks,
		)
		if err != nil {
			return nil, fmt.Errorf("gagal query all-time statistik: %w", err)
		}
		return &res, nil
	}

	// Skenario 2: Filter Rentang Tanggal Tertentu
	// Menghitung aktivitas spesifik pada rentang tanggal tersebut dari scrobble_logs
	query := `
		SELECT
			COUNT(*) AS total_scrobbles,
			COUNT(DISTINCT raw_artist) AS total_artists,
			COUNT(DISTINCT CASE
				WHEN raw_album IS NOT NULL AND raw_album != '' THEN raw_album
			END) AS total_albums,
			COUNT(DISTINCT COALESCE(track_id, raw_artist || ' - ' || raw_title)) AS total_tracks
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
		return nil, fmt.Errorf("gagal query statistik rentang tanggal: %w", err)
	}

	return &res, nil
}
