package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
)

// TopRepository menyediakan kueri agregasi peringkat teratas untuk track, artist, dan album.
type TopRepository interface {
	// FetchTrackCounts mengambil jumlah putar per (raw_title, raw_artist) dalam rentang waktu
	// tanpa LIMIT SQL agar varian alias dapat digabungkan di layer service.
	FetchTrackCounts(ctx context.Context, from, to time.Time) ([]model.TopItem, error)
	FetchTopArtists(ctx context.Context, from, to time.Time, limit int) ([]model.TopItem, error)
	FetchTopAlbums(ctx context.Context, from, to time.Time, limit int) ([]model.TopItem, error)
}

var _ TopRepository = (*sqlTopRepository)(nil)

type sqlTopRepository struct {
	db *sql.DB
}

func NewTopRepository(db *sql.DB) TopRepository {
	return &sqlTopRepository{db: db}
}

func (r *sqlTopRepository) FetchTrackCounts(ctx context.Context, from, to time.Time) ([]model.TopItem, error) {
	whereClause, args := buildTopWhere([]string{"TRIM(raw_title) != ''"}, from, to)

	query := fmt.Sprintf(`
		SELECT MIN(TRIM(raw_title)), MIN(TRIM(raw_artist)), COUNT(*) AS play_count
		FROM scrobble_logs
		%s
		GROUP BY LOWER(TRIM(raw_title)), LOWER(TRIM(raw_artist))
	`, whereClause)

	rows, err := r.db.QueryContext(ctx, query, args...)
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

func (r *sqlTopRepository) FetchTopArtists(ctx context.Context, from, to time.Time, limit int) ([]model.TopItem, error) {
	whereClause, args := buildTopWhere([]string{"TRIM(raw_artist) != ''"}, from, to)

	query := fmt.Sprintf(`
		SELECT MIN(TRIM(raw_artist)), COUNT(*) AS play_count
		FROM scrobble_logs
		%s
		GROUP BY LOWER(TRIM(raw_artist))
		ORDER BY play_count DESC, MIN(TRIM(raw_artist)) COLLATE NOCASE ASC
		LIMIT ?
	`, whereClause)

	args = append(args, limit)
	return r.queryTopPairs(ctx, query, args...)
}

func (r *sqlTopRepository) FetchTopAlbums(ctx context.Context, from, to time.Time, limit int) ([]model.TopItem, error) {
	whereClause, args := buildTopWhere([]string{"raw_album IS NOT NULL", "TRIM(raw_album) != ''"}, from, to)

	query := fmt.Sprintf(`
		SELECT MIN(TRIM(raw_album)), COUNT(*) AS play_count
		FROM scrobble_logs
		%s
		GROUP BY LOWER(TRIM(raw_album))
		ORDER BY play_count DESC, MIN(TRIM(raw_album)) COLLATE NOCASE ASC
		LIMIT ?
	`, whereClause)

	args = append(args, limit)
	return r.queryTopPairs(ctx, query, args...)
}

func (r *sqlTopRepository) queryTopPairs(ctx context.Context, query string, args ...any) ([]model.TopItem, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("gagal query top items: %w", err)
	}
	defer rows.Close()

	var results []model.TopItem
	for rows.Next() {
		var item model.TopItem
		if err := rows.Scan(&item.Name, &item.Count); err != nil {
			return nil, fmt.Errorf("gagal membaca baris top items: %w", err)
		}
		results = append(results, item)
	}
	return results, rows.Err()
}

func buildTopWhere(baseConds []string, from, to time.Time) (string, []any) {
	conds := append([]string{}, baseConds...)
	var args []any

	if !from.IsZero() {
		conds = append(conds, "played_at >= ?")
		args = append(args, from.Unix())
	}
	if !to.IsZero() {
		conds = append(conds, "played_at <= ?")
		args = append(args, to.Unix())
	}

	if len(conds) == 0 {
		return "", nil
	}
	return "WHERE " + strings.Join(conds, " AND "), args
}
