package service

import (
	"context"
	"strings"
	"time"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
	"github.com/mbagusaditya/scrobbles-cli/internal/repository"
)

const fallbackTrackDurationMs int64 = 210_000 // 3.5 menit jika durasi aktual tidak tersedia

type StatsService struct {
	statsRepo repository.StatsRepository
	trackRepo repository.TrackRepository
	alias     *AliasService
}

func NewStatsService(statsRepo repository.StatsRepository, trackRepo repository.TrackRepository, alias *AliasService) *StatsService {
	return &StatsService{
		statsRepo: statsRepo,
		trackRepo: trackRepo,
		alias:     alias,
	}
}

func (s *StatsService) GetStats(ctx context.Context, filter model.StatsFilter) (*model.StatsResult, error) {
	durationMap := s.loadDurationMap(ctx)

	artistSet := make(map[string]struct{})
	albumSet := make(map[string]struct{})
	trackSet := make(map[string]struct{})
	dayCounts := make(map[string]int)
	hourCounts := make(map[int]int)

	var totalScrobbles, actualDurationCount int
	var totalDurationMs int64
	var firstPlayed, lastPlayed time.Time

	err := s.statsRepo.StreamScrobbleRows(ctx, filter.From, filter.To, func(row repository.ScrobbleStatRow) error {
		t := time.Unix(row.PlayedAt, 0).Local()
		if totalScrobbles == 0 {
			firstPlayed = t
		}
		lastPlayed = t
		totalScrobbles++

		canonicalTitle := s.alias.ResolveTrackTitle(row.RawArtist, row.RawTitle)
		normArtist := normalizeKey(row.RawArtist)
		normAlbum := normalizeKey(row.RawAlbum)
		normTitle := normalizeKey(canonicalTitle)
		trackKey := normArtist + "\x00" + normTitle

		if durMs, ok := durationMap[trackKey]; ok && durMs > 0 {
			totalDurationMs += durMs
			actualDurationCount++
		} else {
			totalDurationMs += fallbackTrackDurationMs
		}

		if normArtist != "" {
			artistSet[normArtist] = struct{}{}
		}
		if normAlbum != "" {
			albumSet[normArtist+"\x00"+normAlbum] = struct{}{}
		}
		if normTitle != "" {
			trackSet[trackKey] = struct{}{}
		}

		dayCounts[t.Format("2006-01-02")]++
		hourCounts[t.Hour()]++
		return nil
	})
	if err != nil {
		return nil, err
	}

	if totalScrobbles == 0 {
		return &model.StatsResult{}, nil
	}

	activeDays := len(dayCounts)
	rangeStart, rangeEnd := firstPlayed, lastPlayed
	if !filter.From.IsZero() {
		rangeStart = filter.From
	}
	if !filter.To.IsZero() {
		rangeEnd = filter.To
	}

	totalDays := max(calcInclusiveCalendarDays(rangeStart, rangeEnd), activeDays)

	peakDay, peakDayHits := findPeak(dayCounts)
	peakHour, _ := findPeak(hourCounts)

	var repeatRatio float64
	if len(trackSet) > 0 {
		repeatRatio = float64(totalScrobbles) / float64(len(trackSet))
	}

	return &model.StatsResult{
		TotalScrobbles:      totalScrobbles,
		TotalArtists:        len(artistSet),
		TotalAlbums:         len(albumSet),
		TotalTracks:         len(trackSet),
		TotalDurationMs:     totalDurationMs,
		ActualDurationCount: actualDurationCount,
		ActualDurationRatio: (float64(actualDurationCount) / float64(totalScrobbles)) * 100.0,
		FirstScrobble:       firstPlayed,
		LastScrobble:        lastPlayed,
		ActiveDays:          activeDays,
		TotalDays:           totalDays,
		AvgPerDay:           float64(totalScrobbles) / float64(activeDays),
		PeakDayDate:         peakDay,
		PeakDayHits:         peakDayHits,
		PeakHour:            peakHour,
		RepeatRatio:         repeatRatio,
	}, nil
}

func (s *StatsService) loadDurationMap(ctx context.Context) map[string]int64 {
	result := make(map[string]int64)
	if s.trackRepo == nil {
		return result
	}

	known, err := s.trackRepo.FetchKnownDurations(ctx)
	if err != nil {
		return result
	}

	for rawKey, durMs := range known {
		if artist, title, ok := strings.Cut(rawKey, "\x00"); ok {
			key := aliasKey(artist, s.alias.ResolveTrackTitle(artist, title))
			if durMs > result[key] {
				result[key] = durMs
			}
		}
	}
	return result
}

func findPeak[K comparable](counts map[K]int) (K, int) {
	var bestKey K
	var bestVal int
	for k, v := range counts {
		if v > bestVal {
			bestKey, bestVal = k, v
		}
	}
	return bestKey, bestVal
}

func normalizeKey(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func calcInclusiveCalendarDays(start, end time.Time) int {
	s := time.Date(start.Year(), start.Month(), start.Day(), 12, 0, 0, 0, start.Location())
	e := time.Date(end.Year(), end.Month(), end.Day(), 12, 0, 0, 0, end.Location())
	if e.Before(s) {
		return 1
	}
	return int(e.Sub(s).Hours()/24) + 1
}
