package replica

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
)

// ListTracksByIDs hanya mengambil baris tracks yang ID-nya ada dalam daftar ids.
func (r *Repository) ListTracksByIDs(ctx context.Context, ids []string) ([]model.ReplicaTrack, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var tracks []model.ReplicaTrack
	for i := 0; i < len(ids); i += inClauseChunkSize {
		end := min(i+inClauseChunkSize, len(ids))
		chunk := ids[i:end]

		query := fmt.Sprintf(
			`SELECT id, artist_id, title, album_name, duration_ms, created_at, cover_url FROM tracks WHERE id IN (%s)`,
			placeholders(len(chunk)),
		)
		rows, err := r.db.QueryContext(ctx, query, toAnySlice(chunk)...)
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var t model.ReplicaTrack
			var album, cover sql.NullString
			var dur sql.NullInt64
			if err := rows.Scan(&t.ID, &t.ArtistID, &t.Title, &album, &dur, &t.CreatedAt, &cover); err != nil {
				_ = rows.Close()
				return nil, err
			}
			t.AlbumName = album.String
			t.CoverURL = cover.String
			t.DurationMs = dur.Int64
			t.HasDur = dur.Valid
			tracks = append(tracks, t)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
	}

	return tracks, nil
}

// GetTrackNaturalMapByArtists memuat peta (ArtistID, Title, Album) -> TrackID
// khusus untuk daftar target artistIDs yang terlibat dalam batch sinkronisasi.
func (r *Repository) GetTrackNaturalMapByArtists(ctx context.Context, artistIDs []string) (map[model.TrackNaturalKey]string, error) {
	result := make(map[model.TrackNaturalKey]string)
	if len(artistIDs) == 0 {
		return result, nil
	}

	for i := 0; i < len(artistIDs); i += inClauseChunkSize {
		end := min(i+inClauseChunkSize, len(artistIDs))
		chunk := artistIDs[i:end]

		query := fmt.Sprintf(
			`SELECT id, artist_id, title, COALESCE(album_name, '') FROM tracks WHERE artist_id IN (%s)`,
			placeholders(len(chunk)),
		)
		rows, err := r.db.QueryContext(ctx, query, toAnySlice(chunk)...)
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var id, artistID, title, album string
			if err := rows.Scan(&id, &artistID, &title, &album); err != nil {
				_ = rows.Close()
				return nil, err
			}
			result[model.TrackNaturalKey{
				ArtistID: artistID,
				Title:    title,
				Album:    album,
			}] = id
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
	}

	return result, nil
}

// UpsertTracksBatch menyisipkan track baru dan memperkaya duration_ms/cover_url yang masih kosong
// dalam satu transaksi database.
func (r *Repository) UpsertTracksBatch(ctx context.Context, toInsert []model.ReplicaTrack, toEnrich []model.ReplicaTrack) error {
	if len(toInsert) == 0 && len(toEnrich) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if len(toInsert) > 0 {
		insertStmt, err := tx.PrepareContext(ctx, `
			INSERT INTO tracks (id, artist_id, title, album_name, duration_ms, created_at, cover_url)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			return err
		}
		defer insertStmt.Close()

		for _, t := range toInsert {
			var dur sql.NullInt64
			if t.HasDur {
				dur = sql.NullInt64{Int64: t.DurationMs, Valid: true}
			}
			if _, err := insertStmt.ExecContext(
				ctx,
				t.ID,
				t.ArtistID,
				t.Title,
				toNullString(t.AlbumName),
				dur,
				t.CreatedAt,
				toNullString(t.CoverURL),
			); err != nil {
				return err
			}
		}
	}

	if len(toEnrich) > 0 {
		enrichStmt, err := tx.PrepareContext(ctx, `
			UPDATE tracks SET
				duration_ms = COALESCE(duration_ms, ?),
				cover_url = COALESCE(NULLIF(cover_url, ''), ?)
			WHERE id = ?
		`)
		if err != nil {
			return err
		}
		defer enrichStmt.Close()

		for _, t := range toEnrich {
			var dur sql.NullInt64
			if t.HasDur {
				dur = sql.NullInt64{Int64: t.DurationMs, Valid: true}
			}
			if _, err := enrichStmt.ExecContext(ctx, dur, toNullString(t.CoverURL), t.ID); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}
