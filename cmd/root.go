package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var app = NewApp()

var rootCmd = &cobra.Command{
	Use:   "scrobbles",
	Short: "CLI untuk mengurus data scrobble Last.fm kamu",
	Long: `scrobbles adalah CLI untuk sinkronisasi dan mengelola riwayat
scrobble musik dari Last.fm ke database Turso kamu sendiri.`,
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
