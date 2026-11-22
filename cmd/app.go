package cmd

import (
	"database/sql"
	"fmt"

	"github.com/mbagusaditya/scrobbles-cli/internal/config"
	"github.com/mbagusaditya/scrobbles-cli/internal/db"
	"github.com/mbagusaditya/scrobbles-cli/internal/lastfm"
	"github.com/mbagusaditya/scrobbles-cli/internal/repository"
	"github.com/mbagusaditya/scrobbles-cli/internal/service"
)

// App adalah Lazy Dependency Injection Container untuk subcommand CLI.
// Resource seperti konfigurasi, koneksi database, client Last.fm, repository,
// dan service hanya diinisialisasi saat method getter-nya pertama kali dipanggil.
type App struct {
	cfg    *config.Config
	db     *sql.DB
	client *lastfm.Client

	aliasSvc    *service.AliasService
	scrobbleSvc *service.ScrobbleService
	infoSvc     *service.InfoService
	latestSvc   *service.LatestService
	librarySvc  *service.LibraryService
	resolverSvc *service.ResolverService
	statsSvc    *service.StatsService
	topSvc      *service.TopService
}

// NewApp membuat instance App kosong tanpa membuka koneksi database atau memuat file config.
func NewApp() *App {
	return &App{}
}

// Config memuat konfigurasi aplikasi sekali dan menyimpannya di cache.
func (a *App) Config() (*config.Config, error) {
	if a.cfg != nil {
		return a.cfg, nil
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("gagal memuat konfigurasi: %w", err)
	}

	a.cfg = cfg
	return a.cfg, nil
}

// DB membuka koneksi database (*sql.DB) secara lazy hanya ketika dibutuhkan.
func (a *App) DB() (*sql.DB, error) {
	if a.db != nil {
		return a.db, nil
	}

	cfg, err := a.Config()
	if err != nil {
		return nil, err
	}

	database, err := db.Connect(cfg)
	if err != nil {
		return nil, fmt.Errorf("gagal konek ke database: %w", err)
	}

	a.db = database
	return a.db, nil
}

// LastfmClient membuat instance API client Last.fm secara lazy berdasarkan konfigurasi.
func (a *App) LastfmClient() (*lastfm.Client, error) {
	if a.client != nil {
		return a.client, nil
	}

	cfg, err := a.Config()
	if err != nil {
		return nil, err
	}

	a.client = lastfm.NewClient(cfg.LastfmAPIKey, cfg.LastfmUsername)
	return a.client, nil
}

// AliasService memuat aliases.json secara lazy tanpa memicu koneksi database.
func (a *App) AliasService() (*service.AliasService, error) {
	if a.aliasSvc != nil {
		return a.aliasSvc, nil
	}

	aliasRepo, err := repository.NewAliasRepository("")
	if err != nil {
		return nil, fmt.Errorf("gagal menyiapkan repository alias: %w", err)
	}

	svc, err := service.NewAliasService(aliasRepo)
	if err != nil {
		return nil, fmt.Errorf("gagal memuat konfigurasi alias: %w", err)
	}

	a.aliasSvc = svc
	return a.aliasSvc, nil
}

// ScrobbleService merakit ScrobbleRepository -> ScrobbleService secara lazy.
func (a *App) ScrobbleService() (*service.ScrobbleService, error) {
	if a.scrobbleSvc != nil {
		return a.scrobbleSvc, nil
	}

	database, err := a.DB()
	if err != nil {
		return nil, err
	}

	client, err := a.LastfmClient()
	if err != nil {
		return nil, err
	}

	aliasSvc, err := a.AliasService()
	if err != nil {
		return nil, err
	}

	repo := repository.NewScrobbleRepository(database)
	a.scrobbleSvc = service.NewScrobbleService(repo, client, aliasSvc)
	return a.scrobbleSvc, nil
}

// InfoService merakit InfoRepository -> InfoService secara lazy.
func (a *App) InfoService() (*service.InfoService, error) {
	if a.infoSvc != nil {
		return a.infoSvc, nil
	}

	database, err := a.DB()
	if err != nil {
		return nil, err
	}

	client, err := a.LastfmClient()
	if err != nil {
		return nil, err
	}

	aliasSvc, err := a.AliasService()
	if err != nil {
		return nil, err
	}

	repo := repository.NewInfoRepository(database)
	a.infoSvc = service.NewInfoService(repo, client, aliasSvc)
	return a.infoSvc, nil
}

// LatestService merakit LatestRepository -> LatestService secara lazy.
func (a *App) LatestService() (*service.LatestService, error) {
	if a.latestSvc != nil {
		return a.latestSvc, nil
	}

	database, err := a.DB()
	if err != nil {
		return nil, err
	}

	aliasSvc, err := a.AliasService()
	if err != nil {
		return nil, err
	}

	repo := repository.NewLatestRepository(database)
	a.latestSvc = service.NewLatestService(repo, aliasSvc)
	return a.latestSvc, nil
}

// LibraryService merakit LibraryRepository -> LibraryService secara lazy.
func (a *App) LibraryService() (*service.LibraryService, error) {
	if a.librarySvc != nil {
		return a.librarySvc, nil
	}

	database, err := a.DB()
	if err != nil {
		return nil, err
	}

	aliasSvc, err := a.AliasService()
	if err != nil {
		return nil, err
	}

	repo := repository.NewLibraryRepository(database)
	a.librarySvc = service.NewLibraryService(repo, aliasSvc)
	return a.librarySvc, nil
}

// ResolverService merakit ResolverRepository + TrackRepository -> ResolverService secara lazy.
func (a *App) ResolverService() (*service.ResolverService, error) {
	if a.resolverSvc != nil {
		return a.resolverSvc, nil
	}

	database, err := a.DB()
	if err != nil {
		return nil, err
	}

	client, err := a.LastfmClient()
	if err != nil {
		return nil, err
	}

	aliasSvc, err := a.AliasService()
	if err != nil {
		return nil, err
	}

	resolverRepo := repository.NewResolverRepository(database)
	trackRepo := repository.NewTrackRepository(database)
	a.resolverSvc = service.NewResolverService(resolverRepo, trackRepo, client, aliasSvc)
	return a.resolverSvc, nil
}

// StatsService merakit StatsRepository + TrackRepository -> StatsService secara lazy.
func (a *App) StatsService() (*service.StatsService, error) {
	if a.statsSvc != nil {
		return a.statsSvc, nil
	}

	database, err := a.DB()
	if err != nil {
		return nil, err
	}

	aliasSvc, err := a.AliasService()
	if err != nil {
		return nil, err
	}

	statsRepo := repository.NewStatsRepository(database)
	trackRepo := repository.NewTrackRepository(database)
	a.statsSvc = service.NewStatsService(statsRepo, trackRepo, aliasSvc)
	return a.statsSvc, nil
}

// TopService merakit TopRepository -> TopService secara lazy.
func (a *App) TopService() (*service.TopService, error) {
	if a.topSvc != nil {
		return a.topSvc, nil
	}

	database, err := a.DB()
	if err != nil {
		return nil, err
	}

	aliasSvc, err := a.AliasService()
	if err != nil {
		return nil, err
	}

	repo := repository.NewTopRepository(database)
	a.topSvc = service.NewTopService(repo, aliasSvc)
	return a.topSvc, nil
}

// Close menutup koneksi database apabila sempat dibuka selama eksekusi command.
func (a *App) Close() error {
	if a.db != nil {
		err := a.db.Close()
		a.db = nil
		return err
	}
	return nil
}
