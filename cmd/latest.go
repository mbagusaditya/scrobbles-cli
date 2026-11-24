package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
)

var latestLimit int

var latestCmd = &cobra.Command{
	Use:   "latest [track|artist|album]",
	Short: "Menampilkan riwayat entitas yang paling baru diputar",
	Long: `Menampilkan daftar unik artist, album, atau track yang paling baru didengarkan.
Pilih salah satu subperintah: artist, album, atau track.
Jika dipanggil tanpa subperintah, default akan menampilkan track lagu terbaru.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLatest(cmd, "Track")
	},
}

var latestTrackCmd = &cobra.Command{
	Use:   "track",
	Short: "Menampilkan daftar track lagu terakhir yang didengarkan",
	RunE: func(cmd *cobra.Command, args []string) error {
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

func init() {
	rootCmd.AddCommand(latestCmd)
	latestCmd.AddCommand(latestTrackCmd)
	latestCmd.AddCommand(latestArtistCmd)
	latestCmd.AddCommand(latestAlbumCmd)

	latestCmd.PersistentFlags().IntVarP(&latestLimit, "limit", "n", 10, "Jumlah data yang ditampilkan (default 10, max 100)")
}

func runLatest(cmd *cobra.Command, targetType string) error {
	ctx := cmd.Context()
	limit := normalizeLimit(latestLimit, 10, 100)

	latestSvc, err := app.LatestService()
	if err != nil {
		return humanizeError(err)
	}

	var items []model.LatestItem
	switch targetType {
	case "Artist":
		items, err = latestSvc.GetLatestArtists(ctx, limit)
	case "Album":
		items, err = latestSvc.GetLatestAlbums(ctx, limit)
	case "Track":
		items, err = latestSvc.GetLatestTracks(ctx, limit)
	}

	if err != nil {
		return humanizeError(err)
	}

	if len(items) == 0 {
		fmt.Println("Belum ada riwayat scrobble di database.")
		return nil
	}

	fmt.Printf("=== Latest %d %ss ===\n\n", len(items), targetType)

	names := make([]string, len(items))
	for i, item := range items {
		names[i] = item.Name
	}
	maxColWidth := calcColumnWidth(names, 25, 45)

	for i, item := range items {
		timeStr := item.PlayedAt.Format("2006-01-02 15:04")
		if item.Detail != "" {
			fmt.Printf(" %2d.  %s  %s  by %s\n", i+1, timeStr, padRight(item.Name, maxColWidth), item.Detail)
		} else {
			fmt.Printf(" %2d.  %s  %s\n", i+1, timeStr, item.Name)
		}
	}

	return nil
}
