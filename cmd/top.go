package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
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

var topCmd = &cobra.Command{
	Use:   "top [track|artist|album]",
	Short: "Menampilkan ranking track, artist, atau album yang paling sering diputar",
	Long: `Menampilkan daftar peringkat trek, artis, atau album teratas berdasarkan jumlah scrobble.
Mendukung filter waktu preset (--day, --week, --month, --year) maupun custom range (--from dan --to).
Jika dipanggil tanpa subperintah, default akan menampilkan peringkat track teratas.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTop(cmd, "Track")
	},
}

var topTrackCmd = &cobra.Command{
	Use:   "track",
	Short: "Menampilkan daftar track lagu teratas",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTop(cmd, "Track")
	},
}

var topArtistCmd = &cobra.Command{
	Use:   "artist",
	Short: "Menampilkan daftar artist teratas",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTop(cmd, "Artist")
	},
}

var topAlbumCmd = &cobra.Command{
	Use:   "album",
	Short: "Menampilkan daftar album teratas",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTop(cmd, "Album")
	},
}

func init() {
	rootCmd.AddCommand(topCmd)
	topCmd.AddCommand(topTrackCmd)
	topCmd.AddCommand(topArtistCmd)
	topCmd.AddCommand(topAlbumCmd)

	topCmd.PersistentFlags().StringVar(&topFrom, "from", "", "Batas awal tanggal (format: YYYY-MM-DD)")
	topCmd.PersistentFlags().StringVar(&topTo, "to", "", "Batas akhir tanggal (format: YYYY-MM-DD)")
	topCmd.PersistentFlags().BoolVar(&topDay, "day", false, "Peringkat hari ini")
	topCmd.PersistentFlags().BoolVar(&topWeek, "week", false, "Peringkat 7 hari terakhir")
	topCmd.PersistentFlags().BoolVar(&topMonth, "month", false, "Peringkat bulan berjalan")
	topCmd.PersistentFlags().BoolVar(&topYear, "year", false, "Peringkat tahun berjalan")
	topCmd.PersistentFlags().IntVarP(&topLimit, "limit", "n", 10, "Jumlah baris data yang ditampilkan (default 10, max 100)")
}

func runTop(cmd *cobra.Command, targetType string) error {
	ctx := cmd.Context()
	limit := normalizeLimit(topLimit, 10, 100)

	fromTime, toTime, periodLabel, err := parsePeriodRange(topFrom, topTo, topDay, topWeek, topMonth, topYear)
	if err != nil {
		return err
	}

	topSvc, err := app.TopService()
	if err != nil {
		return humanizeError(err)
	}

	filter := model.TopFilter{
		From:  fromTime,
		To:    toTime,
		Limit: limit,
	}

	var items []model.TopItem
	switch targetType {
	case "Artist":
		items, err = topSvc.GetTopArtists(ctx, filter)
	case "Album":
		items, err = topSvc.GetTopAlbums(ctx, filter)
	case "Track":
		items, err = topSvc.GetTopTracks(ctx, filter)
	}

	if err != nil {
		return humanizeError(err)
	}

	fmt.Printf("=== Top %d %ss (%s) ===\n\n", limit, targetType, periodLabel)
	if len(items) == 0 {
		fmt.Println("Tidak ada data scrobble pada periode ini.")
		return nil
	}

	names := make([]string, len(items))
	for i, item := range items {
		names[i] = item.Name
	}
	colWidth := calcColumnWidth(names, 25, 45)

	for i, item := range items {
		paddedName := padRight(item.Name, colWidth)
		formattedCount := formatNumber(item.Count)

		if item.Detail != "" {
			fmt.Printf(" %2d.  %s  by %s  %6s scrobbles\n", i+1, paddedName, padRight(item.Detail, 20), formattedCount)
		} else {
			fmt.Printf(" %2d.  %s  %6s scrobbles\n", i+1, paddedName, formattedCount)
		}
	}

	return nil
}
