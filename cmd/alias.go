package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

var (
	aliasArtist string
	aliasFrom   string
	aliasTo     string
)

var aliasCmd = &cobra.Command{
	Use:   "alias",
	Short: "Mengelola pemetaan alias judul lagu lokal (aliases.json)",
	Long: `Mengelola aturan normalisasi judul track pada file JSON lokal (~/.config/scrobbles/aliases.json)
untuk menggabungkan statistik lagu duplikat (misal variasi Kanji vs Romaji) tanpa mengubah database.`,
}

var aliasListCmd = &cobra.Command{
	Use:   "list",
	Short: "Menampilkan daftar aturan alias yang aktif",
	RunE: func(cmd *cobra.Command, args []string) error {
		aliases := app.aliasSvc.ListTrackAliases()
		fmt.Printf("File konfigurasi: %s\n\n", app.aliasSvc.ConfigPath())

		if len(aliases) == 0 {
			fmt.Println("Belum ada aturan alias yang terdaftar.")
			return nil
		}

		fmt.Printf("=== Track Aliases (%d rules) ===\n\n", len(aliases))
		for i, a := range aliases {
			fmt.Printf(" %2d. [%s] %q  ->  %q\n", i+1, a.Artist, a.From, a.To)
		}
		return nil
	},
}

var aliasAddCmd = &cobra.Command{
	Use:     "add",
	Short:   "Menambahkan aturan alias judul lagu baru",
	Example: `  scrobbles alias add -a "MyGO!!!!!" --from "孤壊牢 - KOKORO" --to "孤壊牢"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := app.aliasSvc.AddTrackAlias(aliasArtist, aliasFrom, aliasTo); err != nil {
			return humanizeError(err)
		}

		fmt.Printf("✅ Alias disimpan: [%s] %q -> %q\n", aliasArtist, aliasFrom, aliasTo)
		return nil
	},
}

var aliasRemoveCmd = &cobra.Command{
	Use:     "remove <nomor_urut>",
	Aliases: []string{"rm", "delete"},
	Short:   "Menghapus aturan alias berdasarkan nomor indeks di 'alias list'",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		idx, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("nomor indeks harus berupa angka: %w", err)
		}

		removed, err := app.aliasSvc.RemoveTrackAlias(idx)
		if err != nil {
			return humanizeError(err)
		}

		fmt.Printf("🗑️  Alias dihapus: [%s] %q -> %q\n", removed.Artist, removed.From, removed.To)
		return nil
	},
}

/*
// Subcommand apply (mutasi ke Turso DB) dinonaktifkan sementara.
var aliasApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Menerapkan aturan aliases.json ke data historis di database",
	RunE: func(cmd *cobra.Command, args []string) error {
		updated, err := app.aliasService.ApplyToDatabase(cmd.Context())
		if err != nil {
			return humanizeError(err)
		}
		fmt.Printf("✅ Berhasil menormalisasi %d baris scrobble di database.\n", updated)
		return nil
	},
}
*/

func init() {
	rootCmd.AddCommand(aliasCmd)
	aliasCmd.AddCommand(aliasListCmd)
	aliasCmd.AddCommand(aliasAddCmd)
	aliasCmd.AddCommand(aliasRemoveCmd)
	// aliasCmd.AddCommand(aliasApplyCmd)

	aliasAddCmd.Flags().StringVarP(&aliasArtist, "artist", "a", "", "Nama artis (wajib)")
	aliasAddCmd.Flags().StringVar(&aliasFrom, "from", "", "Judul lagu varian/lama yang ingin digabung (wajib)")
	aliasAddCmd.Flags().StringVar(&aliasTo, "to", "", "Judul lagu utama/kanonik tujuan (wajib)")

	_ = aliasAddCmd.MarkFlagRequired("artist")
	_ = aliasAddCmd.MarkFlagRequired("from")
	_ = aliasAddCmd.MarkFlagRequired("to")
}
