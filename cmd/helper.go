package cmd

import (
	"fmt"
	"os"
	"time"
)

// parseDateRange mem-parsing pasangan string tanggal YYYY-MM-DD menjadi batas waktu lokal
// (00:00:00 untuk from, 23:59:59 untuk to), memvalidasi urutannya, dan mengembalikan label periode.
func parseDateRange(fromStr, toStr string) (fromTime, toTime time.Time, periodLabel string, err error) {
	periodLabel = "All Time"

	if fromStr != "" {
		t, parseErr := time.ParseInLocation("2006-01-02", fromStr, time.Local)
		if parseErr != nil {
			return time.Time{}, time.Time{}, "", fmt.Errorf("format flag --from tidak valid (gunakan YYYY-MM-DD): %w", parseErr)
		}
		fromTime = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
	}

	if toStr != "" {
		t, parseErr := time.ParseInLocation("2006-01-02", toStr, time.Local)
		if parseErr != nil {
			return time.Time{}, time.Time{}, "", fmt.Errorf("format flag --to tidak valid (gunakan YYYY-MM-DD): %w", parseErr)
		}
		toTime = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.Local)
	}

	if !fromTime.IsZero() && !toTime.IsZero() {
		if fromTime.After(toTime) {
			return time.Time{}, time.Time{}, "", fmt.Errorf("rentang tanggal tidak valid: --from tidak boleh lebih baru dari --to")
		}
		periodLabel = fmt.Sprintf("%s s.d. %s", fromTime.Format("2006-01-02"), toTime.Format("2006-01-02"))
	} else if !fromTime.IsZero() {
		periodLabel = fmt.Sprintf("Sejak %s", fromTime.Format("2006-01-02"))
	} else if !toTime.IsZero() {
		periodLabel = fmt.Sprintf("Hingga %s", toTime.Format("2006-01-02"))
	}

	return fromTime, toTime, periodLabel, nil
}

// parsePeriodRange memvalidasi preset periode (--day, --week, --month, --year) beserta
// custom range (--from, --to), lalu mengembalikan batas waktu [fromTime, toTime] dan labelnya.
func parsePeriodRange(fromStr, toStr string, day, week, month, year bool) (fromTime, toTime time.Time, periodLabel string, err error) {
	presetCount := 0
	for _, active := range []bool{day, week, month, year} {
		if active {
			presetCount++
		}
	}

	if presetCount > 1 {
		return time.Time{}, time.Time{}, "", fmt.Errorf("pilih salah satu preset saja: --day, --week, --month, atau --year")
	}
	if presetCount == 1 && (fromStr != "" || toStr != "") {
		return time.Time{}, time.Time{}, "", fmt.Errorf("flag preset (--day/--week/--month/--year) tidak dapat digabung dengan --from atau --to")
	}

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	todayEnd := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.Local)

	switch {
	case day:
		return todayStart, todayEnd, fmt.Sprintf("Hari Ini (%s)", now.Format("2006-01-02")), nil
	case week:
		from := todayStart.AddDate(0, 0, -6)
		return from, todayEnd, fmt.Sprintf("7 Hari Terakhir (%s s.d. %s)", from.Format("2006-01-02"), todayEnd.Format("2006-01-02")), nil
	case month:
		from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
		return from, todayEnd, fmt.Sprintf("Bulan Ini (%s s.d. %s)", from.Format("2006-01-02"), todayEnd.Format("2006-01-02")), nil
	case year:
		from := time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, time.Local)
		return from, todayEnd, fmt.Sprintf("Tahun Ini (%s s.d. %s)", from.Format("2006-01-02"), todayEnd.Format("2006-01-02")), nil
	default:
		return parseDateRange(fromStr, toStr)
	}
}

// normalizeLimit menormalisasi nilai limit agar berada dalam rentang aman.
func normalizeLimit(limit, defaultLimit, maxLimit int) int {
	if limit > maxLimit {
		fmt.Fprintf(os.Stderr, "⚠️  Limit melebihi batas, otomatis disesuaikan ke maksimal %d.\n", maxLimit)
		return maxLimit
	}
	if limit <= 0 {
		return defaultLimit
	}
	return limit
}

// normalizePagination menormalisasi nilai limit dan page agar berada dalam rentang aman.
func normalizePagination(limit, page, defaultLimit, maxLimit int) (int, int) {
	limit = normalizeLimit(limit, defaultLimit, maxLimit)
	if page <= 0 {
		page = 1
	}
	return limit, page
}
