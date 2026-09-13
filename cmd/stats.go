package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mbagusaditya/scrobbles-cli/internal/service"
)

var (
	statsFrom  string
	statsTo    string
	statsDay   bool
	statsWeek  bool
	statsMonth bool
	statsYear  bool
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Menampilkan statistik agregasi scrobble",
	Long: `Menampilkan ringkasan total scrobbles, artis unik, album unik, dan track unik.
Mendukung rentang custom via --from dan --to, atau preset via --day, --week, --month, dan --year.
Jika tidak ada flag yang diberikan, statistik akan dihitung untuk sepanjang waktu (all-time).`,
	RunE: runStats,
}

func init() {
	rootCmd.AddCommand(statsCmd)

	statsCmd.Flags().StringVar(&statsFrom, "from", "", "Batas awal tanggal (format: YYYY-MM-DD)")
	statsCmd.Flags().StringVar(&statsTo, "to", "", "Batas akhir tanggal (format: YYYY-MM-DD)")
	statsCmd.Flags().BoolVar(&statsDay, "day", false, "Statistik hari ini")
	statsCmd.Flags().BoolVar(&statsWeek, "week", false, "Statistik 7 hari terakhir")
	statsCmd.Flags().BoolVar(&statsMonth, "month", false, "Statistik bulan berjalan (sejak tgl 1)")
	statsCmd.Flags().BoolVar(&statsYear, "year", false, "Statistik tahun berjalan (sejak 1 Jan)")
}

func runStats(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	now := time.Now()

	// 1. Validasi mutual exclusion untuk preset flag
	presetCount := 0
	if statsDay {
		presetCount++
	}
	if statsWeek {
		presetCount++
	}
	if statsMonth {
		presetCount++
	}
	if statsYear {
		presetCount++
	}

	if presetCount > 1 {
		return fmt.Errorf("pilih salah satu preset saja: --day, --week, --month, atau --year")
	}

	if presetCount == 1 && (statsFrom != "" || statsTo != "") {
		return fmt.Errorf("flag preset (--day/--week/--month/--year) tidak dapat digabung dengan --from atau --to")
	}

	// 2. Tentukan batas waktu [from, to]
	var fromTime, toTime time.Time
	periodLabel := "All Time"

	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	todayEnd := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.Local)

	switch {
	case statsDay:
		fromTime = todayStart
		toTime = todayEnd
		periodLabel = fmt.Sprintf("Hari Ini (%s)", now.Format("2006-01-02"))

	case statsWeek:
		// 7 hari terakhir bergulir (rolling 7 days) s.d. akhir hari ini
		fromTime = todayStart.AddDate(0, 0, -6)
		toTime = todayEnd
		periodLabel = fmt.Sprintf("7 Hari Terakhir (%s s.d. %s)", fromTime.Format("2006-01-02"), toTime.Format("2006-01-02"))

	case statsMonth:
		fromTime = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
		toTime = todayEnd
		periodLabel = fmt.Sprintf("Bulan Ini (%s s.d. %s)", fromTime.Format("2006-01-02"), toTime.Format("2006-01-02"))

	case statsYear:
		fromTime = time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, time.Local)
		toTime = todayEnd
		periodLabel = fmt.Sprintf("Tahun Ini (%s s.d. %s)", fromTime.Format("2006-01-02"), toTime.Format("2006-01-02"))

	default:
		// Custom range via --from dan/atau --to
		if statsFrom != "" {
			t, err := time.ParseInLocation("2006-01-02", statsFrom, time.Local)
			if err != nil {
				return fmt.Errorf("format flag --from tidak valid (gunakan YYYY-MM-DD): %w", err)
			}
			fromTime = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
		}

		if statsTo != "" {
			t, err := time.ParseInLocation("2006-01-02", statsTo, time.Local)
			if err != nil {
				return fmt.Errorf("format flag --to tidak valid (gunakan YYYY-MM-DD): %w", err)
			}
			toTime = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.Local)
		}

		if !fromTime.IsZero() && !toTime.IsZero() {
			if fromTime.After(toTime) {
				return fmt.Errorf("rentang tanggal tidak valid: --from tidak boleh lebih baru dari --to")
			}
			periodLabel = fmt.Sprintf("%s s.d. %s", fromTime.Format("2006-01-02"), toTime.Format("2006-01-02"))
		} else if !fromTime.IsZero() {
			periodLabel = fmt.Sprintf("Sejak %s", fromTime.Format("2006-01-02"))
		} else if !toTime.IsZero() {
			periodLabel = fmt.Sprintf("Hingga %s", toTime.Format("2006-01-02"))
		}
	}

	// 3. Panggil query statistik via service
	filter := service.StatsFilter{
		From: fromTime,
		To:   toTime,
	}

	stats, err := app.scrobbleService.GetStats(ctx, filter)
	if err != nil {
		return fmt.Errorf("gagal mengambil statistik: %w", err)
	}

	// 4. Render output ke terminal
	fmt.Println("=== Scrobbles Statistics ===")
	fmt.Printf("Periode         : %s\n\n", periodLabel)
	fmt.Printf("Total Scrobbles : %s\n", formatNumber(stats.TotalScrobbles))
	fmt.Printf("Unique Tracks   : %s\n", formatNumber(stats.TotalTracks))
	fmt.Printf("Unique Artists  : %s\n", formatNumber(stats.TotalArtists))
	fmt.Printf("Unique Albums   : %s\n", formatNumber(stats.TotalAlbums))

	return nil
}

// formatNumber memformat angka integer dengan pemisah ribuan titik (1234567 -> 1.234.567)
func formatNumber(n int) string {
	if n < 0 {
		return fmt.Sprintf("-%s", formatNumber(-n))
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	var res []byte
	rem := len(s) % 3
	if rem > 0 {
		res = append(res, s[:rem]...)
	}

	for i := rem; i < len(s); i += 3 {
		if len(res) > 0 {
			res = append(res, '.')
		}
		res = append(res, s[i:i+3]...)
	}

	return string(res)
}
