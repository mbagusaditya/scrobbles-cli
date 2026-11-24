package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
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
	Long: `Menampilkan ringkasan total scrobbles, library unik, waktu putar (aktual/estimasi),
serta kebiasaan mendengarkan (rata-rata harian, hari tersibuk, jam puncak, dan repeat ratio).
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
	ctx := cmd.Context()

	fromTime, toTime, periodLabel, err := parsePeriodRange(statsFrom, statsTo, statsDay, statsWeek, statsMonth, statsYear)
	if err != nil {
		return err
	}

	statsSvc, err := app.StatsService()
	if err != nil {
		return humanizeError(err)
	}

	stats, err := statsSvc.GetStats(ctx, model.StatsFilter{
		From: fromTime,
		To:   toTime,
	})
	if err != nil {
		return humanizeError(err)
	}

	if stats.TotalScrobbles == 0 {
		fmt.Println("=== Scrobbles Statistics ===")
		fmt.Printf("Periode: %s\n\n", periodLabel)
		fmt.Println("Tidak ada data scrobble pada periode ini.")
		return nil
	}

	totalHours := float64(stats.TotalDurationMs) / (1000.0 * 60.0 * 60.0)
	totalDaysListening := totalHours / 24.0

	var activeRatio float64
	if stats.TotalDays > 0 {
		activeRatio = (float64(stats.ActiveDays) / float64(stats.TotalDays)) * 100.0
	}

	fmt.Println("=== Scrobbles Statistics ===")
	if periodLabel == "All Time" && !stats.FirstScrobble.IsZero() {
		fmt.Printf("Periode           : All Time (%s s.d. %s)\n\n",
			stats.FirstScrobble.Format("2006-01-02"),
			stats.LastScrobble.Format("2006-01-02"))
	} else {
		fmt.Printf("Periode           : %s\n\n", periodLabel)
	}

	fmt.Println("--- Overview ---")
	fmt.Printf("Total Scrobbles   : %s\n", formatNumber(stats.TotalScrobbles))
	if stats.ActualDurationCount == stats.TotalScrobbles {
		fmt.Printf("Listening Time    : %.1f Jam (%.1f Hari)\n", totalHours, totalDaysListening)
	} else if stats.ActualDurationCount > 0 {
		fmt.Printf("Listening Time    : ~%.1f Jam (~%.1f Hari) [%.1f%% durasi aktual]\n",
			totalHours, totalDaysListening, stats.ActualDurationRatio)
	} else {
		fmt.Printf("Estimated Time    : ~%.1f Jam (~%.1f Hari) [est. 3,5m/lagu]\n",
			totalHours, totalDaysListening)
	}
	fmt.Printf("Unique Tracks     : %s\n", formatNumber(stats.TotalTracks))
	fmt.Printf("Unique Artists    : %s\n", formatNumber(stats.TotalArtists))
	fmt.Printf("Unique Albums     : %s\n\n", formatNumber(stats.TotalAlbums))

	fmt.Println("--- Listening Habit ---")
	fmt.Printf("Avg / Active Day  : %.1f scrobbles/hari\n", stats.AvgPerDay)
	if stats.TotalDays > 1 {
		fmt.Printf("Active Days       : %d / %d hari (%.1f%%)\n", stats.ActiveDays, stats.TotalDays, activeRatio)
	} else {
		fmt.Printf("Active Days       : %d hari\n", stats.ActiveDays)
	}

	if stats.PeakDayDate != "" {
		fmt.Printf("Peak Day          : %s (%s scrobbles)\n", stats.PeakDayDate, formatNumber(stats.PeakDayHits))
	}
	fmt.Printf("Peak Hour         : %02d:00 - %02d:00\n", stats.PeakHour, (stats.PeakHour+1)%24)
	fmt.Printf("Repeat Ratio      : %.1fx per track\n", stats.RepeatRatio)

	return nil
}
