package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/mbagusaditya/scrobbles-cli/internal/lastfm"
	"github.com/mbagusaditya/scrobbles-cli/internal/model"
)

type ListFilter struct {
	Artist string
	Album  string
	From   time.Time
	To     time.Time
	Limit  int
	Page   int
}

type ScrobbleService struct {
	db     *sql.DB
	client *lastfm.Client
}

func NewScrobbleService(database *sql.DB, client *lastfm.Client) *ScrobbleService {
	return &ScrobbleService{db: database, client: client}
}

// MaxCustomRangeDays adalah batas maksimal rentang tanggal yang boleh
// diminta untuk custom range sync (dipakai untuk validasi di cmd).
const MaxCustomRangeDays = 7

type SyncResult struct {
	Inserted int
	Skipped  int
	Fetched  int
}

// PageProgress berisi ringkasan hasil pemrosesan satu halaman, dikirim
// ke ProgressFunc setelah tiap halaman selesai diambil dan disimpan.
type PageProgress struct {
	Page       int
	TotalPages int
	Fetched    int // jumlah track valid (non now-playing) di halaman ini
	Inserted   int
	Skipped    int
}

// ProgressFunc dipanggil setelah tiap halaman selesai diproses selama
// FetchRange/SyncLatest berjalan. Boleh nil kalau tidak butuh feedback.
// Service sengaja tidak melakukan print apapun sendiri -- itu tanggung
// jawab pemanggil (cmd), service cuma melaporkan progress lewat callback.
type ProgressFunc func(p PageProgress)

// SyncLatest mengambil scrobble baru dari Last.fm sejak scrobble terakhir
// yang tersimpan di database sampai saat ini. Kalau database masih kosong
// (belum pernah sync sama sekali), otomatis mengambil dari awal history.
func (s *ScrobbleService) SyncLatest(ctx context.Context, onProgress ProgressFunc) (*SyncResult, error) {
	lastPlayedAt, err := s.getLastPlayedAt(ctx)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil timestamp scrobble terakhir: %w", err)
	}

	return s.FetchRange(ctx, lastPlayedAt, time.Now(), onProgress)
}

// FetchRange mengambil seluruh scrobble dalam rentang [from, to] dari
// Last.fm, melakukan pagination otomatis sampai seluruh halaman habis,
// dan menyimpan hasilnya (duplikat otomatis di-skip). Ini adalah inti
// mekanisme yang dipakai oleh semua mode sync (harian, mingguan, custom
// range, maupun incremental via SyncLatest).
//
// Kirim time.Time{} (zero value) pada from untuk tidak membatasi batas
// bawah (ambil dari awal history). onProgress boleh nil.
func (s *ScrobbleService) FetchRange(ctx context.Context, from, to time.Time, onProgress ProgressFunc) (*SyncResult, error) {
	// Jika from kosong (misal user hanya isi --to), ambil titik awal dari scrobble terakhir di DB
	if from.IsZero() {
		lastPlayed, err := s.getLastPlayedAt(ctx)
		if err != nil {
			return nil, fmt.Errorf("gagal mengambil timestamp scrobble terakhir: %w", err)
		}
		from = lastPlayed
	}

	result := &SyncResult{}
	page := 1

	for {
		resp, err := s.client.GetRecentTracks(ctx, from, to, page)
		if err != nil {
			return result, fmt.Errorf("gagal fetch halaman %d dari Last.fm: %w", page, err)
		}

		// Jeda 500ms setelah fetch data agar mematuhi rate limit Last.fm
		select {
		case <-time.After(500 * time.Millisecond):
		case <-ctx.Done():
			return result, ctx.Err()
		}

		scrobbles := mapTracksToScrobbles(resp.RecentTracks.Tracks)
		result.Fetched += len(scrobbles)

		// Insert dan commit langsung per halaman (batch)
		inserted, skipped, err := s.insertScrobbles(ctx, scrobbles)
		if err != nil {
			return result, fmt.Errorf("gagal insert batch halaman %d: %w", page, err)
		}
		result.Inserted += inserted
		result.Skipped += skipped

		totalPages := resp.RecentTracks.Attr.TotalPagesInt()

		if onProgress != nil {
			onProgress(PageProgress{
				Page:       page,
				TotalPages: totalPages,
				Fetched:    len(scrobbles),
				Inserted:   inserted,
				Skipped:    skipped,
			})
		}

		if page >= totalPages {
			break
		}
		page++
	}

	return result, nil
}

func mapTracksToScrobbles(tracks lastfm.TrackList) []*model.Scrobble {
	scrobbles := make([]*model.Scrobble, 0, len(tracks))

	for _, t := range tracks {
		if t.IsNowPlaying() {
			continue
		}

		playedAt, err := t.PlayedAt()
		if err != nil {
			continue
		}

		scrobbles = append(scrobbles, model.NewScrobble(
			t.Name,
			t.Artist.Text,
			t.Album.Text,
			playedAt,
		))
	}

	return scrobbles
}

func (s *ScrobbleService) insertScrobbles(ctx context.Context, scrobbles []*model.Scrobble) (inserted int, skipped int, err error) {
	if len(scrobbles) == 0 {
		return 0, 0, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("gagal memulai transaction: %w", err)
	}
	defer tx.Rollback()

	for _, sc := range scrobbles {
		ok, insertErr := s.insertScrobble(ctx, tx, sc)
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

func (s *ScrobbleService) insertScrobble(ctx context.Context, tx *sql.Tx, sc *model.Scrobble) (bool, error) {
	const query = `
		INSERT OR IGNORE INTO scrobble_logs
			(id, track_id, raw_title, raw_artist, raw_album, played_at, created_at)
		VALUES
			(?, ?, ?, ?, ?, ?, ?)
	`

	result, err := tx.ExecContext(ctx, query,
		sc.ID,
		sc.TrackID,
		sc.RawTitle,
		sc.RawArtist,
		sc.RawAlbum,
		sc.PlayedAt.Unix(),
		sc.CreatedAt.Unix(),
	)
	if err != nil {
		return false, fmt.Errorf("gagal insert scrobble %q by %q: %w", sc.RawTitle, sc.RawArtist, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("gagal membaca rows affected: %w", err)
	}

	return affected > 0, nil
}

func (s *ScrobbleService) getLastPlayedAt(ctx context.Context) (time.Time, error) {
	const query = `SELECT MAX(played_at) FROM scrobble_logs`

	var maxPlayedAt sql.NullInt64
	if err := s.db.QueryRowContext(ctx, query).Scan(&maxPlayedAt); err != nil {
		return time.Time{}, fmt.Errorf("gagal query last played_at: %w", err)
	}

	if !maxPlayedAt.Valid {
		return time.Time{}, nil
	}

	return time.Unix(maxPlayedAt.Int64, 0).UTC(), nil
}

func (s *ScrobbleService) ListScrobbles(ctx context.Context, filter ListFilter) ([]*model.Scrobble, error) {
	// 1. Normalisasi limit dan page ke offset SQL
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	} else if limit > 200 {
		limit = 200
	}

	page := filter.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	// 2. Susun dynamic SQL query dan binding argumen
	query := `
		SELECT id, track_id, raw_title, raw_artist, raw_album, played_at, created_at
		FROM scrobble_logs
		WHERE 1=1
	`
	var args []any

	if filter.Artist != "" {
		query += " AND raw_artist = ?"
		args = append(args, filter.Artist)
	}

	if filter.Album != "" {
		query += " AND raw_album = ?"
		args = append(args, filter.Album)
	}

	if !filter.From.IsZero() {
		query += " AND played_at >= ?"
		args = append(args, filter.From.Unix())
	}

	if !filter.To.IsZero() {
		query += " AND played_at <= ?"
		args = append(args, filter.To.Unix())
	}

	query += " ORDER BY played_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	// 3. Eksekusi query
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("gagal query list scrobbles: %w", err)
	}
	defer rows.Close()

	// 4. Scan baris ke struct model.Scrobble
	var results []*model.Scrobble
	for rows.Next() {
		var (
			sc          model.Scrobble
			playedUnix  int64
			createdUnix int64
		)

		err := rows.Scan(
			&sc.ID,
			&sc.TrackID,
			&sc.RawTitle,
			&sc.RawArtist,
			&sc.RawAlbum,
			&playedUnix,
			&createdUnix,
		)
		if err != nil {
			return nil, fmt.Errorf("gagal scan scrobble: %w", err)
		}

		sc.PlayedAt = time.Unix(playedUnix, 0).Local()
		sc.CreatedAt = time.Unix(createdUnix, 0).Local()

		results = append(results, &sc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterasi baris scrobbles: %w", err)
	}

	return results, nil
}
