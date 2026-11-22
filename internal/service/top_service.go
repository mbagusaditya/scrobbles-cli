package service

import (
	"context"
	"sort"
	"strings"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
	"github.com/mbagusaditya/scrobbles-cli/internal/repository"
)

const defaultTopLimit = 10

// TopService menghitung peringkat lagu, artis, dan album yang paling sering diputar.
type TopService struct {
	repo  repository.TopRepository
	alias *AliasService
}

func NewTopService(repo repository.TopRepository, alias *AliasService) *TopService {
	return &TopService{
		repo:  repo,
		alias: alias,
	}
}

func normalizeTopLimit(limit int) int {
	if limit <= 0 {
		return defaultTopLimit
	}
	return limit
}

// GetTopTracks mengambil daftar lagu teratas dengan menggabungkan hitungan varian alias.
func (s *TopService) GetTopTracks(ctx context.Context, filter model.TopFilter) ([]model.TopItem, error) {
	limit := normalizeTopLimit(filter.Limit)

	rawItems, err := s.repo.FetchTrackCounts(ctx, filter.From, filter.To)
	if err != nil {
		return nil, err
	}

	return s.mergeAndRankTracks(rawItems, limit), nil
}

// GetTopArtists mengambil daftar artis yang paling sering didengarkan berdasarkan rentang waktu.
func (s *TopService) GetTopArtists(ctx context.Context, filter model.TopFilter) ([]model.TopItem, error) {
	return s.repo.FetchTopArtists(ctx, filter.From, filter.To, normalizeTopLimit(filter.Limit))
}

// GetTopAlbums mengambil daftar album yang paling sering didengarkan berdasarkan rentang waktu.
func (s *TopService) GetTopAlbums(ctx context.Context, filter model.TopFilter) ([]model.TopItem, error) {
	return s.repo.FetchTopAlbums(ctx, filter.From, filter.To, normalizeTopLimit(filter.Limit))
}

func (s *TopService) mergeAndRankTracks(rawItems []model.TopItem, limit int) []model.TopItem {
	mergedMap := make(map[string]*model.TopItem, len(rawItems))

	for _, raw := range rawItems {
		canonicalTitle := s.alias.ResolveTrackTitle(raw.Detail, raw.Name)
		key := aliasKey(raw.Detail, canonicalTitle)

		if existing, ok := mergedMap[key]; ok {
			existing.Count += raw.Count
			existing.Name = canonicalTitle
		} else {
			mergedMap[key] = &model.TopItem{
				Name:   canonicalTitle,
				Detail: raw.Detail,
				Count:  raw.Count,
			}
		}
	}

	results := make([]model.TopItem, 0, len(mergedMap))
	for _, item := range mergedMap {
		results = append(results, *item)
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Count != results[j].Count {
			return results[i].Count > results[j].Count
		}
		return strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name)
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results
}
