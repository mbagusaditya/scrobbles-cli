package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mbagusaditya/scrobbles-cli/internal/service"
)

var (
	resolveBatchSize int
	resolveDurations bool
)

var resolveCmd = &cobra.Command{
	Use:   "resolve",
	Short: "Menghubungkan scrobble_logs ke tabel master artists dan tracks",
	Long: `Mencari scrobble mentah yang belum terhubung (track_id IS NULL),
secara otomatis mengisi master artists dan tracks, lalu menautkan ID-nya kembali ke log.
Gunakan --durations (-d) untuk mengambil metadata durasi lagu dari Last.fm API.`,
	RunE: runResolve,
}

func init() {
	rootCmd.AddCommand(resolveCmd)

	resolveCmd.Flags().IntVarP(&resolveBatchSize, "batch-size", "b", 500, "Jumlah kombinasi lagu unik yang diproses per batch transaksi")
	resolveCmd.Flags().BoolVarP(&resolveDurations, "durations", "d", false, "Ambil durasi lagu aktual dari Last.fm untuk track yang durasinya masih kosong")
}

func runResolve(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	fmt.Println("Memeriksa scrobble logs yang belum terhubung (unlinked)...")

	batchIndex := 1
	onProgress := func(p service.ResolveProgress) {
		fmt.Printf("[Batch %d] Menautkan %d log (Artis Baru: +%d, Trek Baru: +%d) | Sisa log: %d\n",
			batchIndex, p.UpdatedLogs, p.NewArtists, p.NewTracks, p.Remaining)
		batchIndex++
	}

	res, err := app.scrobbleService.ResolveUnlinkedScrobbles(ctx, resolveBatchSize, onProgress)
	if err != nil {
		return humanizeError(err)
	}

	if res.TotalUpdatedLogs == 0 {
		fmt.Println("Semua scrobble sudah terhubung ke tabel artists dan tracks.")
	} else {
		fmt.Println("\nResolusi selesai dengan sukses!")
		fmt.Printf("Total Log Ditautkan : %s\n", formatNumber(res.TotalUpdatedLogs))
		fmt.Printf("Total Artis Baru    : %s\n", formatNumber(res.TotalNewArtists))
		fmt.Printf("Total Trek Baru     : %s\n", formatNumber(res.TotalNewTracks))
	}

	if resolveDurations {
		fmt.Println("\nMengambil metadata durasi lagu aktual dari Last.fm...")
		onDurProgress := func(p service.DurationProgress) {
			if p.Found {
				sec := p.DurationMs / 1000
				fmt.Printf("[%d/%d] ✓ %s - %s (%d:%02d)\n", p.Current, p.Total, p.Artist, p.Title, sec/60, sec%60)
			} else {
				fmt.Printf("[%d/%d] - %s - %s (durasi tidak tersedia di Last.fm)\n", p.Current, p.Total, p.Artist, p.Title)
			}
		}

		durRes, err := app.scrobbleService.SyncTrackDurations(ctx, onDurProgress)
		if err != nil {
			return humanizeError(err)
		}

		if durRes.Processed == 0 {
			fmt.Println("Semua track di database sudah memiliki data durasi.")
		} else {
			fmt.Println("\nSinkronisasi durasi selesai!")
			fmt.Printf("Durasi Diperbarui   : %s track\n", formatNumber(durRes.Updated))
			fmt.Printf("Tidak Tersedia      : %s track (akan memakai fallback 3,5 menit)\n", formatNumber(durRes.NotFound))
		}
	}

	return nil
}
