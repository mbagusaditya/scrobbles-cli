package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
)

// InsertScrobble menyisipkan satu scrobble ke tabel scrobble_logs.
// Menggunakan INSERT OR IGNORE sehingga aman dipanggil berulang kali
// dengan data yang sama (duplikat akan otomatis di-skip oleh unique
// index idx_scrobble_logs_unique_event pada level database).
//
// Return value bool menandakan apakah baris benar-benar disisipkan
// (true) atau di-skip karena sudah ada sebelumnya (false).
func InsertScrobble(ctx context.Context, tx *sql.Tx, s *model.Scrobble) (bool, error) {
	const query = `
		INSERT OR IGNORE INTO scrobble_logs
			(id, track_id, raw_title, raw_artist, raw_album, played_at, created_at)
		VALUES
			(?, ?, ?, ?, ?, ?, ?)
	`

	result, err := tx.ExecContext(ctx, query,
		s.ID,
		s.TrackID,
		s.RawTitle,
		s.RawArtist,
		s.RawAlbum,
		s.PlayedAt.Unix(),
		s.CreatedAt.Unix(),
	)
	if err != nil {
		return false, fmt.Errorf("gagal insert scrobble %q by %q: %w", s.RawTitle, s.RawArtist, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("gagal membaca rows affected: %w", err)
	}

	return affected > 0, nil
}

// InsertScrobbles menyisipkan banyak scrobble sekaligus dalam satu
// transaction (lebih cepat daripada insert satu-satu di luar transaction).
// Mengembalikan jumlah baris yang berhasil disisipkan dan jumlah yang
// di-skip karena duplikat.
func InsertScrobbles(ctx context.Context, database *sql.DB, scrobbles []*model.Scrobble) (inserted int, skipped int, err error) {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("gagal memulai transaction: %w", err)
	}
	defer tx.Rollback() // no-op kalau sudah di-Commit

	for _, s := range scrobbles {
		ok, insertErr := InsertScrobble(ctx, tx, s)
		if insertErr != nil {
			return 0, 0, insertErr
		}
		if ok {
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

// GetLastPlayedAt mengembalikan timestamp scrobble terbaru yang sudah
// tersimpan di database. Dipakai untuk incremental sync (parameter
// "from" saat memanggil Last.fm API).
//
// Mengembalikan time.Time{} (zero value) kalau tabel masih kosong
// (belum ada scrobble sama sekali) -> caller harus treat ini sebagai
// sinyal untuk full backfill dari awal.
func GetLastPlayedAt(ctx context.Context, database *sql.DB) (time.Time, error) {
	const query = `SELECT MAX(played_at) FROM scrobble_logs`

	var maxPlayedAt sql.NullInt64
	if err := database.QueryRowContext(ctx, query).Scan(&maxPlayedAt); err != nil {
		return time.Time{}, fmt.Errorf("gagal query last played_at: %w", err)
	}

	if !maxPlayedAt.Valid {
		return time.Time{}, nil
	}

	return time.Unix(maxPlayedAt.Int64, 0).UTC(), nil
}
