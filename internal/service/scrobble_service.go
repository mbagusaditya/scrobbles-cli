package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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
	alias  *AliasService
}

func NewScrobbleService(database *sql.DB, client *lastfm.Client, alias *AliasService) *ScrobbleService {
	return &ScrobbleService{
		db:     database,
		client: client,
		alias:  alias,
	}
}

// canonicalTrackTitle mengembalikan judul kanonik dari AliasService jika tersedia.
func (s *ScrobbleService) canonicalTrackTitle(artist, title string) string {
	if s.alias != nil {
		return s.alias.ResolveTrackTitle(artist, title)
	}
	return strings.TrimSpace(title)
}

const MaxCustomRangeDays = 7

type SyncResult struct {
	Inserted int
	Skipped  int
	Fetched  int
}

type PageProgress struct {
	Page       int
	TotalPages int
	Fetched    int
	Inserted   int
	Skipped    int
}

type ProgressFunc func(p PageProgress)

func (s *ScrobbleService) SyncLatest(ctx context.Context, onProgress ProgressFunc) (*SyncResult, error) {
	lastPlayedAt, err := s.getLastPlayedAt(ctx)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil timestamp scrobble terakhir: %w", err)
	}

	return s.FetchRange(ctx, lastPlayedAt, time.Now(), onProgress)
}

func (s *ScrobbleService) FetchRange(ctx context.Context, from, to time.Time, onProgress ProgressFunc) (*SyncResult, error) {
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

		select {
		case <-time.After(500 * time.Millisecond):
		case <-ctx.Done():
			return result, ctx.Err()
		}

		scrobbles := s.mapTracksToScrobbles(resp.RecentTracks.Tracks)
		result.Fetched += len(scrobbles)

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

func (s *ScrobbleService) mapTracksToScrobbles(tracks lastfm.TrackList) []*model.Scrobble {
	scrobbles := make([]*model.Scrobble, 0, len(tracks))

	for _, t := range tracks {
		if t.IsNowPlaying() {
			continue
		}

		playedAt, err := t.PlayedAt()
		if err != nil {
			continue
		}

		canonicalTitle := s.canonicalTrackTitle(t.Artist.Text, t.Name)

		scrobbles = append(scrobbles, model.NewScrobble(
			canonicalTitle,
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

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("gagal query list scrobbles: %w", err)
	}
	defer rows.Close()

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
