package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mbagusaditya/scrobbles-cli/internal/service"
)

var resolveBatchSize int

var resolveCmd = &cobra.Command{
	Use:   "resolve",
	Short: "Menghubungkan scrobble_logs ke tabel master artists dan tracks",
	Long: `Mencari scrobble mentah yang belum terhubung (track_id IS NULL),
secara otomatis mengisi master artists dan tracks, lalu menautkan ID-nya kembali ke log.`,
	RunE: runResolve,
}

func init() {
	rootCmd.AddCommand(resolveCmd)

	resolveCmd.Flags().IntVarP(&resolveBatchSize, "batch-size", "b", 500, "Jumlah kombinasi lagu unik yang diproses per batch transaksi")
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
		fmt.Println("Semua scrobble sudah terhubung ke tabel artists dan tracks. Tidak ada data pending.")
		return nil
	}

	fmt.Println("\nResolusi selesai dengan sukses!")
	fmt.Printf("Total Log Ditautkan : %s\n", formatNumber(res.TotalUpdatedLogs))
	fmt.Printf("Total Artis Baru    : %s\n", formatNumber(res.TotalNewArtists))
	fmt.Printf("Total Trek Baru     : %s\n", formatNumber(res.TotalNewTracks))

	return nil
}
