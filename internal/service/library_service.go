package service

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
)

// normalizePagination memastikan page minimal 1 dan limit default 50.
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

// calculateTotalPages menghitung jumlah total halaman berdasarkan total item dan limit.
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

// ListLibraryArtists mengambil daftar artis unik diurutkan A-Z beserta total scrobble-nya.
func (s *ScrobbleService) ListLibraryArtists(ctx context.Context, filter model.LibraryFilter) (*model.LibraryResult, error) {
	page, limit, offset := normalizePagination(filter)

	// 1. Hitung total artis unik (selaras dengan normalizeKey di stat_service.go)
	countQuery := `
		SELECT COUNT(DISTINCT LOWER(TRIM(raw_artist)))
		FROM scrobble_logs
		WHERE TRIM(raw_artist) != ''
	`
	var totalItems int
	if err := s.db.QueryRowContext(ctx, countQuery).Scan(&totalItems); err != nil {
		return nil, fmt.Errorf("gagal menghitung total artists di pustaka: %w", err)
	}

	if totalItems == 0 {
		return &model.LibraryResult{
			Items:      []model.LibraryItem{},
			TotalItems: 0,
			Page:       page,
			Limit:      limit,
			TotalPages: 1,
		}, nil
	}

	// 2. Ambil data dengan sorting A-Z case-insensitive
	query := `
		SELECT MIN(TRIM(raw_artist)), COUNT(*) as play_count
		FROM scrobble_logs
		WHERE TRIM(raw_artist) != ''
		GROUP BY LOWER(TRIM(raw_artist))
		ORDER BY MIN(TRIM(raw_artist)) COLLATE NOCASE ASC
		LIMIT ? OFFSET ?
	`

	rows, err := s.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("gagal query list library artists: %w", err)
	}
	defer rows.Close()

	var items []model.LibraryItem
	for rows.Next() {
		var item model.LibraryItem
		if err := rows.Scan(&item.Name, &item.TotalPlays); err != nil {
			return nil, fmt.Errorf("gagal scan baris library artist: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterasi library artists: %w", err)
	}

	return &model.LibraryResult{
		Items:      items,
		TotalItems: totalItems,
		Page:       page,
		Limit:      limit,
		TotalPages: calculateTotalPages(totalItems, limit),
	}, nil
}

// ListLibraryAlbums mengambil daftar album unik (mengabaikan nilai kosong) diurutkan A-Z.
func (s *ScrobbleService) ListLibraryAlbums(ctx context.Context, filter model.LibraryFilter) (*model.LibraryResult, error) {
	page, limit, offset := normalizePagination(filter)

	// 1. Hitung total kombinasi album dan artis unik yang valid
	countQuery := `
		SELECT COUNT(*) FROM (
			SELECT LOWER(TRIM(raw_album)), LOWER(TRIM(raw_artist))
			FROM scrobble_logs
			WHERE raw_album IS NOT NULL AND TRIM(raw_album) != ''
			GROUP BY LOWER(TRIM(raw_album)), LOWER(TRIM(raw_artist))
		)
	`
	var totalItems int
	if err := s.db.QueryRowContext(ctx, countQuery).Scan(&totalItems); err != nil {
		return nil, fmt.Errorf("gagal menghitung total albums di pustaka: %w", err)
	}

	if totalItems == 0 {
		return &model.LibraryResult{
			Items:      []model.LibraryItem{},
			TotalItems: 0,
			Page:       page,
			Limit:      limit,
			TotalPages: 1,
		}, nil
	}

	// 2. Ambil data album diurutkan A-Z berdasarkan judul album
	query := `
		SELECT MIN(TRIM(raw_album)), MIN(TRIM(raw_artist)), COUNT(*) as play_count
		FROM scrobble_logs
		WHERE raw_album IS NOT NULL AND TRIM(raw_album) != ''
		GROUP BY LOWER(TRIM(raw_album)), LOWER(TRIM(raw_artist))
		ORDER BY MIN(TRIM(raw_album)) COLLATE NOCASE ASC, MIN(TRIM(raw_artist)) COLLATE NOCASE ASC
		LIMIT ? OFFSET ?
	`

	rows, err := s.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("gagal query list library albums: %w", err)
	}
	defer rows.Close()

	var items []model.LibraryItem
	for rows.Next() {
		var item model.LibraryItem
		var album, artist sql.NullString
		var playCount int

		if err := rows.Scan(&album, &artist, &playCount); err != nil {
			return nil, fmt.Errorf("gagal scan baris library album: %w", err)
		}

		item.Name = album.String
		item.Detail = artist.String
		item.TotalPlays = playCount
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterasi library albums: %w", err)
	}

	return &model.LibraryResult{
		Items:      items,
		TotalItems: totalItems,
		Page:       page,
		Limit:      limit,
		TotalPages: calculateTotalPages(totalItems, limit),
	}, nil
}

// ListLibraryTracks mengambil daftar trek unik beserta artisnya diurutkan A-Z.
// Ganti fungsi ListLibraryTracks di internal/service/library_service.go dengan versi ini:
func (s *ScrobbleService) ListLibraryTracks(ctx context.Context, filter model.LibraryFilter) (*model.LibraryResult, error) {
	page, limit, offset := normalizePagination(filter)

	query := `
		SELECT MIN(TRIM(raw_title)), MIN(TRIM(raw_artist)), COUNT(*) as play_count
		FROM scrobble_logs
		WHERE TRIM(raw_title) != ''
		GROUP BY LOWER(TRIM(raw_title)), LOWER(TRIM(raw_artist))
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("gagal query list library tracks: %w", err)
	}
	defer rows.Close()

	type trackAgg struct {
		title  string
		artist string
		plays  int
	}

	mergedMap := make(map[string]*trackAgg)

	for rows.Next() {
		var rawTitle, rawArtist string
		var playCount int
		if err := rows.Scan(&rawTitle, &rawArtist, &playCount); err != nil {
			return nil, fmt.Errorf("gagal scan baris library track: %w", err)
		}

		canonicalTitle := s.canonicalTrackTitle(rawArtist, rawTitle)
		key := normalizeKey(rawArtist) + "\x00" + normalizeKey(canonicalTitle)

		if existing, ok := mergedMap[key]; ok {
			existing.plays += playCount
			// Pertahankan casing dari judul kanonik
			existing.title = canonicalTitle
		} else {
			mergedMap[key] = &trackAgg{
				title:  canonicalTitle,
				artist: rawArtist,
				plays:  playCount,
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterasi library tracks: %w", err)
	}

	totalItems := len(mergedMap)
	if totalItems == 0 {
		return &model.LibraryResult{
			Items:      []model.LibraryItem{},
			TotalItems: 0,
			Page:       page,
			Limit:      limit,
			TotalPages: 1,
		}, nil
	}

	allItems := make([]model.LibraryItem, 0, totalItems)
	for _, agg := range mergedMap {
		allItems = append(allItems, model.LibraryItem{
			Name:       agg.title,
			Detail:     agg.artist,
			TotalPlays: agg.plays,
		})
	}

	// Sort alfabetis A-Z case-insensitive (selaras dengan ORDER BY COLLATE NOCASE ASC)
	sort.Slice(allItems, func(i, j int) bool {
		ti := strings.ToLower(allItems[i].Name)
		tj := strings.ToLower(allItems[j].Name)
		if ti != tj {
			return ti < tj
		}
		return strings.ToLower(allItems[i].Detail) < strings.ToLower(allItems[j].Detail)
	})

	// Terapkan pagination slice
	if offset >= totalItems {
		return &model.LibraryResult{
			Items:      []model.LibraryItem{},
			TotalItems: totalItems,
			Page:       page,
			Limit:      limit,
			TotalPages: calculateTotalPages(totalItems, limit),
		}, nil
	}

	end := offset + limit
	if end > totalItems {
		end = totalItems
	}

	return &model.LibraryResult{
		Items:      allItems[offset:end],
		TotalItems: totalItems,
		Page:       page,
		Limit:      limit,
		TotalPages: calculateTotalPages(totalItems, limit),
	}, nil
}
