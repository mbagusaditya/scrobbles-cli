package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/mbagusaditya/scrobbles-cli/internal/service"
)

var (
	historyArtist string
	historyAlbum  string
	historyFrom   string
	historyTo     string
	historyLimit  int
	historyPage   int
)

var historyCmd = &cobra.Command{
	Use:     "history",
	Aliases: []string{"log", "logs"},
	Short:   "Menampilkan riwayat scrobble kronologis dari database",
	Long: `Menampilkan riwayat pemutaran scrobble lagu secara berurutan (dari yang terbaru).
Mendukung filter berdasarkan nama artis, nama album, rentang tanggal (--from / --to),
serta pagination (-n / -p).`,
	RunE: runHistory,
}

func init() {
	rootCmd.AddCommand(historyCmd)

	historyCmd.Flags().StringVarP(&historyArtist, "artist", "a", "", "Filter exact nama artis")
	historyCmd.Flags().StringVar(&historyAlbum, "album", "", "Filter exact nama album")
	historyCmd.Flags().StringVar(&historyFrom, "from", "", "Batas awal tanggal (format: YYYY-MM-DD)")
	historyCmd.Flags().StringVar(&historyTo, "to", "", "Batas akhir tanggal (format: YYYY-MM-DD)")
	historyCmd.Flags().IntVarP(&historyLimit, "limit", "n", 50, "Jumlah data per halaman (default 50, max 200)")
	historyCmd.Flags().IntVarP(&historyPage, "page", "p", 1, "Nomor halaman pagination (default 1)")
}

func runHistory(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// 1. Validasi dan clamping limit
	limit := historyLimit
	if limit > 200 {
		fmt.Fprintln(os.Stderr, "⚠️  Limit melebihi batas, otomatis disesuaikan ke maksimal 200.")
		limit = 200
	} else if limit <= 0 {
		limit = 50
	}

	// 2. Normalisasi page
	page := historyPage
	if page <= 0 {
		page = 1
	}

	// 3. Parsing tanggal rentang waktu
	var fromTime, toTime time.Time

	if historyFrom != "" {
		t, err := time.ParseInLocation("2006-01-02", historyFrom, time.Local)
		if err != nil {
			return fmt.Errorf("format flag --from tidak valid (gunakan YYYY-MM-DD): %w", err)
		}
		fromTime = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
	}

	if historyTo != "" {
		t, err := time.ParseInLocation("2006-01-02", historyTo, time.Local)
		if err != nil {
			return fmt.Errorf("format flag --to tidak valid (gunakan YYYY-MM-DD): %w", err)
		}
		toTime = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.Local)
	}

	if !fromTime.IsZero() && !toTime.IsZero() && fromTime.After(toTime) {
		return fmt.Errorf("rentang tanggal tidak valid: --from tidak boleh lebih baru dari --to")
	}

	// 4. Filter query untuk service
	filter := service.ListFilter{
		Artist: historyArtist,
		Album:  historyAlbum,
		From:   fromTime,
		To:     toTime,
		Limit:  limit,
		Page:   page,
	}

	// 5. Ambil data menggunakan ListScrobbles yang sudah ada di scrobbleService
	scrobbles, err := app.scrobbleService.ListScrobbles(ctx, filter)
	if err != nil {
		return humanizeError(err)
	}

	// 6. Tangani hasil kosong
	if len(scrobbles) == 0 {
		fmt.Println("Tidak ada riwayat scrobble yang ditemukan untuk kriteria ini.")
		return nil
	}

	// 7. Render riwayat ke terminal
	fmt.Println("=== Scrobble History ===")
	for _, sc := range scrobbles {
		playedAtStr := sc.PlayedAt.Format("2006-01-02 15:04:05")

		albumInfo := ""
		if sc.RawAlbum.Valid && sc.RawAlbum.String != "" {
			albumInfo = fmt.Sprintf(" (%s)", sc.RawAlbum.String)
		}

		fmt.Printf("[%s] %s - %s%s\n", playedAtStr, sc.RawArtist, sc.RawTitle, albumInfo)
	}

	// 8. Footer pagination
	fmt.Printf("\nShowing %d results (Page %d)\n", len(scrobbles), page)

	return nil
}
