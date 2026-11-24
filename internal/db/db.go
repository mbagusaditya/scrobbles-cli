package db

import (
	"database/sql"
	"fmt"

	"github.com/mbagusaditya/scrobbles-cli/internal/config"
)

// Driver mendefinisikan antarmuka standar untuk setiap implementasi database spesifik.
type Driver interface {
	Name() string
	Open() (*sql.DB, error)
}

// OpenLocal membuka koneksi database SQLite lokal dan memastikan skema tabel tersedia.
func OpenLocal(cfg *config.Config) (*sql.DB, error) {
	driver := NewSQLiteDriver(cfg.LocalDBPath)
	database, err := driver.Open()
	if err != nil {
		return nil, err
	}

	if err := EnsureSchema(database); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("gagal inisialisasi skema SQLite lokal: %w", err)
	}

	return database, nil
}

// OpenRemote memvalidasi kredensial Turso lalu membuka koneksi ke Turso Cloud.
func OpenRemote(cfg *config.Config) (*sql.DB, error) {
	if err := cfg.ValidateRemote(); err != nil {
		return nil, err
	}

	driver := NewTursoDriver(cfg.DatabaseURL, cfg.DatabaseAuthToken)
	database, err := driver.Open()
	if err != nil {
		return nil, err
	}

	return database, nil
}
