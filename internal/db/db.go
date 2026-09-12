package db

import (
	"database/sql"
	"fmt"

	_ "github.com/tursodatabase/libsql-client-go/libsql"

	"github.com/mbagusaditya/scrobbles-cli/internal/config"
)

// Connect membuka koneksi ke database Turso menggunakan native driver
// libsql-client-go (tanpa CGO). Auth token dikirim sebagai query
// parameter pada DSN, sesuai format yang diharapkan driver ini.
func Connect(cfg *config.Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s?authToken=%s", cfg.DatabaseURL, cfg.DatabaseAuthToken)

	database, err := sql.Open("libsql", dsn)
	if err != nil {
		return nil, fmt.Errorf("gagal membuka koneksi ke Turso: %w", err)
	}

	if err := database.Ping(); err != nil {
		return nil, fmt.Errorf("gagal melakukan ping ke Turso (cek DATABASE_URL/DATABASE_AUTH_TOKEN): %w", err)
	}

	return database, nil
}
