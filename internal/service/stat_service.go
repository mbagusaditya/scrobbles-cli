package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const fallbackTrackDurationMs int64 = 210_000 // 3.5 menit (210 detik) jika durasi aktual tidak tersedia

type StatsFilter struct {
	From time.Time
	To   time.Time
}

type StatsResult struct {
	// Ringkasan Utama
	TotalScrobbles int
	TotalArtists   int
	TotalAlbums    int
	TotalTracks    int

	// Durasi Mendengarkan
	TotalDurationMs     int64   // Akumulasi total durasi (aktual + fallback) dalam milidetik
	ActualDurationCount int     // Jumlah scrobble yang memiliki durasi aktual (> 0 ms)
	ActualDurationRatio float64 // Persentase scrobble dengan durasi aktual (0.0 - 100.0)

	// Rentang & Metrik Tambahan
	FirstScrobble time.Time
	LastScrobble  time.Time
	ActiveDays    int
	TotalDays     int
	AvgPerDay     float64
	PeakDayDate   string
	PeakDayHits   int
	PeakHour      int
	RepeatRatio   float64
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

// loadTrackDurationMap memuat durasi aktual (dalam ms) dari tabel tracks + artists.
// Kunci dinormalisasi dengan AliasService agar variasi judul ikut mendapat durasi kanonik.
func (s *ScrobbleService) loadTrackDurationMap(ctx context.Context) map[string]int64 {
	durationMap := make(map[string]int64)

	const query = `
		SELECT a.name, t.title, MAX(t.duration_ms)
		FROM tracks t
		JOIN artists a ON t.artist_id = a.id
		WHERE t.duration_ms IS NOT NULL AND t.duration_ms > 0
		GROUP BY LOWER(TRIM(a.name)), LOWER(TRIM(t.title))
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		// Jika gagal membaca durasi (misal tabel kosong), kembalikan map kosong agar fallback berjalan aman
		return durationMap
	}
	defer rows.Close()

	for rows.Next() {
		var artist, title string
		var durationMs int64
		if err := rows.Scan(&artist, &title, &durationMs); err == nil && durationMs > 0 {
			canonicalTitle := s.canonicalTrackTitle(artist, title)
			key := normalizeKey(artist) + "\x00" + normalizeKey(canonicalTitle)
			if existing, ok := durationMap[key]; !ok || durationMs > existing {
				durationMap[key] = durationMs
			}
		}
	}

	return durationMap
}

func (s *ScrobbleService) GetStats(ctx context.Context, filter StatsFilter) (*StatsResult, error) {
	// Muat peta durasi aktual (1 kueri ringan ke tabel tracks)
	durationMap := s.loadTrackDurationMap(ctx)

	var conditions []string
	var args []any

	if !filter.From.IsZero() {
		conditions = append(conditions, "played_at >= ?")
		args = append(args, filter.From.Unix())
	}
	if !filter.To.IsZero() {
		conditions = append(conditions, "played_at <= ?")
		args = append(args, filter.To.Unix())
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT
			raw_artist,
			COALESCE(raw_album, ''),
			raw_title,
			played_at
		FROM scrobble_logs
		%s
		ORDER BY played_at ASC
	`, whereClause)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("gagal query scrobble logs untuk statistik: %w", err)
	}
	defer rows.Close()

	artistSet := make(map[string]struct{})
	albumSet := make(map[string]struct{})
	trackSet := make(map[string]struct{})
	dayCounts := make(map[string]int)
	hourCounts := make(map[int]int)

	var totalScrobbles int
	var totalDurationMs int64
	var actualDurationCount int
	var firstPlayed, lastPlayed time.Time

	for rows.Next() {
		var artist, album, title string
		var epoch int64

		if err := rows.Scan(&artist, &album, &title, &epoch); err != nil {
			return nil, fmt.Errorf("gagal membaca baris statistik: %w", err)
		}

		t := time.Unix(epoch, 0).Local()
		if totalScrobbles == 0 {
			firstPlayed = t
		}
		lastPlayed = t
		totalScrobbles++

		canonicalTitle := s.canonicalTrackTitle(artist, title)

		normArtist := normalizeKey(artist)
		normAlbum := normalizeKey(album)
		normTitle := normalizeKey(canonicalTitle)

		trackKey := normArtist + "\x00" + normTitle

		// Akumulasi durasi aktual jika tersedia, fallback 3.5 menit jika belum tersedia
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
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterasi baris statistik: %w", err)
	}

	if totalScrobbles == 0 {
		return &StatsResult{}, nil
	}

	activeDays := len(dayCounts)
	rangeStart := firstPlayed
	rangeEnd := lastPlayed

	if !filter.From.IsZero() {
		rangeStart = filter.From
	}
	if !filter.To.IsZero() {
		rangeEnd = filter.To
	}

	totalDays := calcInclusiveCalendarDays(rangeStart, rangeEnd)
	if activeDays > totalDays {
		totalDays = activeDays
	}

	var peakDay string
	var peakDayHits int
	for day, count := range dayCounts {
		if count > peakDayHits {
			peakDayHits = count
			peakDay = day
		}
	}

	var peakHour int
	var peakHourHits int
	for hour, count := range hourCounts {
		if count > peakHourHits {
			peakHourHits = count
			peakHour = hour
		}
	}

	repeatRatio := 0.0
	if len(trackSet) > 0 {
		repeatRatio = float64(totalScrobbles) / float64(len(trackSet))
	}

	actualRatio := (float64(actualDurationCount) / float64(totalScrobbles)) * 100.0

	return &StatsResult{
		TotalScrobbles:      totalScrobbles,
		TotalArtists:        len(artistSet),
		TotalAlbums:         len(albumSet),
		TotalTracks:         len(trackSet),
		TotalDurationMs:     totalDurationMs,
		ActualDurationCount: actualDurationCount,
		ActualDurationRatio: actualRatio,
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
