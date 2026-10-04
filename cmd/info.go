package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
	"github.com/mbagusaditya/scrobbles-cli/internal/service"
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

	// 1. Parsing filter tanggal
	var fromTime, toTime time.Time
	periodLabel := "All Time"

	if infoFrom != "" {
		t, err := time.ParseInLocation("2006-01-02", infoFrom, time.Local)
		if err != nil {
			return fmt.Errorf("format flag --from tidak valid (gunakan YYYY-MM-DD): %w", err)
		}
		fromTime = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
	}

	if infoTo != "" {
		t, err := time.ParseInLocation("2006-01-02", infoTo, time.Local)
		if err != nil {
			return fmt.Errorf("format flag --to tidak valid (gunakan YYYY-MM-DD): %w", err)
		}
		toTime = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.Local)
	}

	if !fromTime.IsZero() && !toTime.IsZero() {
		if fromTime.After(toTime) {
			return fmt.Errorf("rentang tanggal tidak valid: --from tidak boleh lebih baru dari --to")
		}
		periodLabel = fmt.Sprintf("%s s.d. %s", fromTime.Format("2006-01-02"), toTime.Format("2006-01-02"))
	} else if !fromTime.IsZero() {
		periodLabel = fmt.Sprintf("Sejak %s", fromTime.Format("2006-01-02"))
	} else if !toTime.IsZero() {
		periodLabel = fmt.Sprintf("Hingga %s", toTime.Format("2006-01-02"))
	}

	filter := service.InfoFilter{
		Query:  query,
		Artist: infoArtist,
		From:   fromTime,
		To:     toTime,
	}

	// 2. Eksekusi query service
	var res *model.EntityInfoResult
	var err error

	switch targetType {
	case "Artist":
		res, err = app.scrobbleService.GetArtistInfo(ctx, filter)
	case "Album":
		res, err = app.scrobbleService.GetAlbumInfo(ctx, filter)
	case "Track":
		res, err = app.scrobbleService.GetTrackInfo(ctx, filter)
	}

	if err != nil {
		return humanizeError(err)
	}

	// 3. Render Output ke Terminal
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
