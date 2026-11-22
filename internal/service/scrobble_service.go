package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mbagusaditya/scrobbles-cli/internal/lastfm"
	"github.com/mbagusaditya/scrobbles-cli/internal/model"
	"github.com/mbagusaditya/scrobbles-cli/internal/repository"
)

const (
	MaxCustomRangeDays     = 7
	syncPageRateLimitDelay = 500 * time.Millisecond
)

type ListFilter struct {
	Artist string
	Album  string
	From   time.Time
	To     time.Time
	Limit  int
	Page   int
}

type ProgressFunc func(p model.PageProgress)

// ScrobbleService mengorkestrasi pengambilan data dari Last.fm API,
// normalisasi judul lagu, penyimpanan log scrobble, serta pembacaan riwayat.
type ScrobbleService struct {
	repo   repository.ScrobbleRepository
	client *lastfm.Client
	alias  *AliasService
}

func NewScrobbleService(repo repository.ScrobbleRepository, client *lastfm.Client, alias *AliasService) *ScrobbleService {
	return &ScrobbleService{
		repo:   repo,
		client: client,
		alias:  alias,
	}
}

func (s *ScrobbleService) SyncLatest(ctx context.Context, onProgress ProgressFunc) (*model.SyncResult, error) {
	lastPlayedAt, err := s.repo.GetLastPlayedAt(ctx)
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil timestamp scrobble terakhir: %w", err)
	}

	return s.FetchRange(ctx, lastPlayedAt, time.Now(), onProgress)
}

func (s *ScrobbleService) FetchRange(ctx context.Context, from, to time.Time, onProgress ProgressFunc) (*model.SyncResult, error) {
	if from.IsZero() {
		lastPlayed, err := s.repo.GetLastPlayedAt(ctx)
		if err != nil {
			return nil, fmt.Errorf("gagal mengambil timestamp scrobble terakhir: %w", err)
		}
		from = lastPlayed
	}

	result := &model.SyncResult{}
	page := 1

	for {
		resp, err := s.client.GetRecentTracks(ctx, from, to, page)
		if err != nil {
			return result, fmt.Errorf("gagal fetch halaman %d dari Last.fm: %w", page, err)
		}

		select {
		case <-time.After(syncPageRateLimitDelay):
		case <-ctx.Done():
			return result, ctx.Err()
		}

		scrobbles := s.mapTracksToScrobbles(resp.RecentTracks.Tracks)
		result.Fetched += len(scrobbles)

		inserted, skipped, err := s.repo.InsertBatch(ctx, scrobbles)
		if err != nil {
			return result, fmt.Errorf("gagal insert batch halaman %d: %w", page, err)
		}
		result.Inserted += inserted
		result.Skipped += skipped

		totalPages := resp.RecentTracks.Attr.TotalPagesInt()
		if onProgress != nil {
			onProgress(model.PageProgress{
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

	repoFilter := model.ScrobbleListFilter{
		Artist: strings.TrimSpace(filter.Artist),
		Album:  strings.TrimSpace(filter.Album),
		From:   filter.From,
		To:     filter.To,
		Limit:  limit,
		Offset: (page - 1) * limit,
	}

	return s.repo.List(ctx, repoFilter)
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

		canonicalTitle := s.alias.ResolveTrackTitle(t.Artist.Text, t.Name)

		scrobbles = append(scrobbles, model.NewScrobble(
			canonicalTitle,
			t.Artist.Text,
			t.Album.Text,
			playedAt,
		))
	}

	return scrobbles
}
