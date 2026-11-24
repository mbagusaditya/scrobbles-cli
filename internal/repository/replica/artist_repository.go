package replica

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
)

// ListArtistsByIDs hanya mengambil baris artists yang ID-nya ada dalam daftar ids.
func (r *Repository) ListArtistsByIDs(ctx context.Context, ids []string) ([]model.ReplicaArtist, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var artists []model.ReplicaArtist
	for i := 0; i < len(ids); i += inClauseChunkSize {
		end := min(i+inClauseChunkSize, len(ids))
		chunk := ids[i:end]

		query := fmt.Sprintf(
			`SELECT id, name, mbid, image_url, created_at FROM artists WHERE id IN (%s)`,
			placeholders(len(chunk)),
		)
		rows, err := r.db.QueryContext(ctx, query, toAnySlice(chunk)...)
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var a model.ReplicaArtist
			var mbid, img sql.NullString
			if err := rows.Scan(&a.ID, &a.Name, &mbid, &img, &a.CreatedAt); err != nil {
				_ = rows.Close()
				return nil, err
			}
			a.MBID = mbid.String
			a.ImageURL = img.String
			artists = append(artists, a)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
	}

	return artists, nil
}

// GetArtistNameMapByNames memuat peta nama artis -> ID khusus untuk daftar nama yang diberikan.
func (r *Repository) GetArtistNameMapByNames(ctx context.Context, names []string) (map[string]string, error) {
	result := make(map[string]string, len(names))
	if len(names) == 0 {
		return result, nil
	}

	for i := 0; i < len(names); i += inClauseChunkSize {
		end := min(i+inClauseChunkSize, len(names))
		chunk := names[i:end]

		query := fmt.Sprintf(
			`SELECT id, name FROM artists WHERE name IN (%s)`,
			placeholders(len(chunk)),
		)
		rows, err := r.db.QueryContext(ctx, query, toAnySlice(chunk)...)
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var id, name string
			if err := rows.Scan(&id, &name); err != nil {
				_ = rows.Close()
				return nil, err
			}
			result[name] = id
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
	}

	return result, nil
}

// UpsertArtistsBatch menyisipkan artis baru dan memperkaya metadata kosong (mbid/image_url)
// dalam satu transaksi database.
func (r *Repository) UpsertArtistsBatch(ctx context.Context, batch []model.ReplicaArtist) error {
	if len(batch) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO artists (id, name, mbid, image_url, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			mbid = COALESCE(NULLIF(artists.mbid, ''), excluded.mbid),
			image_url = COALESCE(NULLIF(artists.image_url, ''), excluded.image_url)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, a := range batch {
		if _, err := stmt.ExecContext(
			ctx,
			a.ID,
			a.Name,
			toNullString(a.MBID),
			toNullString(a.ImageURL),
			a.CreatedAt,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}
