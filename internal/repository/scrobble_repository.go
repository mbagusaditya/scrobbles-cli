package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
)

// ScrobbleRepository menangani persistensi dan pembacaan log riwayat scrobble mentah.
type ScrobbleRepository interface {
	GetLastPlayedAt(ctx context.Context) (time.Time, error)
	InsertBatch(ctx context.Context, scrobbles []*model.Scrobble) (inserted int, skipped int, err error)
	List(ctx context.Context, filter model.ScrobbleListFilter) ([]*model.Scrobble, error)
}

var _ ScrobbleRepository = (*sqlScrobbleRepository)(nil)

type sqlScrobbleRepository struct {
	db *sql.DB
}

func NewScrobbleRepository(db *sql.DB) ScrobbleRepository {
	return &sqlScrobbleRepository{db: db}
}

func (r *sqlScrobbleRepository) GetLastPlayedAt(ctx context.Context) (time.Time, error) {
	const query = `SELECT MAX(played_at) FROM scrobble_logs`

	var maxPlayedAt sql.NullInt64
	if err := r.db.QueryRowContext(ctx, query).Scan(&maxPlayedAt); err != nil {
		return time.Time{}, fmt.Errorf("gagal query last played_at: %w", err)
	}

	if !maxPlayedAt.Valid {
		return time.Time{}, nil
	}
	return time.Unix(maxPlayedAt.Int64, 0).UTC(), nil
}

func (r *sqlScrobbleRepository) InsertBatch(ctx context.Context, scrobbles []*model.Scrobble) (inserted int, skipped int, err error) {
	if len(scrobbles) == 0 {
		return 0, 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("gagal memulai transaction: %w", err)
	}
	defer tx.Rollback()

	const query = `
		INSERT OR IGNORE INTO scrobble_logs
			(id, track_id, raw_title, raw_artist, raw_album, played_at, created_at)
		VALUES
			(?, ?, ?, ?, ?, ?, ?)
	`
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return 0, 0, fmt.Errorf("gagal menyiapkan statement insert scrobble: %w", err)
	}
	defer stmt.Close()

	for _, sc := range scrobbles {
		res, execErr := stmt.ExecContext(ctx,
			sc.ID,
			sc.TrackID,
			sc.RawTitle,
			sc.RawArtist,
			sc.RawAlbum,
			sc.PlayedAt.Unix(),
			sc.CreatedAt.Unix(),
		)
		if execErr != nil {
			return 0, 0, fmt.Errorf("gagal insert scrobble %q by %q: %w", sc.RawTitle, sc.RawArtist, execErr)
		}

		affected, affErr := res.RowsAffected()
		if affErr != nil {
			return 0, 0, fmt.Errorf("gagal membaca rows affected: %w", affErr)
		}
		if affected > 0 {
			inserted++
		} else {
			skipped++
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("gagal commit transaction: %w", err)
	}
	return inserted, skipped, nil
}

func (r *sqlScrobbleRepository) List(ctx context.Context, filter model.ScrobbleListFilter) ([]*model.Scrobble, error) {
	var conds []string
	var args []any

	if filter.Artist != "" {
		conds = append(conds, "raw_artist = ? COLLATE NOCASE")
		args = append(args, filter.Artist)
	}
	if filter.Album != "" {
		conds = append(conds, "raw_album = ? COLLATE NOCASE")
		args = append(args, filter.Album)
	}
	if !filter.From.IsZero() {
		conds = append(conds, "played_at >= ?")
		args = append(args, filter.From.Unix())
	}
	if !filter.To.IsZero() {
		conds = append(conds, "played_at <= ?")
		args = append(args, filter.To.Unix())
	}

	whereClause := ""
	if len(conds) > 0 {
		whereClause = "WHERE " + strings.Join(conds, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, track_id, raw_title, raw_artist, raw_album, played_at, created_at
		FROM scrobble_logs
		%s
		ORDER BY played_at DESC
		LIMIT ? OFFSET ?
	`, whereClause)

	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("gagal query list scrobbles: %w", err)
	}
	defer rows.Close()

	var results []*model.Scrobble
	for rows.Next() {
		var sc model.Scrobble
		var playedUnix, createdUnix int64

		if err := rows.Scan(
			&sc.ID,
			&sc.TrackID,
			&sc.RawTitle,
			&sc.RawArtist,
			&sc.RawAlbum,
			&playedUnix,
			&createdUnix,
		); err != nil {
			return nil, fmt.Errorf("gagal scan scrobble: %w", err)
		}

		sc.PlayedAt = time.Unix(playedUnix, 0).Local()
		sc.CreatedAt = time.Unix(createdUnix, 0).Local()
		results = append(results, &sc)
	}

	return results, rows.Err()
}
