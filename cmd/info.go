package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
)

var (
	infoArtist string
	infoFrom   string
	infoTo     string
)

var infoCmd = &cobra.Command{
	Use:   "info [track|artist|album] <query>",
	Short: "Menampilkan detail informasi dan statistik entitas musik",
	Long: `Menampilkan metadata global dari Last.fm yang dipadukan dengan statistik
pribadi dari database Turso lokal untuk artis, album, atau lagu tertentu.`,
}

var infoArtistCmd = &cobra.Command{
	Use:   "artist <name>",
	Short: "Menampilkan detail informasi artis",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInfo(cmd, "Artist", args[0])
	},
}

var infoAlbumCmd = &cobra.Command{
	Use:   "album <title>",
	Short: "Menampilkan detail informasi album",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInfo(cmd, "Album", args[0])
	},
}

var infoTrackCmd = &cobra.Command{
	Use:   "track <title>",
	Short: "Menampilkan detail informasi lagu/track",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInfo(cmd, "Track", args[0])
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
	infoCmd.AddCommand(infoArtistCmd)
	infoCmd.AddCommand(infoAlbumCmd)
	infoCmd.AddCommand(infoTrackCmd)

	infoCmd.PersistentFlags().StringVarP(&infoArtist, "artist", "a", "", "Nama artis (opsional untuk track/album)")
	infoCmd.PersistentFlags().StringVar(&infoFrom, "from", "", "Batas awal statistik (format: YYYY-MM-DD)")
	infoCmd.PersistentFlags().StringVar(&infoTo, "to", "", "Batas akhir statistik (format: YYYY-MM-DD)")
}

func runInfo(cmd *cobra.Command, targetType, query string) error {
	ctx := cmd.Context()

	fromTime, toTime, periodLabel, err := parseDateRange(infoFrom, infoTo)
	if err != nil {
		return err
	}

	filter := model.InfoFilter{
		Query:  query,
		Artist: infoArtist,
		From:   fromTime,
		To:     toTime,
	}

	infoSvc, err := app.InfoService()
	if err != nil {
		return humanizeError(err)
	}

	var res *model.EntityInfoResult
	switch targetType {
	case "Artist":
		res, err = infoSvc.GetArtistInfo(ctx, filter)
	case "Album":
		res, err = infoSvc.GetAlbumInfo(ctx, filter)
	case "Track":
		res, err = infoSvc.GetTrackInfo(ctx, filter)
	}

	if err != nil {
		return humanizeError(err)
	}

	fmt.Printf("=== %s Info: %s ===\n\n", res.Type, res.Name)

	fmt.Println("--- Global Metadata (Last.fm) ---")
	if res.Artist != "" {
		fmt.Printf("Artist        : %s\n", res.Artist)
	}
	if res.Album != "" {
		fmt.Printf("Album         : %s\n", res.Album)
	}
	if res.DurationSec > 0 {
		min := res.DurationSec / 60
		sec := res.DurationSec % 60
		fmt.Printf("Duration      : %d:%02d\n", min, sec)
	}
	if len(res.Tags) > 0 {
		fmt.Printf("Tags          : %s\n", strings.Join(res.Tags, ", "))
	}
	if res.GlobalListeners > 0 || res.GlobalPlaycount > 0 {
		fmt.Printf("Global Stats  : %s listeners | %s total plays\n",
			formatNumber(res.GlobalListeners), formatNumber(res.GlobalPlaycount))
	}
	if res.BioOrSummary != "" {
		fmt.Printf("Summary       : %s\n", res.BioOrSummary)
	}
	fmt.Println()

	fmt.Println("--- Your Statistics (Turso) ---")
	fmt.Printf("Periode       : %s\n", periodLabel)
	fmt.Printf("Your Plays    : %s scrobbles\n", formatNumber(res.UserPlaycount))
	if res.UserPlaycount > 0 {
		fmt.Printf("Share Ratio   : %.2f%% dari total pustaka (%s scrobbles)\n",
			res.ShareRatio, formatNumber(res.TotalLibrary))
		fmt.Printf("First Heard   : %s\n", res.FirstHeard.Format("2006-01-02 15:04"))
		fmt.Printf("Last Heard    : %s\n", res.LastHeard.Format("2006-01-02 15:04"))
	} else {
		fmt.Println("Status        : Belum pernah didengarkan di database lokal.")
	}

	return nil
}
