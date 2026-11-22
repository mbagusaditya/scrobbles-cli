package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mbagusaditya/scrobbles-cli/internal/model"
)

// ResolverRepository menangani pemetaan scrobble_logs mentah ke tabel master artists dan tracks.
type ResolverRepository interface {
	CountUnlinked(ctx context.Context) (int, error)
	FetchUnlinkedBatch(ctx context.Context, limit int) ([]model.UnresolvedTrackItem, error)
	LinkUnresolvedBatch(ctx context.Context, items []model.UnresolvedTrackItem) (model.BatchLinkResult, error)
}

var _ ResolverRepository = (*sqlResolverRepository)(nil)

type sqlResolverRepository struct {
	db *sql.DB
}

func NewResolverRepository(db *sql.DB) ResolverRepository {
	return &sqlResolverRepository{db: db}
}

type trackLookupKey struct {
	artistID string
	title    string
	album    string
}

func (r *sqlResolverRepository) CountUnlinked(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM scrobble_logs WHERE track_id IS NULL").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("gagal menghitung sisa scrobble unlinked: %w", err)
	}
	return count, nil
}

func (r *sqlResolverRepository) FetchUnlinkedBatch(ctx context.Context, limit int) ([]model.UnresolvedTrackItem, error) {
	const query = `
		SELECT DISTINCT raw_artist, raw_title, COALESCE(raw_album, '')
		FROM scrobble_logs
		WHERE track_id IS NULL
		LIMIT ?
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("gagal query unresolved scrobbles: %w", err)
	}
	defer rows.Close()

	var items []model.UnresolvedTrackItem
	for rows.Next() {
		var it model.UnresolvedTrackItem
		if err := rows.Scan(&it.RawArtist, &it.RawTitle, &it.RawAlbum); err != nil {
			return nil, fmt.Errorf("gagal scan baris unresolved: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (r *sqlResolverRepository) LinkUnresolvedBatch(ctx context.Context, items []model.UnresolvedTrackItem) (model.BatchLinkResult, error) {
	var res model.BatchLinkResult
	if len(items) == 0 {
		return res, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return res, err
	}
	defer tx.Rollback()

	// 1. Preload master artists & tracks ke map memori (2 SELECT saja, nol N+1 query)
	artistMap, err := loadArtistMapTx(ctx, tx)
	if err != nil {
		return res, err
	}
	trackMap, err := loadTrackMapTx(ctx, tx)
	if err != nil {
		return res, err
	}

	now := time.Now().Unix()

	for _, it := range items {
		// 2. Pastikan artist ada
		artistID, exists := artistMap[it.RawArtist]
		if !exists {
			artistID = uuid.NewString()
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO artists (id, name, created_at) VALUES (?, ?, ?)`, artistID, it.RawArtist, now); err != nil {
				return res, err
			}
			artistMap[it.RawArtist] = artistID
			res.NewArtists++
		}

		// 3. Pastikan track ada
		key := trackLookupKey{artistID: artistID, title: it.RawTitle, album: it.RawAlbum}
		trackID, exists := trackMap[key]
		if !exists {
			trackID = uuid.NewString()
			var albumVal sql.NullString
			if strings.TrimSpace(it.RawAlbum) != "" {
				albumVal = sql.NullString{String: it.RawAlbum, Valid: true}
			}
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO tracks (id, artist_id, title, album_name, created_at) VALUES (?, ?, ?, ?, ?)`, trackID, artistID, it.RawTitle, albumVal, now); err != nil {
				return res, err
			}
			trackMap[key] = trackID
			res.NewTracks++
		}

		// 4. Tautkan track_id pada scrobble_logs
		upd, err := tx.ExecContext(ctx, `
			UPDATE scrobble_logs SET track_id = ?
			WHERE raw_artist = ? AND raw_title = ?
			  AND (raw_album = ? OR (raw_album IS NULL AND ? = ''))
			  AND track_id IS NULL
		`, trackID, it.RawArtist, it.RawTitle, it.RawAlbum, it.RawAlbum)
		if err != nil {
			return res, fmt.Errorf("gagal update track_id di scrobble_logs: %w", err)
		}
		if affected, _ := upd.RowsAffected(); affected > 0 {
			res.UpdatedLogs += int(affected)
		}
	}

	if err := tx.Commit(); err != nil {
		return res, err
	}
	return res, nil
}

func loadArtistMapTx(ctx context.Context, tx *sql.Tx) (map[string]string, error) {
	rows, err := tx.QueryContext(ctx, "SELECT id, name FROM artists")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[string]string)
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		m[name] = id
	}
	return m, rows.Err()
}

func loadTrackMapTx(ctx context.Context, tx *sql.Tx) (map[trackLookupKey]string, error) {
	rows, err := tx.QueryContext(ctx, "SELECT id, artist_id, title, COALESCE(album_name, '') FROM tracks")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[trackLookupKey]string)
	for rows.Next() {
		var id string
		var k trackLookupKey
		if err := rows.Scan(&id, &k.artistID, &k.title, &k.album); err != nil {
			return nil, err
		}
		m[k] = id
	}
	return m, rows.Err()
}
