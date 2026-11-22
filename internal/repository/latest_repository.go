package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
)

// LatestRepository mendefinisikan kontrak kueri riwayat pemutaran terakhir per entitas.
type LatestRepository interface {
	GetLatestArtists(ctx context.Context, limit int) ([]model.LatestItem, error)
	GetLatestAlbums(ctx context.Context, limit int) ([]model.LatestItem, error)
	// GetAllTrackLastPlayed mengambil waktu putar terakhir per kombinasi unik (artist, title)
	// tanpa LIMIT SQL agar penggabungan varian alias di layer service akurat.
	GetAllTrackLastPlayed(ctx context.Context) ([]model.LatestItem, error)
}

var _ LatestRepository = (*sqlLatestRepository)(nil)

type sqlLatestRepository struct {
	db *sql.DB
}

// NewLatestRepository membuat implementasi LatestRepository berbasis *sql.DB (SQLite/Turso).
func NewLatestRepository(db *sql.DB) LatestRepository {
	return &sqlLatestRepository{db: db}
}

func (r *sqlLatestRepository) GetLatestArtists(ctx context.Context, limit int) ([]model.LatestItem, error) {
	const query = `
		SELECT MIN(TRIM(raw_artist)), '', MAX(played_at) AS last_played
		FROM scrobble_logs
		WHERE TRIM(raw_artist) != ''
		GROUP BY LOWER(TRIM(raw_artist))
		ORDER BY last_played DESC
		LIMIT ?
	`
	return r.queryLatestItems(ctx, query, limit)
}

func (r *sqlLatestRepository) GetLatestAlbums(ctx context.Context, limit int) ([]model.LatestItem, error) {
	const query = `
		SELECT MIN(TRIM(raw_album)), MIN(TRIM(raw_artist)), MAX(played_at) AS last_played
		FROM scrobble_logs
		WHERE raw_album IS NOT NULL AND TRIM(raw_album) != ''
		GROUP BY LOWER(TRIM(raw_album)), LOWER(TRIM(raw_artist))
		ORDER BY last_played DESC
		LIMIT ?
	`
	return r.queryLatestItems(ctx, query, limit)
}

func (r *sqlLatestRepository) GetAllTrackLastPlayed(ctx context.Context) ([]model.LatestItem, error) {
	const query = `
		SELECT MIN(TRIM(raw_title)), MIN(TRIM(raw_artist)), MAX(played_at) AS last_played
		FROM scrobble_logs
		WHERE TRIM(raw_title) != ''
		GROUP BY LOWER(TRIM(raw_title)), LOWER(TRIM(raw_artist))
	`
	return r.queryLatestItems(ctx, query)
}

func (r *sqlLatestRepository) queryLatestItems(ctx context.Context, query string, args ...any) ([]model.LatestItem, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("gagal query latest items: %w", err)
	}
	defer rows.Close()

	var items []model.LatestItem
	for rows.Next() {
		var name, detail string
		var epoch int64
		if err := rows.Scan(&name, &detail, &epoch); err != nil {
			return nil, fmt.Errorf("gagal membaca baris latest item: %w", err)
		}
		items = append(items, model.LatestItem{
			Name:     name,
			Detail:   detail,
			PlayedAt: time.Unix(epoch, 0).Local(),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterasi latest items: %w", err)
	}

	return items, nil
}
