package cmd

import (
	"fmt"

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

	limit, page := normalizePagination(historyLimit, historyPage, 50, 200)

	fromTime, toTime, _, err := parseDateRange(historyFrom, historyTo)
	if err != nil {
		return err
	}

	scrobbleSvc, err := app.ScrobbleService()
	if err != nil {
		return humanizeError(err)
	}

	scrobbles, err := scrobbleSvc.ListScrobbles(ctx, service.ListFilter{
		Artist: historyArtist,
		Album:  historyAlbum,
		From:   fromTime,
		To:     toTime,
		Limit:  limit,
		Page:   page,
	})
	if err != nil {
		return humanizeError(err)
	}

	if len(scrobbles) == 0 {
		fmt.Println("Tidak ada riwayat scrobble yang ditemukan untuk kriteria ini.")
		return nil
	}

	fmt.Println("=== Scrobble History ===")
	for _, sc := range scrobbles {
		playedAtStr := sc.PlayedAt.Format("2006-01-02 15:04:05")

		albumInfo := ""
		if sc.RawAlbum.Valid && sc.RawAlbum.String != "" {
			albumInfo = fmt.Sprintf(" (%s)", sc.RawAlbum.String)
		}

		fmt.Printf("[%s] %s - %s%s\n", playedAtStr, sc.RawArtist, sc.RawTitle, albumInfo)
	}

	fmt.Printf("\nShowing %d results (Page %d)\n", len(scrobbles), page)
	return nil
}
