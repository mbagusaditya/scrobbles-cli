package service

import (
	"context"
	"sort"
	"strings"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
	"github.com/mbagusaditya/scrobbles-cli/internal/repository"
)

// LibraryService mengelola penjelajahan master katalog unik (artists, albums, tracks).
type LibraryService struct {
	repo  repository.LibraryRepository
	alias *AliasService
}

func NewLibraryService(repo repository.LibraryRepository, alias *AliasService) *LibraryService {
	return &LibraryService{
		repo:  repo,
		alias: alias,
	}
}

// ListLibraryArtists mengambil daftar artis unik diurutkan A-Z beserta total scrobble-nya.
func (s *LibraryService) ListLibraryArtists(ctx context.Context, filter model.LibraryFilter) (*model.LibraryResult, error) {
	page, limit, offset := normalizePagination(filter)

	totalItems, err := s.repo.CountUniqueArtists(ctx)
	if err != nil {
		return nil, err
	}
	if totalItems == 0 || offset >= totalItems {
		return buildLibraryPage([]model.LibraryItem{}, totalItems, page, limit), nil
	}

	items, err := s.repo.FetchPagedArtists(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	return buildLibraryPage(items, totalItems, page, limit), nil
}

// ListLibraryAlbums mengambil daftar album unik (mengabaikan nilai kosong) diurutkan A-Z.
func (s *LibraryService) ListLibraryAlbums(ctx context.Context, filter model.LibraryFilter) (*model.LibraryResult, error) {
	page, limit, offset := normalizePagination(filter)

	totalItems, err := s.repo.CountUniqueAlbums(ctx)
	if err != nil {
		return nil, err
	}
	if totalItems == 0 || offset >= totalItems {
		return buildLibraryPage([]model.LibraryItem{}, totalItems, page, limit), nil
	}

	items, err := s.repo.FetchPagedAlbums(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	return buildLibraryPage(items, totalItems, page, limit), nil
}

// ListLibraryTracks mengambil daftar trek unik (sudah digabung berdasarkan aturan alias) diurutkan A-Z.
func (s *LibraryService) ListLibraryTracks(ctx context.Context, filter model.LibraryFilter) (*model.LibraryResult, error) {
	page, limit, offset := normalizePagination(filter)

	rawItems, err := s.repo.FetchAllTrackCounts(ctx)
	if err != nil {
		return nil, err
	}

	merged := s.mergeAndSortTracks(rawItems)
	totalItems := len(merged)

	if totalItems == 0 || offset >= totalItems {
		return buildLibraryPage([]model.LibraryItem{}, totalItems, page, limit), nil
	}

	end := min(offset+limit, totalItems)

	return buildLibraryPage(merged[offset:end], totalItems, page, limit), nil
}

func (s *LibraryService) mergeAndSortTracks(rawItems []model.LibraryItem) []model.LibraryItem {
	mergedMap := make(map[string]*model.LibraryItem, len(rawItems))

	for _, raw := range rawItems {
		canonicalTitle := raw.Name
		if s.alias != nil {
			canonicalTitle = s.alias.ResolveTrackTitle(raw.Detail, raw.Name)
		}

		key := aliasKey(raw.Detail, canonicalTitle)
		if existing, ok := mergedMap[key]; ok {
			existing.TotalPlays += raw.TotalPlays
			existing.Name = canonicalTitle
		} else {
			mergedMap[key] = &model.LibraryItem{
				Name:       canonicalTitle,
				Detail:     raw.Detail,
				TotalPlays: raw.TotalPlays,
			}
		}
	}

	allItems := make([]model.LibraryItem, 0, len(mergedMap))
	for _, item := range mergedMap {
		allItems = append(allItems, *item)
	}

	sort.Slice(allItems, func(i, j int) bool {
		ti := strings.ToLower(allItems[i].Name)
		tj := strings.ToLower(allItems[j].Name)
		if ti != tj {
			return ti < tj
		}
		return strings.ToLower(allItems[i].Detail) < strings.ToLower(allItems[j].Detail)
	})

	return allItems
}

func normalizePagination(filter model.LibraryFilter) (page int, limit int, offset int) {
	limit = filter.Limit
	if limit <= 0 {
		limit = 50
	} else if limit > 200 {
		limit = 200
	}

	page = filter.Page
	if page <= 0 {
		page = 1
	}

	offset = (page - 1) * limit
	return page, limit, offset
}

func calculateTotalPages(totalItems, limit int) int {
	if totalItems == 0 || limit <= 0 {
		return 1
	}
	totalPages := totalItems / limit
	if totalItems%limit != 0 {
		totalPages++
	}
	return totalPages
}

func buildLibraryPage(items []model.LibraryItem, totalItems, page, limit int) *model.LibraryResult {
	if items == nil {
		items = []model.LibraryItem{}
	}
	return &model.LibraryResult{
		Items:      items,
		TotalItems: totalItems,
		Page:       page,
		Limit:      limit,
		TotalPages: calculateTotalPages(totalItems, limit),
	}
}
