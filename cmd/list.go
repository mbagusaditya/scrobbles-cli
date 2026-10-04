package cmd

import (
	"fmt"
	"os"

	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
)

var (
	listLimit int
	listPage  int
)

var listCmd = &cobra.Command{
	Use:   "list [track|artist|album]",
	Short: "Menampilkan katalog pustaka musik (diurutkan A-Z)",
	Long: `Menampilkan daftar unik track, artist, atau album yang tersimpan di pustaka lokal.
Secara default diurutkan secara alfabetis (A-Z) dan mendukung pagination.
Jika dipanggil tanpa subperintah, default akan menampilkan katalog track.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLibraryList(cmd, "Track")
	},
}

var listTrackCmd = &cobra.Command{
	Use:   "track",
	Short: "Menampilkan katalog lagu/track yang pernah didengarkan",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLibraryList(cmd, "Track")
	},
}

var listArtistCmd = &cobra.Command{
	Use:   "artist",
	Short: "Menampilkan katalog artist yang pernah didengarkan",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLibraryList(cmd, "Artist")
	},
}

var listAlbumCmd = &cobra.Command{
	Use:   "album",
	Short: "Menampilkan katalog album yang pernah didengarkan",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLibraryList(cmd, "Album")
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.AddCommand(listTrackCmd)
	listCmd.AddCommand(listArtistCmd)
	listCmd.AddCommand(listAlbumCmd)

	listCmd.PersistentFlags().IntVarP(&listLimit, "limit", "n", 50, "Jumlah data per halaman (default 50, max 200)")
	listCmd.PersistentFlags().IntVarP(&listPage, "page", "p", 1, "Nomor halaman (default 1)")
}

func runLibraryList(cmd *cobra.Command, targetType string) error {
	ctx := cmd.Context()

	// 1. Clamping limit dan normalisasi page
	limit := listLimit
	if limit > 200 {
		fmt.Fprintln(os.Stderr, "⚠️  Limit melebihi batas, disesuaikan ke maksimal 200.")
		limit = 200
	} else if limit <= 0 {
		limit = 50
	}

	page := listPage
	if page <= 0 {
		page = 1
	}

	filter := model.LibraryFilter{
		Page:  page,
		Limit: limit,
	}

	// 2. Eksekusi query service berdasarkan tipe
	var result *model.LibraryResult
	var err error

	switch targetType {
	case "Artist":
		result, err = app.scrobbleService.ListLibraryArtists(ctx, filter)
	case "Album":
		result, err = app.scrobbleService.ListLibraryAlbums(ctx, filter)
	case "Track":
		result, err = app.scrobbleService.ListLibraryTracks(ctx, filter)
	}

	if err != nil {
		return humanizeError(err)
	}

	// 3. Tangani hasil kosong
	if result.TotalItems == 0 {
		fmt.Printf("Pustaka %s masih kosong. Belum ada scrobble yang tersimpan.\n", targetType)
		return nil
	}

	if len(result.Items) == 0 {
		fmt.Printf("Halaman %d kosong. Total hanya ada %d halaman.\n", page, result.TotalPages)
		return nil
	}

	// 4. Header katalog
	fmt.Printf("=== Library %ss (A-Z) ===\n\n", targetType)

	// Hitung padding visual kolom dinamis untuk menangani karakter CJK
	maxColWidth := 25
	for _, item := range result.Items {
		w := runewidth.StringWidth(item.Name)
		if w > maxColWidth {
			maxColWidth = w
		}
	}
	if maxColWidth > 45 {
		maxColWidth = 45
	}

	// Hitung nomor awal berdasarkan offset halaman
	startIndex := (page-1)*limit + 1

	for i, item := range result.Items {
		rowNumber := startIndex + i
		paddedName := padRight(item.Name, maxColWidth)
		formattedPlays := formatNumber(item.TotalPlays)

		if item.Detail != "" {
			fmt.Printf(" %3d.  %s  by %-20s  %5s plays\n", rowNumber, paddedName, item.Detail, formattedPlays)
		} else {
			fmt.Printf(" %3d.  %s  %5s plays\n", rowNumber, paddedName, formattedPlays)
		}
	}

	// 5. Footer informasi paginasi
	startItem := startIndex
	endItem := startIndex + len(result.Items) - 1

	fmt.Printf("\nShowing %d-%d of %s %ss (Page %d/%d)\n",
		startItem, endItem, formatNumber(result.TotalItems), targetType, page, result.TotalPages)

	if page < result.TotalPages {
		fmt.Printf("Gunakan '-p %d' untuk melihat halaman berikutnya.\n", page+1)
	}

	return nil
}
