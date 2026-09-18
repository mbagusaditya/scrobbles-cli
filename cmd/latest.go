package cmd

import (
	"fmt"

	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"

	"github.com/mbagusaditya/scrobbles-cli/internal/service"
)

var latestLimit int

// latestCmd bertindak sebagai parent command
var latestCmd = &cobra.Command{
	Use:   "latest [artist|album|track]",
	Short: "Menampilkan riwayat entitas yang paling baru diputar",
	Long: `Menampilkan daftar unik artist, album, atau track yang paling baru didengarkan.
Pilih salah satu subperintah: artist, album, atau track.
Jika dipanggil tanpa subperintah, default akan menampilkan track lagu terbaru.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Fallback default jika user hanya mengetik `scrobbles latest`
		return runLatest(cmd, "Track")
	},
}

var latestArtistCmd = &cobra.Command{
	Use:   "artist",
	Short: "Menampilkan daftar artist terakhir yang didengarkan",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLatest(cmd, "Artist")
	},
}

var latestAlbumCmd = &cobra.Command{
	Use:   "album",
	Short: "Menampilkan daftar album terakhir yang didengarkan",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLatest(cmd, "Album")
	},
}

var latestTrackCmd = &cobra.Command{
	Use:   "track",
	Short: "Menampilkan daftar track lagu terakhir yang didengarkan",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLatest(cmd, "Track")
	},
}

func init() {
	rootCmd.AddCommand(latestCmd)
	latestCmd.AddCommand(latestArtistCmd)
	latestCmd.AddCommand(latestAlbumCmd)
	latestCmd.AddCommand(latestTrackCmd)

	// PersistentFlags agar flag limit otomatis berlaku di latest maupun anak perintahnya
	latestCmd.PersistentFlags().IntVarP(&latestLimit, "limit", "n", 10, "Jumlah data yang ditampilkan")
}

func runLatest(cmd *cobra.Command, targetType string) error {
	ctx := cmd.Context()

	var items []service.LatestItem
	var err error

	switch targetType {
	case "Artist":
		items, err = app.scrobbleService.GetLatestArtists(ctx, latestLimit)
	case "Album":
		items, err = app.scrobbleService.GetLatestAlbums(ctx, latestLimit)
	case "Track":
		items, err = app.scrobbleService.GetLatestTracks(ctx, latestLimit)
	}

	if err != nil {
		return humanizeError(err)
	}

	if len(items) == 0 {
		fmt.Println("Belum ada riwayat scrobble di database.")
		return nil
	}

	// Render tabel dengan padRight
	fmt.Printf("=== Latest %d %s ===\n\n", len(items), targetType)

	maxColWidth := 25
	for _, item := range items {
		w := runewidth.StringWidth(item.Name)
		if w > maxColWidth {
			maxColWidth = w
		}
	}
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
