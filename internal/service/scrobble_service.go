package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/mbagusaditya/scrobbles-cli/internal/lastfm"
	"github.com/mbagusaditya/scrobbles-cli/internal/model"
)

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
	result := &SyncResult{}
	page := 1

	for {
		resp, err := s.client.GetRecentTracks(ctx, from, to, page)
		if err != nil {
			return nil, fmt.Errorf("gagal fetch halaman %d dari Last.fm: %w", page, err)
		}

		scrobbles := mapTracksToScrobbles(resp.RecentTracks.Tracks)
		result.Fetched += len(scrobbles)

		inserted, skipped, err := s.insertScrobbles(ctx, scrobbles)
		if err != nil {
			return nil, fmt.Errorf("gagal insert batch halaman %d: %w", page, err)
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
