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
			COALESCE(track_id, ''),
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
		var artist, album, title, trackID string
		var epoch int64

		if err := rows.Scan(&artist, &album, &title, &trackID, &epoch); err != nil {
			return nil, fmt.Errorf("gagal membaca baris statistik: %w", err)
		}

		t := time.Unix(epoch, 0).Local()
		if totalScrobbles == 0 {
			firstPlayed = t
		}
		lastPlayed = t
		totalScrobbles++

		// Unique Artists
		artistSet[artist] = struct{}{}

		// Unique Albums
		if album != "" {
			albumSet[album] = struct{}{}
		}

		// Unique Tracks (gunakan track_id jika ada, fallback ke string komposit)
		if trackID != "" {
			trackSet[trackID] = struct{}{}
		} else {
			trackSet[artist+"\x00"+title] = struct{}{}
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

	// Hitung metrik turunan di Go
	activeDays := len(dayCounts)
	totalDays := int(lastPlayed.Sub(firstPlayed).Hours()/24) + 1
	if totalDays <= 0 {
		totalDays = 1
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
