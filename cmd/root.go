package cmd

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mbagusaditya/scrobbles-cli/internal/config"
	"github.com/mbagusaditya/scrobbles-cli/internal/db"
	"github.com/mbagusaditya/scrobbles-cli/internal/lastfm"
	"github.com/mbagusaditya/scrobbles-cli/internal/service"
)

var app struct {
	config          *config.Config
	database        *sql.DB
	aliasService    *service.AliasService
	scrobbleService *service.ScrobbleService
}

var rootCmd = &cobra.Command{
	Use:   "scrobbles",
	Short: "CLI untuk mengurus data scrobble Last.fm kamu",
	Long: `scrobbles adalah CLI untuk sinkronisasi dan mengelola riwayat
scrobble musik dari Last.fm ke database Turso kamu sendiri.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initApp()
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if app.database != nil {
			return app.database.Close()
		}
		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}

func initApp() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("gagal memuat konfigurasi: %w", err)
	}

	aliasSvc, err := service.NewAliasService("")
	if err != nil {
		return fmt.Errorf("gagal memuat konfigurasi alias: %w", err)
	}

	database, err := db.Connect(cfg)
	if err != nil {
		return fmt.Errorf("gagal konek ke database: %w", err)
	}

	client := lastfm.NewClient(cfg.LastfmAPIKey, cfg.LastfmUsername)

	app.config = cfg
	app.database = database
	app.aliasService = aliasSvc
	app.scrobbleService = service.NewScrobbleService(database, client, aliasSvc)

	return nil
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
