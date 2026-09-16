package cmd

import (
	"fmt"
	"time"

	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
	"github.com/mbagusaditya/scrobbles-cli/internal/service"
)

var (
	topFrom  string
	topTo    string
	topDay   bool
	topWeek  bool
	topMonth bool
	topYear  bool
	topLimit int
)

// topCmd bertindak sebagai parent command
var topCmd = &cobra.Command{
	Use:   "top [artist|album|track]",
	Short: "Menampilkan ranking artist, album, atau track yang paling sering diputar",
	Long: `Menampilkan daftar peringkat artis, album, atau trek teratas berdasarkan jumlah scrobble.
Mendukung filter waktu preset (--day, --week, --month, --year) maupun custom range (--from dan --to).`,
}

// topArtistCmd untuk peringkat artis
var topArtistCmd = &cobra.Command{
	Use:   "artist",
	Short: "Menampilkan daftar artist teratas",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTop(cmd, "Artist")
	},
}

// topAlbumCmd untuk peringkat album
var topAlbumCmd = &cobra.Command{
	Use:   "album",
	Short: "Menampilkan daftar album teratas",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTop(cmd, "Album")
	},
}

// topTrackCmd untuk peringkat lagu/track
var topTrackCmd = &cobra.Command{
	Use:   "track",
	Short: "Menampilkan daftar track lagu teratas",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTop(cmd, "Track")
	},
}

func init() {
	rootCmd.AddCommand(topCmd)
	topCmd.AddCommand(topArtistCmd)
	topCmd.AddCommand(topAlbumCmd)
	topCmd.AddCommand(topTrackCmd)

	// PersistentFlags agar tersedia di semua child command (artist, album, track)
	topCmd.PersistentFlags().StringVar(&topFrom, "from", "", "Batas awal tanggal (format: YYYY-MM-DD)")
	topCmd.PersistentFlags().StringVar(&topTo, "to", "", "Batas akhir tanggal (format: YYYY-MM-DD)")
	topCmd.PersistentFlags().BoolVar(&topDay, "day", false, "Peringkat hari ini")
	topCmd.PersistentFlags().BoolVar(&topWeek, "week", false, "Peringkat 7 hari terakhir")
	topCmd.PersistentFlags().BoolVar(&topMonth, "month", false, "Peringkat bulan berjalan")
	topCmd.PersistentFlags().BoolVar(&topYear, "year", false, "Peringkat tahun berjalan")
	topCmd.PersistentFlags().IntVarP(&topLimit, "limit", "n", 10, "Jumlah baris data yang ditampilkan")
}

func runTop(cmd *cobra.Command, targetType string) error {
	ctx := cmd.Context()
	now := time.Now()

	// 1. Validasi mutual exclusion preset flags
	presetCount := 0
	if topDay {
		presetCount++
	}
	if topWeek {
		presetCount++
	}
	if topMonth {
		presetCount++
	}
	if topYear {
		presetCount++
	}

	if presetCount > 1 {
		return fmt.Errorf("pilih salah satu preset saja: --day, --week, --month, atau --year")
	}

	if presetCount == 1 && (topFrom != "" || topTo != "") {
		return fmt.Errorf("flag preset (--day/--week/--month/--year) tidak dapat digabung dengan --from atau --to")
	}

	// 2. Evaluasi batas waktu [from, to]
	var fromTime, toTime time.Time
	periodLabel := "All Time"

	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	todayEnd := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.Local)

	switch {
	case topDay:
		fromTime = todayStart
		toTime = todayEnd
		periodLabel = fmt.Sprintf("Hari Ini (%s)", now.Format("2006-01-02"))

	case topWeek:
		fromTime = todayStart.AddDate(0, 0, -6)
		toTime = todayEnd
		periodLabel = fmt.Sprintf("7 Hari Terakhir (%s s.d. %s)", fromTime.Format("2006-01-02"), toTime.Format("2006-01-02"))

	case topMonth:
		fromTime = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
		toTime = todayEnd
		periodLabel = fmt.Sprintf("Bulan Ini (%s s.d. %s)", fromTime.Format("2006-01-02"), toTime.Format("2006-01-02"))

	case topYear:
		fromTime = time.Date(now.Year(), time.January, 1, 0, 0, 0, 0, time.Local)
		toTime = todayEnd
		periodLabel = fmt.Sprintf("Tahun Ini (%s s.d. %s)", fromTime.Format("2006-01-02"), toTime.Format("2006-01-02"))

	default:
		if topFrom != "" {
			t, err := time.ParseInLocation("2006-01-02", topFrom, time.Local)
			if err != nil {
				return fmt.Errorf("format flag --from tidak valid (gunakan YYYY-MM-DD): %w", err)
			}
			fromTime = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
		}

		if topTo != "" {
			t, err := time.ParseInLocation("2006-01-02", topTo, time.Local)
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

	filter := service.TopFilter{
		From:  fromTime,
		To:    toTime,
		Limit: topLimit,
	}

	// 3. Eksekusi query sesuai target subcommand
	var items []model.TopItem
	var err error

	switch targetType {
	case "Artist":
		items, err = app.scrobbleService.GetTopArtists(ctx, filter)
	case "Album":
		items, err = app.scrobbleService.GetTopAlbums(ctx, filter)
	case "Track":
		items, err = app.scrobbleService.GetTopTracks(ctx, filter)
	}

	if err != nil {
		return humanizeError(err)
	}

	// 4. Tampilkan output yang rapi
	fmt.Printf("=== Top %d %s (%s) ===\n\n", filter.Limit, targetType, periodLabel)
	if len(items) == 0 {
		fmt.Println("Tidak ada data scrobble pada periode ini.")
		return nil
	}

	// Hitung lebar visual kolom nama secara dinamis
	colWidth := 25
	for _, item := range items {
		w := runewidth.StringWidth(item.Name)
		if w > colWidth {
			colWidth = w
		}
	}
	if colWidth > 45 {
		colWidth = 45
	}

	for i, item := range items {
		paddedName := padRight(item.Name, colWidth)
		formattedCount := formatNumber(item.Count)

		if item.Detail != "" {
			fmt.Printf(" %2d.  %s  by %-20s  %6s scrobbles\n", i+1, paddedName, item.Detail, formattedCount)
		} else {
			fmt.Printf(" %2d.  %s  %6s scrobbles\n", i+1, paddedName, formattedCount)
		}
	}

	return nil
}
