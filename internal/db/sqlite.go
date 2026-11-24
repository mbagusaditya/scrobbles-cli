package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type SQLiteDriver struct {
	path string
}

func NewSQLiteDriver(path string) *SQLiteDriver {
	return &SQLiteDriver{path: path}
}

func (s *SQLiteDriver) Name() string {
	return "sqlite"
}

func (s *SQLiteDriver) Open() (*sql.DB, error) {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("gagal membuat direktori database lokal (%s): %w", dir, err)
	}

	// Aktifkan WAL mode, busy_timeout 5 detik, dan foreign_keys langsung melalui DSN pragma
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_pragma=synchronous(NORMAL)", s.path)

	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("gagal membuka database SQLite lokal: %w", err)
	}

	// SQLite single-writer: batasi koneksi terbuka agar tidak terjadi SQLITE_BUSY saat transaksi batch
	database.SetMaxOpenConns(1)

	if err := database.Ping(); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("gagal mengakses file SQLite lokal (%s): %w", s.path, err)
	}

	return database, nil
}
