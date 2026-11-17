package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

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

// normalizeKey menormalisasi string (trim spasi & lowercase) agar konsisten
// antara kalkulasi in-memory Go dan grouping SQLite.
func normalizeKey(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// calcInclusiveCalendarDays menghitung jumlah hari kalender secara inklusif
// berdasarkan titik tengah hari (12:00:00) agar kebal terhadap selisih jam/menit.
func calcInclusiveCalendarDays(start, end time.Time) int {
	s := time.Date(start.Year(), start.Month(), start.Day(), 12, 0, 0, 0, start.Location())
	e := time.Date(end.Year(), end.Month(), end.Day(), 12, 0, 0, 0, end.Location())
	if e.Before(s) {
		return 1
	}
	return int(e.Sub(s).Hours()/24) + 1
}

func (s *ScrobbleService) GetStats(ctx context.Context, filter StatsFilter) (*StatsResult, error) {
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

	// 1 kali scan linier murni: Turso hanya membaca baris data persis sebanyak jumlah log
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

	// In-memory set & counter di Go (zero cost reads ke Turso)
	artistSet := make(map[string]struct{})
	albumSet := make(map[string]struct{})
	trackSet := make(map[string]struct{})
	dayCounts := make(map[string]int) // "YYYY-MM-DD" -> count
	hourCounts := make(map[int]int)   // 0-23 -> count

	var totalScrobbles int
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

		normArtist := normalizeKey(artist)
		normAlbum := normalizeKey(album)
		normTitle := normalizeKey(title)

		// Unique Artists (selaras dengan ListLibraryArtists)
		if normArtist != "" {
			artistSet[normArtist] = struct{}{}
		}

		// Unique Albums (selaras dengan ListLibraryAlbums: kombinasi album + artist)
		if normAlbum != "" {
			albumSet[normArtist+"\x00"+normAlbum] = struct{}{}
		}

		// Unique Tracks (selaras dengan ListLibraryTracks: kombinasi title + artist)
		if normTitle != "" {
			trackSet[normArtist+"\x00"+normTitle] = struct{}{}
		}

		// Distribusi waktu
		dayCounts[t.Format("2006-01-02")]++
		hourCounts[t.Hour()]++
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterasi baris statistik: %w", err)
	}

	if totalScrobbles == 0 {
		return &StatsResult{}, nil
	}

	// Hitung TotalDays berdasarkan batas kalender inklusif
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

	return &StatsResult{
		TotalScrobbles: totalScrobbles,
		TotalArtists:   len(artistSet),
		TotalAlbums:    len(albumSet),
		TotalTracks:    len(trackSet),
		FirstScrobble:  firstPlayed,
		LastScrobble:   lastPlayed,
		ActiveDays:     activeDays,
		TotalDays:      totalDays,
		AvgPerDay:      float64(totalScrobbles) / float64(activeDays),
		PeakDayDate:    peakDay,
		PeakDayHits:    peakDayHits,
		PeakHour:       peakHour,
		RepeatRatio:    repeatRatio,
	}, nil
}
