package cmd

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/mbagusaditya/scrobbles-cli/internal/config"
	"github.com/mbagusaditya/scrobbles-cli/internal/db"
	"github.com/mbagusaditya/scrobbles-cli/internal/lastfm"
	"github.com/mbagusaditya/scrobbles-cli/internal/repository"
	"github.com/mbagusaditya/scrobbles-cli/internal/service"
)

type App struct {
	cfg      *config.Config
	db       *sql.DB
	remoteDB *sql.DB
	client   *lastfm.Client

	aliasSvc    *service.AliasService
	replicaSvc  *service.ReplicaService
	scrobbleSvc *service.ScrobbleService
	infoSvc     *service.InfoService
	latestSvc   *service.LatestService
	librarySvc  *service.LibraryService
	resolverSvc *service.ResolverService
	statsSvc    *service.StatsService
	topSvc      *service.TopService
}

func NewApp() *App {
	return &App{}
}

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

// DB membuka koneksi database SQLite lokal secara lazy.
func (a *App) DB() (*sql.DB, error) {
	if a.db != nil {
		return a.db, nil
	}

	cfg, err := a.Config()
	if err != nil {
		return nil, err
	}

	database, err := db.OpenLocal(cfg)
	if err != nil {
		return nil, fmt.Errorf("gagal membuka database lokal: %w", err)
	}

	a.db = database
	return a.db, nil
}

// RemoteDB membuka koneksi database Turso Cloud secara lazy (khusus untuk `scrobbles db`).
func (a *App) RemoteDB() (*sql.DB, error) {
	if a.remoteDB != nil {
		return a.remoteDB, nil
	}

	cfg, err := a.Config()
	if err != nil {
		return nil, err
	}

	database, err := db.OpenRemote(cfg)
	if err != nil {
		return nil, err
	}

	a.remoteDB = database
	return a.remoteDB, nil
}

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

func (a *App) ReplicaService() (*service.ReplicaService, error) {
	if a.replicaSvc != nil {
		return a.replicaSvc, nil
	}

	localDB, err := a.DB()
	if err != nil {
		return nil, err
	}

	remoteDB, err := a.RemoteDB()
	if err != nil {
		return nil, err
	}

	localRepo := repository.NewReplicaRepository(localDB)
	remoteRepo := repository.NewReplicaRepository(remoteDB)
	a.replicaSvc = service.NewReplicaService(localRepo, remoteRepo)
	return a.replicaSvc, nil
}

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

// Close menutup seluruh koneksi database (lokal maupun remote) yang sempat dibuka.
func (a *App) Close() error {
	var errs []error
	if a.db != nil {
		if err := a.db.Close(); err != nil {
			errs = append(errs, err)
		}
		a.db = nil
	}
	if a.remoteDB != nil {
		if err := a.remoteDB.Close(); err != nil {
			errs = append(errs, err)
		}
		a.remoteDB = nil
	}
	return errors.Join(errs...)
}
