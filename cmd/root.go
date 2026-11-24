package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var app = NewApp()

var rootCmd = &cobra.Command{
	Use:   "scrobbles",
	Short: "CLI local-first untuk mengelola riwayat scrobble Last.fm",
	Long: `scrobbles adalah CLI local-first untuk sinkronisasi, analisis, dan pengelolaan
riwayat scrobble musik dari Last.fm ke database SQLite lokal dengan dukungan
sinkronisasi cadangan ke Turso Cloud melalui perintah 'scrobbles db'.`,
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return app.Close()
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
