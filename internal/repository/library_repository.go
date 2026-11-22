package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
)

// LibraryRepository mendefinisikan kontrak kueri katalog unik (artists, albums, tracks).
type LibraryRepository interface {
	CountUniqueArtists(ctx context.Context) (int, error)
	FetchPagedArtists(ctx context.Context, limit, offset int) ([]model.LibraryItem, error)

	CountUniqueAlbums(ctx context.Context) (int, error)
	FetchPagedAlbums(ctx context.Context, limit, offset int) ([]model.LibraryItem, error)

	// FetchAllTrackCounts mengambil agregasi mentah (title, artist, play_count)
	// agar dapat digabungkan berdasarkan aturan AliasService di layer service.
	FetchAllTrackCounts(ctx context.Context) ([]model.LibraryItem, error)
}

var _ LibraryRepository = (*sqlLibraryRepository)(nil)

type sqlLibraryRepository struct {
	db *sql.DB
}

// NewLibraryRepository membuat implementasi LibraryRepository berbasis *sql.DB (SQLite/Turso).
func NewLibraryRepository(db *sql.DB) LibraryRepository {
	return &sqlLibraryRepository{db: db}
}

func (r *sqlLibraryRepository) CountUniqueArtists(ctx context.Context) (int, error) {
	const query = `
		SELECT COUNT(DISTINCT LOWER(TRIM(raw_artist)))
		FROM scrobble_logs
		WHERE TRIM(raw_artist) != ''
	`
	var total int
	if err := r.db.QueryRowContext(ctx, query).Scan(&total); err != nil {
		return 0, fmt.Errorf("gagal menghitung total artists di pustaka: %w", err)
	}
	return total, nil
}

func (r *sqlLibraryRepository) FetchPagedArtists(ctx context.Context, limit, offset int) ([]model.LibraryItem, error) {
	const query = `
		SELECT MIN(TRIM(raw_artist)), COUNT(*) AS play_count
		FROM scrobble_logs
		WHERE TRIM(raw_artist) != ''
		GROUP BY LOWER(TRIM(raw_artist))
		ORDER BY MIN(TRIM(raw_artist)) COLLATE NOCASE ASC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("gagal query list library artists: %w", err)
	}
	defer rows.Close()

	var items []model.LibraryItem
	for rows.Next() {
		var item model.LibraryItem
		if err := rows.Scan(&item.Name, &item.TotalPlays); err != nil {
			return nil, fmt.Errorf("gagal scan baris library artist: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterasi library artists: %w", err)
	}
	return items, nil
}

func (r *sqlLibraryRepository) CountUniqueAlbums(ctx context.Context) (int, error) {
	const query = `
		SELECT COUNT(*) FROM (
			SELECT LOWER(TRIM(raw_album)), LOWER(TRIM(raw_artist))
			FROM scrobble_logs
			WHERE raw_album IS NOT NULL AND TRIM(raw_album) != ''
			GROUP BY LOWER(TRIM(raw_album)), LOWER(TRIM(raw_artist))
		)
	`
	var total int
	if err := r.db.QueryRowContext(ctx, query).Scan(&total); err != nil {
		return 0, fmt.Errorf("gagal menghitung total albums di pustaka: %w", err)
	}
	return total, nil
}

func (r *sqlLibraryRepository) FetchPagedAlbums(ctx context.Context, limit, offset int) ([]model.LibraryItem, error) {
	const query = `
		SELECT MIN(TRIM(raw_album)), MIN(TRIM(raw_artist)), COUNT(*) AS play_count
		FROM scrobble_logs
		WHERE raw_album IS NOT NULL AND TRIM(raw_album) != ''
		GROUP BY LOWER(TRIM(raw_album)), LOWER(TRIM(raw_artist))
		ORDER BY MIN(TRIM(raw_album)) COLLATE NOCASE ASC, MIN(TRIM(raw_artist)) COLLATE NOCASE ASC
		LIMIT ? OFFSET ?
	`
	return r.scanTrackOrAlbumRows(ctx, query, limit, offset)
}

func (r *sqlLibraryRepository) FetchAllTrackCounts(ctx context.Context) ([]model.LibraryItem, error) {
	const query = `
		SELECT MIN(TRIM(raw_title)), MIN(TRIM(raw_artist)), COUNT(*) AS play_count
		FROM scrobble_logs
		WHERE TRIM(raw_title) != ''
		GROUP BY LOWER(TRIM(raw_title)), LOWER(TRIM(raw_artist))
	`
	return r.scanTrackOrAlbumRows(ctx, query)
}

func (r *sqlLibraryRepository) scanTrackOrAlbumRows(ctx context.Context, query string, args ...any) ([]model.LibraryItem, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("gagal query library items: %w", err)
	}
	defer rows.Close()

	var items []model.LibraryItem
	for rows.Next() {
		var item model.LibraryItem
		if err := rows.Scan(&item.Name, &item.Detail, &item.TotalPlays); err != nil {
			return nil, fmt.Errorf("gagal scan baris library item: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterasi library items: %w", err)
	}
	return items, nil
}
