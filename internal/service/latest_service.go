package service

import (
	"context"
	"sort"
	"strings"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
	"github.com/mbagusaditya/scrobbles-cli/internal/repository"
)

const defaultLatestLimit = 10

// LatestService menangani pengambilan daftar entitas yang terakhir kali diputar.
type LatestService struct {
	repo  repository.LatestRepository
	alias *AliasService
}

func NewLatestService(repo repository.LatestRepository, alias *AliasService) *LatestService {
	return &LatestService{
		repo:  repo,
		alias: alias,
	}
}

func normalizeLatestLimit(limit int) int {
	if limit <= 0 {
		return defaultLatestLimit
	}
	return limit
}

// GetLatestArtists mengambil n artis unik terakhir yang diputar.
func (s *LatestService) GetLatestArtists(ctx context.Context, limit int) ([]model.LatestItem, error) {
	return s.repo.GetLatestArtists(ctx, normalizeLatestLimit(limit))
}

// GetLatestAlbums mengambil n album unik terakhir yang diputar.
func (s *LatestService) GetLatestAlbums(ctx context.Context, limit int) ([]model.LatestItem, error) {
	return s.repo.GetLatestAlbums(ctx, normalizeLatestLimit(limit))
}

// GetLatestTracks mengambil n trek unik terakhir yang diputar, menggabungkan
// varian alias dan mempertahankan waktu putar paling baru (latest PlayedAt).
func (s *LatestService) GetLatestTracks(ctx context.Context, limit int) ([]model.LatestItem, error) {
	limit = normalizeLatestLimit(limit)

	rawItems, err := s.repo.GetAllTrackLastPlayed(ctx)
	if err != nil {
		return nil, err
	}

	merged := make(map[string]*model.LatestItem, len(rawItems))
	for _, item := range rawItems {
		canonicalTitle := s.resolveTitle(item.Detail, item.Name)
		key := aliasKey(item.Detail, canonicalTitle)

		if existing, ok := merged[key]; ok {
			if item.PlayedAt.After(existing.PlayedAt) {
				existing.PlayedAt = item.PlayedAt
			}
			existing.Name = canonicalTitle
		} else {
			merged[key] = &model.LatestItem{
				Name:     canonicalTitle,
				Detail:   item.Detail,
				PlayedAt: item.PlayedAt,
			}
		}
	}

	results := make([]model.LatestItem, 0, len(merged))
	for _, item := range merged {
		results = append(results, *item)
	}

	sort.Slice(results, func(i, j int) bool {
		if !results[i].PlayedAt.Equal(results[j].PlayedAt) {
			return results[i].PlayedAt.After(results[j].PlayedAt)
		}
		return strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name)
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

func (s *LatestService) resolveTitle(artist, title string) string {
	if s.alias != nil {
		return s.alias.ResolveTrackTitle(artist, title)
	}
	return strings.TrimSpace(title)
}
