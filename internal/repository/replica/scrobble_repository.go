package replica

import (
	"context"
	"database/sql"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
)

// ListScrobbleLogsSince mengambil scrobble_logs mulai dari minPlayedAt (atau seluruhnya jika minPlayedAt <= 0).
func (r *Repository) ListScrobbleLogsSince(ctx context.Context, minPlayedAt int64) ([]model.ReplicaScrobbleLog, error) {
	var rows *sql.Rows
	var err error

	if minPlayedAt > 0 {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, track_id, raw_title, raw_artist, raw_album, played_at, created_at
			FROM scrobble_logs
			WHERE played_at >= ?
			ORDER BY played_at ASC
		`, minPlayedAt)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, track_id, raw_title, raw_artist, raw_album, played_at, created_at
			FROM scrobble_logs
			ORDER BY played_at ASC
		`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []model.ReplicaScrobbleLog
	for rows.Next() {
		var l model.ReplicaScrobbleLog
		var trackID, rawAlbum sql.NullString
		if err := rows.Scan(&l.ID, &trackID, &l.RawTitle, &l.RawArtist, &rawAlbum, &l.PlayedAt, &l.CreatedAt); err != nil {
			return nil, err
		}
		l.TrackID = trackID.String
		l.RawAlbum = rawAlbum.String
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// UpsertScrobbleLogsBatch menyisipkan scrobble baru dan menautkan track_id bila sebelumnya NULL.
func (r *Repository) UpsertScrobbleLogsBatch(ctx context.Context, batch []model.ReplicaScrobbleLog) (int, error) {
	if len(batch) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO scrobble_logs (id, track_id, raw_title, raw_artist, raw_album, played_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(raw_artist, raw_title, played_at) DO UPDATE SET
			track_id = COALESCE(scrobble_logs.track_id, excluded.track_id)
	`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	affected := 0
	for _, l := range batch {
		res, err := stmt.ExecContext(
			ctx,
			l.ID,
			toNullString(l.TrackID),
			l.RawTitle,
			l.RawArtist,
			toNullString(l.RawAlbum),
			l.PlayedAt,
			l.CreatedAt,
		)
		if err != nil {
			return 0, err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			affected++
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return affected, nil
}
