package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
)

// TopFilter menampung rentang waktu dan limit baris untuk kalkulasi top items.
type TopFilter struct {
	From  time.Time
	To    time.Time
	Limit int
}

// GetTopTracks mengambil daftar lagu (track) yang paling sering didengarkan beserta nama artisnya.
func (s *ScrobbleService) GetTopTracks(ctx context.Context, filter TopFilter) ([]model.TopItem, error) {
	if filter.Limit <= 0 {
		filter.Limit = 10
	}

	var conditions []string
	var args []any

	if !filter.From.IsZero() {
		conditions = append(conditions, "played_at >= ?")
		args = append(args, filter.From.Unix())
	}
	if !filter.To.IsZero() {
		conditions = append(conditions, "played_at <= ?")
		args = append(args, filter.To.Unix())
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT raw_title, raw_artist, COUNT(*) as play_count
		FROM scrobble_logs
		%s
		GROUP BY raw_title, raw_artist
		ORDER BY play_count DESC, raw_title ASC
		LIMIT ?
	`, whereClause)

	args = append(args, filter.Limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("gagal query top tracks: %w", err)
	}
	defer rows.Close()

	var results []model.TopItem
	for rows.Next() {
		var item model.TopItem
		if err := rows.Scan(&item.Name, &item.Detail, &item.Count); err != nil {
			return nil, fmt.Errorf("gagal membaca baris top tracks: %w", err)
		}
		results = append(results, item)
	}

	return results, rows.Err()
}

// GetTopArtists mengambil daftar artis yang paling sering didengarkan berdasarkan rentang waktu.
func (s *ScrobbleService) GetTopArtists(ctx context.Context, filter TopFilter) ([]model.TopItem, error) {
	if filter.Limit <= 0 {
		filter.Limit = 10
	}

	var conditions []string
	var args []any

	if !filter.From.IsZero() {
		conditions = append(conditions, "played_at >= ?")
		args = append(args, filter.From.Unix())
	}
	if !filter.To.IsZero() {
		conditions = append(conditions, "played_at <= ?")
		args = append(args, filter.To.Unix())
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT raw_artist, COUNT(*) as play_count
		FROM scrobble_logs
		%s
		GROUP BY raw_artist
		ORDER BY play_count DESC, raw_artist ASC
		LIMIT ?
	`, whereClause)

	args = append(args, filter.Limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("gagal query top artists: %w", err)
	}
	defer rows.Close()

	var results []model.TopItem
	for rows.Next() {
		var item model.TopItem
		if err := rows.Scan(&item.Name, &item.Count); err != nil {
			return nil, fmt.Errorf("gagal membaca baris top artists: %w", err)
		}
		results = append(results, item)
	}

	return results, rows.Err()
}

// GetTopAlbums mengambil daftar album yang paling sering didengarkan berdasarkan rentang waktu.
// Mengabaikan album yang NULL atau string kosong.
func (s *ScrobbleService) GetTopAlbums(ctx context.Context, filter TopFilter) ([]model.TopItem, error) {
	if filter.Limit <= 0 {
		filter.Limit = 10
	}

	// Filter dasar: raw_album tidak boleh NULL atau kosong
	conditions := []string{"raw_album IS NOT NULL", "raw_album != ''"}
	var args []any

	if !filter.From.IsZero() {
		conditions = append(conditions, "played_at >= ?")
		args = append(args, filter.From.Unix())
	}
	if !filter.To.IsZero() {
		conditions = append(conditions, "played_at <= ?")
		args = append(args, filter.To.Unix())
	}

	whereClause := "WHERE " + strings.Join(conditions, " AND ")

	query := fmt.Sprintf(`
		SELECT raw_album, COUNT(*) as play_count
		FROM scrobble_logs
		%s
		GROUP BY raw_album
		ORDER BY play_count DESC, raw_album ASC
		LIMIT ?
	`, whereClause)

	args = append(args, filter.Limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("gagal query top albums: %w", err)
	}
	defer rows.Close()

	var results []model.TopItem
	for rows.Next() {
		var item model.TopItem
		if err := rows.Scan(&item.Name, &item.Count); err != nil {
			return nil, fmt.Errorf("gagal membaca baris top albums: %w", err)
		}
		results = append(results, item)
	}

	return results, rows.Err()
}
