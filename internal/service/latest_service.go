package service

import (
	"context"
	"fmt"
	"time"
)

// LatestItem merepresentasikan riwayat pemutaran terakhir dari artis, album, atau trek.
type LatestItem struct {
	Name     string
	Detail   string // Nama artis (jika targetnya track/album)
	PlayedAt time.Time
}

// GetLatestArtists mengambil n artis unik terakhir yang diputar.
func (s *ScrobbleService) GetLatestArtists(ctx context.Context, limit int) ([]LatestItem, error) {
	if limit <= 0 {
		limit = 10
	}

	// Mengambil waktu terakhir artis diputar dengan agregasi MAX(played_at)
	query := `
		SELECT raw_artist, MAX(played_at) as last_played
		FROM scrobble_logs
		GROUP BY raw_artist
		ORDER BY last_played DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("gagal query latest artists: %w", err)
	}
	defer rows.Close()

	var items []LatestItem
	for rows.Next() {
		var name string
		var epoch int64
		if err := rows.Scan(&name, &epoch); err != nil {
			return nil, fmt.Errorf("gagal membaca baris latest artist: %w", err)
		}
		items = append(items, LatestItem{
			Name:     name,
			PlayedAt: time.Unix(epoch, 0).Local(),
		})
	}

	return items, rows.Err()
}

// GetLatestAlbums mengambil n album unik terakhir yang diputar.
func (s *ScrobbleService) GetLatestAlbums(ctx context.Context, limit int) ([]LatestItem, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT raw_album, raw_artist, MAX(played_at) as last_played
		FROM scrobble_logs
		WHERE raw_album IS NOT NULL AND raw_album != ''
		GROUP BY raw_album, raw_artist
		ORDER BY last_played DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("gagal query latest albums: %w", err)
	}
	defer rows.Close()

	var items []LatestItem
	for rows.Next() {
		var album, artist string
		var epoch int64
		if err := rows.Scan(&album, &artist, &epoch); err != nil {
			return nil, fmt.Errorf("gagal membaca baris latest album: %w", err)
		}
		items = append(items, LatestItem{
			Name:     album,
			Detail:   artist,
			PlayedAt: time.Unix(epoch, 0).Local(),
		})
	}

	return items, rows.Err()
}

// GetLatestTracks mengambil n trek unik terakhir yang diputar.
func (s *ScrobbleService) GetLatestTracks(ctx context.Context, limit int) ([]LatestItem, error) {
	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT raw_title, raw_artist, MAX(played_at) as last_played
		FROM scrobble_logs
		GROUP BY raw_title, raw_artist
		ORDER BY last_played DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("gagal query latest tracks: %w", err)
	}
	defer rows.Close()

	var items []LatestItem
	for rows.Next() {
		var track, artist string
		var epoch int64
		if err := rows.Scan(&track, &artist, &epoch); err != nil {
			return nil, fmt.Errorf("gagal membaca baris latest track: %w", err)
		}
		items = append(items, LatestItem{
			Name:     track,
			Detail:   artist,
			PlayedAt: time.Unix(epoch, 0).Local(),
		})
	}

	return items, rows.Err()
}
