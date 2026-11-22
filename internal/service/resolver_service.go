package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/mbagusaditya/scrobbles-cli/internal/lastfm"
	"github.com/mbagusaditya/scrobbles-cli/internal/model"
	"github.com/mbagusaditya/scrobbles-cli/internal/repository"
)

const (
	defaultResolveBatchSize  = 500
	durationAPIRateLimitWait = 300 * time.Millisecond
)

type ResolveProgressFunc func(p model.ResolveProgress)
type DurationProgressFunc func(p model.DurationProgress)

// ResolverService mengorkestrasi penautan scrobble mentah ke tabel master
// serta pengayaan metadata durasi lagu dari Last.fm.
type ResolverService struct {
	resolverRepo repository.ResolverRepository
	trackRepo    repository.TrackRepository
	client       *lastfm.Client
	alias        *AliasService
}

func NewResolverService(
	resolverRepo repository.ResolverRepository,
	trackRepo repository.TrackRepository,
	client *lastfm.Client,
	alias *AliasService,
) *ResolverService {
	return &ResolverService{
		resolverRepo: resolverRepo,
		trackRepo:    trackRepo,
		client:       client,
		alias:        alias,
	}
}

// ResolveUnlinkedScrobbles memproses scrobble yang track_id-nya masih NULL per batch transaksi.
func (s *ResolverService) ResolveUnlinkedScrobbles(ctx context.Context, batchSize int, onProgress ResolveProgressFunc) (*model.ResolveResult, error) {
	if batchSize <= 0 {
		batchSize = defaultResolveBatchSize
	}

	result := &model.ResolveResult{}

	for {
		remainingCount, err := s.resolverRepo.CountUnlinked(ctx)
		if err != nil {
			return result, err
		}
		if remainingCount == 0 {
			break
		}

		items, err := s.resolverRepo.FetchUnlinkedBatch(ctx, batchSize)
		if err != nil {
			return result, err
		}
		if len(items) == 0 {
			break
		}

		batchRes, err := s.resolverRepo.LinkUnresolvedBatch(ctx, items)
		if err != nil {
			return result, err
		}

		result.TotalNewArtists += batchRes.NewArtists
		result.TotalNewTracks += batchRes.NewTracks
		result.TotalUpdatedLogs += batchRes.UpdatedLogs

		remainingAfter := remainingCount - batchRes.UpdatedLogs
		if remainingAfter < 0 {
			remainingAfter = 0
		}

		if onProgress != nil {
			onProgress(model.ResolveProgress{
				BatchProcessed: len(items),
				Remaining:      remainingAfter,
				NewArtists:     batchRes.NewArtists,
				NewTracks:      batchRes.NewTracks,
				UpdatedLogs:    batchRes.UpdatedLogs,
			})
		}
	}

	return result, nil
}

// SyncTrackDurations mengambil durasi lagu aktual dari Last.fm (track.getInfo)
// untuk semua track yang kolom duration_ms-nya masih kosong.
func (s *ResolverService) SyncTrackDurations(ctx context.Context, onProgress DurationProgressFunc) (*model.DurationSyncResult, error) {
	targets, err := s.trackRepo.FetchMissingDurations(ctx)
	if err != nil {
		return nil, err
	}

	res := &model.DurationSyncResult{}
	total := len(targets)
	if total == 0 {
		return res, nil
	}

	durationCache := s.buildInitialDurationCache(ctx)

	for i, item := range targets {
		select {
		case <-ctx.Done():
			return res, ctx.Err()
		default:
		}

		res.Processed++
		canonicalTitle := s.resolveCanonicalTitle(item.Artist, item.Title)
		cacheKey := aliasKey(item.Artist, canonicalTitle)

		durationMs, cached := durationCache[cacheKey]
		if !cached {
			var fetchErr error
			durationMs, fetchErr = s.fetchDurationFromAPI(ctx, item.Artist, canonicalTitle)
			if fetchErr != nil {
				return res, fetchErr
			}
			durationCache[cacheKey] = durationMs
		}

		found := durationMs > 0
		if found {
			if err := s.trackRepo.UpdateDuration(ctx, item.Artist, item.Title, durationMs); err != nil {
				return res, err
			}
			res.Updated++
		} else {
			res.NotFound++
		}

		if onProgress != nil {
			onProgress(model.DurationProgress{
				Current:    i + 1,
				Total:      total,
				Artist:     item.Artist,
				Title:      item.Title,
				DurationMs: durationMs,
				Found:      found,
			})
		}
	}

	return res, nil
}

func (s *ResolverService) buildInitialDurationCache(ctx context.Context) map[string]int64 {
	cache := make(map[string]int64)
	known, err := s.trackRepo.FetchKnownDurations(ctx)
	if err != nil {
		return cache
	}

	for rawKey, durMs := range known {
		parts := strings.SplitN(rawKey, "\x00", 2)
		if len(parts) == 2 {
			canonicalTitle := s.resolveCanonicalTitle(parts[0], parts[1])
			cache[aliasKey(parts[0], canonicalTitle)] = durMs
		}
	}
	return cache
}

func (s *ResolverService) fetchDurationFromAPI(ctx context.Context, artist, canonicalTitle string) (int64, error) {
	var durationMs int64

	info, err := s.client.GetTrackInfo(ctx, artist, canonicalTitle)
	if err == nil && info != nil && info.Track.Duration != "" {
		if parsed, parseErr := strconv.ParseInt(strings.TrimSpace(info.Track.Duration), 10, 64); parseErr == nil && parsed > 0 {
			durationMs = parsed
		}
	}

	select {
	case <-time.After(durationAPIRateLimitWait):
		return durationMs, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func (s *ResolverService) resolveCanonicalTitle(artist, title string) string {
	if s.alias != nil {
		return s.alias.ResolveTrackTitle(artist, title)
	}
	return strings.TrimSpace(title)
}
