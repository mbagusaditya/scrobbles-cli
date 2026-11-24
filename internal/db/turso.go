package db

import (
	"database/sql"
	"fmt"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

type TursoDriver struct {
	url       string
	authToken string
}

func NewTursoDriver(url, authToken string) *TursoDriver {
	return &TursoDriver{
		url:       url,
		authToken: authToken,
	}
}

func (t *TursoDriver) Name() string {
	return "turso"
}

func (t *TursoDriver) Open() (*sql.DB, error) {
	dsn := fmt.Sprintf("%s?authToken=%s", t.url, t.authToken)

	database, err := sql.Open("libsql", dsn)
	if err != nil {
		return nil, fmt.Errorf("gagal membuka koneksi ke Turso: %w", err)
	}

	if err := database.Ping(); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("gagal melakukan ping ke Turso (cek DATABASE_URL/DATABASE_AUTH_TOKEN): %w", err)
	}

	return database, nil
}
