package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/mbagusaditya/scrobbles-cli/internal/lastfm"
	"github.com/mbagusaditya/scrobbles-cli/internal/model"
)

// ScrobbleService menangani seluruh business logic terkait scrobble:
// menyimpan ke database dan orkestrasi sync dari Last.fm API.
type ScrobbleService struct {
	db     *sql.DB
	client *lastfm.Client
}

// NewScrobbleService membuat instance ScrobbleService baru.
func NewScrobbleService(database *sql.DB, client *lastfm.Client) *ScrobbleService {
	return &ScrobbleService{db: database, client: client}
}

// SyncResult merangkum hasil satu kali proses sync.
type SyncResult struct {
	Inserted int
	Skipped  int
	Fetched  int
}

// Sync mengambil scrobble baru dari Last.fm sejak scrobble terakhir yang
// tersimpan di database, lalu menyimpannya. Melakukan pagination otomatis
// sampai seluruh halaman baru habis diambil.
func (s *ScrobbleService) Sync(ctx context.Context) (*SyncResult, error) {
	lastPlayedAt, err := s.getLastPlayedAt(ctx)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil timestamp scrobble terakhir: %w", err)
	}

	return s.syncFrom(ctx, lastPlayedAt)
}

// SyncFrom mengambil scrobble dari Last.fm sejak timestamp tertentu
// (dipakai untuk backfill manual, misal via flag --from di CLI).
// Kirim time.Time{} (zero value) untuk full backfill dari awal.
func (s *ScrobbleService) SyncFrom(ctx context.Context, from time.Time) (*SyncResult, error) {
	return s.syncFrom(ctx, from)
}

func (s *ScrobbleService) syncFrom(ctx context.Context, from time.Time) (*SyncResult, error) {
	result := &SyncResult{}
	page := 1

	for {
		resp, err := s.client.GetRecentTracks(ctx, from, page)
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
		if page >= totalPages {
			break
		}
		page++
	}

	return result, nil
}

// mapTracksToScrobbles mengonversi raw Track dari response Last.fm API
// menjadi entity model.Scrobble. Track yang berstatus "now playing"
// otomatis di-skip karena belum punya PlayedAt yang final.
func mapTracksToScrobbles(tracks lastfm.TrackList) []*model.Scrobble {
	scrobbles := make([]*model.Scrobble, 0, len(tracks))

	for _, t := range tracks {
		if t.IsNowPlaying() {
			continue
		}

		playedAt, err := t.PlayedAt()
		if err != nil {
			// Track tanpa timestamp valid (bukan now-playing tapi tetap
			// gagal parse) di-skip, bukan bikin seluruh sync gagal.
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

// insertScrobbles menyimpan banyak scrobble sekaligus dalam satu transaction.
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

// insertScrobble menyisipkan satu scrobble. Memakai INSERT OR IGNORE
// sehingga aman dipanggil berulang -> duplikat otomatis di-skip oleh
// unique index idx_scrobble_logs_unique_event di level database.
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

// getLastPlayedAt mengembalikan timestamp scrobble terbaru di database,
// dipakai sebagai titik awal incremental sync. Time.Time{} (zero value)
// berarti tabel masih kosong -> sync akan mengambil dari awal.
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
