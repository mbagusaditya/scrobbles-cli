package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
)

var (
	dbSyncFull      bool
	dbSyncBatchSize int
)

var dbCmd = &cobra.Command{
	Use:   "db [push|pull|status]",
	Short: "Mengelola replikasi database SQLite lokal dengan Turso Cloud",
	Long: `Menyinkronkan data antara database SQLite lokal dan Turso Cloud secara eksplisit.
Perintah harian (sync, resolve, stats, top, dll.) berjalan 100% pada SQLite lokal.
Gunakan 'db push' untuk mengunggah perubahan lokal ke Turso, atau 'db pull' untuk menarik data dari Turso.`,
}

var dbPushCmd = &cobra.Command{
	Use:   "push",
	Short: "Mengunggah data scrobble dari SQLite lokal ke Turso Cloud",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDBTransfer(cmd, model.DirectionPush)
	},
}

var dbPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Menarik data scrobble dari Turso Cloud ke SQLite lokal",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDBTransfer(cmd, model.DirectionPull)
	},
}

var dbStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Membandingkan ringkasan isi SQLite lokal dan Turso Cloud",
	RunE:  runDBStatus,
}

func init() {
	rootCmd.AddCommand(dbCmd)
	dbCmd.AddCommand(dbPushCmd)
	dbCmd.AddCommand(dbPullCmd)
	dbCmd.AddCommand(dbStatusCmd)

	dbCmd.PersistentFlags().BoolVar(&dbSyncFull, "full", false, "Sinkronisasi seluruh riwayat penuh (tanpa watermark incremental)")
	dbCmd.PersistentFlags().IntVarP(&dbSyncBatchSize, "batch-size", "b", 500, "Jumlah baris per batch transaksi (default 500, max 5000)")
}

func runDBTransfer(cmd *cobra.Command, dir model.ReplicaDirection) error {
	ctx := cmd.Context()
	batchSize := normalizeLimit(dbSyncBatchSize, 500, 5000)

	replicaSvc, err := app.ReplicaService()
	if err != nil {
		return humanizeError(err)
	}

	modeLabel := "Incremental"
	if dbSyncFull {
		modeLabel = "Full Sync"
	}

	if dir == model.DirectionPush {
		fmt.Printf("Memulai PUSH (SQLite Lokal -> Turso Cloud) [%s]...\n\n", modeLabel)
	} else {
		fmt.Printf("Memulai PULL (Turso Cloud -> SQLite Lokal) [%s]...\n\n", modeLabel)
	}

	onProgress := func(p model.ReplicaProgress) {
		fmt.Printf("  [%s] %s / %s baris diproses\n",
			p.Stage, formatNumber(p.Processed), formatNumber(p.Total))
	}

	opts := model.ReplicaOptions{
		Full:      dbSyncFull,
		BatchSize: batchSize,
	}

	var res *model.ReplicaResult
	if dir == model.DirectionPush {
		res, err = replicaSvc.Push(ctx, opts, onProgress)
	} else {
		res, err = replicaSvc.Pull(ctx, opts, onProgress)
	}
	if err != nil {
		return humanizeError(err)
	}

	fmt.Printf("\nSinkronisasi %s selesai dalam %s!\n", dir, res.Duration.Round(1000000))
	fmt.Printf("Artists       : %s diproses (%s baru)\n", formatNumber(res.ArtistsProcessed), formatNumber(res.ArtistsInserted))
	fmt.Printf("Tracks        : %s diproses (%s baru)\n", formatNumber(res.TracksProcessed), formatNumber(res.TracksInserted))
	fmt.Printf("Scrobble Logs : %s diproses (%s disinkronkan)\n", formatNumber(res.LogsProcessed), formatNumber(res.LogsInserted))

	return nil
}

func runDBStatus(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	cfg, err := app.Config()
	if err != nil {
		return humanizeError(err)
	}

	replicaSvc, err := app.ReplicaService()
	if err != nil {
		return humanizeError(err)
	}

	local, remote, err := replicaSvc.InspectDatabases(ctx)
	if err != nil {
		return humanizeError(err)
	}

	localLatest := "-"
	if !local.LatestPlayed.IsZero() {
		localLatest = local.LatestPlayed.Format("2006-01-02 15:04:05")
	}

	remoteLatest := "-"
	if !remote.LatestPlayed.IsZero() {
		remoteLatest = remote.LatestPlayed.Format("2006-01-02 15:04:05")
	}

	fmt.Println("=== Database Replication Status ===")
	fmt.Printf("Local Path    : %s\n\n", cfg.LocalDBPath)

	fmt.Printf("%-15s | %-15s | %-15s\n", "Metrik", "SQLite Lokal", "Turso Cloud")
	fmt.Println("---------------------------------------------------")
	fmt.Printf("%-15s | %-15s | %-15s\n", "Artists", formatNumber(local.ArtistsCount), formatNumber(remote.ArtistsCount))
	fmt.Printf("%-15s | %-15s | %-15s\n", "Tracks", formatNumber(local.TracksCount), formatNumber(remote.TracksCount))
	fmt.Printf("%-15s | %-15s | %-15s\n", "Scrobble Logs", formatNumber(local.LogsCount), formatNumber(remote.LogsCount))
	fmt.Printf("%-15s | %-15s | %-15s\n", "Latest Scrobble", localLatest, remoteLatest)

	return nil
}
