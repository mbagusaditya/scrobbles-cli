package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/mbagusaditya/scrobbles-cli/internal/service"
)

var (
	listArtist string
	listAlbum  string
	listFrom   string
	listTo     string
	listLimit  int
	listPage   int
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Menampilkan riwayat scrobble track yang tersimpan di database",
	Long: `Menampilkan daftar track yang didengarkan bersama timestamp-nya.
Mendukung filter berdasarkan artis, album, rentang tanggal, serta pagination.`,
	RunE: runList,
}

func init() {
	rootCmd.AddCommand(listCmd)

	listCmd.Flags().StringVarP(&listArtist, "artist", "a", "", "Filter exact nama artis")
	listCmd.Flags().StringVar(&listAlbum, "album", "", "Filter exact nama album")
	listCmd.Flags().StringVar(&listFrom, "from", "", "Batas awal tanggal (format: YYYY-MM-DD)")
	listCmd.Flags().StringVar(&listTo, "to", "", "Batas akhir tanggal (format: YYYY-MM-DD)")
	listCmd.Flags().IntVarP(&listLimit, "limit", "n", 50, "Jumlah data per halaman (default 50, max 200)")
	listCmd.Flags().IntVarP(&listPage, "page", "p", 1, "Nomor halaman pagination (default 1)")
}

func runList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// 1. Validasi dan clamping limit
	limit := listLimit
	if limit > 200 {
		fmt.Fprintln(os.Stderr, "⚠️  Limit melebihi batas, otomatis disesuaikan ke maksimal 200.")
		limit = 200
	} else if limit <= 0 {
		limit = 50
	}

	// 2. Normalisasi page
	page := listPage
	if page <= 0 {
		page = 1
	}

	// 3. Parsing tanggal rentang waktu
	var fromTime, toTime time.Time

	if listFrom != "" {
		t, err := time.ParseInLocation("2006-01-02", listFrom, time.Local)
		if err != nil {
			return fmt.Errorf("format flag --from tidak valid (gunakan YYYY-MM-DD): %w", err)
		}
		// Awal hari: 00:00:00
		fromTime = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
	}

	if listTo != "" {
		t, err := time.ParseInLocation("2006-01-02", listTo, time.Local)
		if err != nil {
			return fmt.Errorf("format flag --to tidak valid (gunakan YYYY-MM-DD): %w", err)
		}
		// Akhir hari: 23:59:59
		toTime = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.Local)
	}

	if !fromTime.IsZero() && !toTime.IsZero() && fromTime.After(toTime) {
		return fmt.Errorf("rentang tanggal tidak valid: --from tidak boleh lebih baru dari --to")
	}

	// 4. Siapkan filter query untuk service
	filter := service.ListFilter{
		Artist: listArtist,
		Album:  listAlbum,
		From:   fromTime,
		To:     toTime,
		Limit:  limit,
		Page:   page,
	}

	// 5. Eksekusi query via scrobbleService (instance yang sudah di-wire di root.go)
	scrobbles, err := app.scrobbleService.ListScrobbles(ctx, filter)
	if err != nil {
		return fmt.Errorf("gagal mengambil data scrobble: %w", err)
	}

	// 6. Handle hasil kosong
	if len(scrobbles) == 0 {
		fmt.Println("Tidak ada data scrobble yang ditemukan.")
		return nil
	}

	// 7. Render output teks terformat
	for _, sc := range scrobbles {
		playedAtStr := sc.PlayedAt.Format("2006-01-02 15:04:05")

		albumInfo := ""
		if sc.RawAlbum.Valid && sc.RawAlbum.String != "" {
			albumInfo = fmt.Sprintf(" (%s)", sc.RawAlbum.String)
		}

		fmt.Printf("[%s] %s - %s%s\n", playedAtStr, sc.RawArtist, sc.RawTitle, albumInfo)
	}

	// 8. Footer informasi pagination
	fmt.Printf("\nShowing %d results (Page %d)\n", len(scrobbles), page)

	return nil
}
