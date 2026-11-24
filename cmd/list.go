package cmd

import (
	"fmt"

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
	limit, page := normalizePagination(listLimit, listPage, 50, 200)

	librarySvc, err := app.LibraryService()
	if err != nil {
		return humanizeError(err)
	}

	filter := model.LibraryFilter{
		Page:  page,
		Limit: limit,
	}

	var result *model.LibraryResult
	switch targetType {
	case "Artist":
		result, err = librarySvc.ListLibraryArtists(ctx, filter)
	case "Album":
		result, err = librarySvc.ListLibraryAlbums(ctx, filter)
	case "Track":
		result, err = librarySvc.ListLibraryTracks(ctx, filter)
	}

	if err != nil {
		return humanizeError(err)
	}

	if result.TotalItems == 0 {
		fmt.Printf("Pustaka %s masih kosong. Belum ada scrobble yang tersimpan.\n", targetType)
		return nil
	}

	if len(result.Items) == 0 {
		fmt.Printf("Halaman %d kosong. Total hanya ada %d halaman.\n", page, result.TotalPages)
		return nil
	}

	fmt.Printf("=== Library %ss (A-Z) ===\n\n", targetType)

	names := make([]string, len(result.Items))
	for i, item := range result.Items {
		names[i] = item.Name
	}
	maxColWidth := calcColumnWidth(names, 25, 45)

	startIndex := (page-1)*limit + 1

	for i, item := range result.Items {
		rowNumber := startIndex + i
		paddedName := padRight(item.Name, maxColWidth)
		formattedPlays := formatNumber(item.TotalPlays)

		if item.Detail != "" {
			fmt.Printf(" %3d.  %s  by %s  %5s plays\n", rowNumber, paddedName, padRight(item.Detail, 20), formattedPlays)
		} else {
			fmt.Printf(" %3d.  %s  %5s plays\n", rowNumber, paddedName, formattedPlays)
		}
	}

	startItem := startIndex
	endItem := startIndex + len(result.Items) - 1

	fmt.Printf("\nShowing %d-%d of %s %ss (Page %d/%d)\n",
		startItem, endItem, formatNumber(result.TotalItems), targetType, page, result.TotalPages)

	if page < result.TotalPages {
		fmt.Printf("Gunakan '-p %d' untuk melihat halaman berikutnya.\n", page+1)
	}

	return nil
}
