package cmd

import (
	"fmt"

	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"

	"github.com/mbagusaditya/scrobbles-cli/internal/service"
)

var (
	latestArtist bool
	latestAlbum  bool
	latestTrack  bool
	latestLimit  int
)

var latestCmd = &cobra.Command{
	Use:   "latest",
	Short: "Menampilkan entitas (artist, album, atau track) yang paling baru diputar",
	Long: `Menampilkan daftar unik artist, album, atau trek yang paling baru didengarkan.
Gunakan salah satu flag: --artist, --album, atau --track.
Jika tidak ada flag yang diberikan, secara default akan menampilkan daftar track terbaru.`,
	RunE: runLatest,
}

func init() {
	rootCmd.AddCommand(latestCmd)

	latestCmd.Flags().BoolVar(&latestArtist, "artist", false, "Tampilkan artist terakhir yang didengarkan")
	latestCmd.Flags().BoolVar(&latestAlbum, "album", false, "Tampilkan album terakhir yang didengarkan")
	latestCmd.Flags().BoolVar(&latestTrack, "track", false, "Tampilkan trek lagu terakhir yang didengarkan")
	latestCmd.Flags().IntVarP(&latestLimit, "limit", "n", 10, "Jumlah data yang ditampilkan")
}

func runLatest(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// 1. Validasi mutual exclusion: maksimal hanya boleh 1 flag
	count := 0
	if latestArtist {
		count++
	}
	if latestAlbum {
		count++
	}
	if latestTrack {
		count++
	}

	if count > 1 {
		return fmt.Errorf("pilih salah satu filter saja: --artist, --album, atau --track")
	}

	// 2. Tentukan target eksekusi (default: Track)
	var items []service.LatestItem
	var err error
	headerTitle := "Tracks"

	switch {
	case latestArtist:
		headerTitle = "Artists"
		items, err = app.scrobbleService.GetLatestArtists(ctx, latestLimit)
	case latestAlbum:
		headerTitle = "Albums"
		items, err = app.scrobbleService.GetLatestAlbums(ctx, latestLimit)
	default:
		headerTitle = "Tracks"
		items, err = app.scrobbleService.GetLatestTracks(ctx, latestLimit)
	}

	if err != nil {
		return humanizeError(err)
	}

	if len(items) == 0 {
		fmt.Println("Belum ada riwayat scrobble di database.")
		return nil
	}

	// 3. Render tabel dengan padRight
	fmt.Printf("=== Latest %d %s ===\n\n", len(items), headerTitle)

	// Hitung lebar padding dinamis agar adaptif dengan panjang judul/nama terpanjang
	maxColWidth := 30
	for _, item := range items {
		w := runewidth.StringWidth(item.Name)
		if w > maxColWidth {
			maxColWidth = w
		}
	}
	// Beri batas atas agar tidak terlalu lebar di terminal kecil
	if maxColWidth > 45 {
		maxColWidth = 45
	}

	for i, item := range items {
		timeStr := item.PlayedAt.Format("2006-01-02 15:04")
		paddedName := padRight(item.Name, maxColWidth)

		if item.Detail != "" {
			fmt.Printf(" %2d.  %s  %s  by %s\n", i+1, timeStr, paddedName, item.Detail)
		} else {
			fmt.Printf(" %2d.  %s  %s\n", i+1, timeStr, item.Name)
		}
	}

	return nil
}
