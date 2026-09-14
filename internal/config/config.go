package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

// Config menampung seluruh konfigurasi aplikasi.
type Config struct {
	DatabaseURL        string
	DatabaseAuthToken  string
	LastfmAPIKey       string
	LastfmSharedSecret string
	LastfmUsername     string
}

// Load membaca konfigurasi dengan urutan prioritas:
// 1. File .env di direktori kerja aktif (lokal project / dev mode).
// 2. File XDG standard: $XDG_CONFIG_HOME/scrobbles/config.env (default ~/.config/scrobbles/config.env).
// 3. Environment variables sistem asli (export/container injection).
func Load() (*Config, error) {
	loadEnvironment()

	cfg := &Config{
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		DatabaseAuthToken:  os.Getenv("DATABASE_AUTH_TOKEN"),
		LastfmAPIKey:       os.Getenv("LASTFM_API_KEY"),
		LastfmSharedSecret: os.Getenv("LASTFM_SHARED_SECRET"),
		LastfmUsername:     os.Getenv("LASTFM_USERNAME"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// loadEnvironment mencoba memuat file konfigurasi tanpa menghentikan proses jika tidak ditemukan.
func loadEnvironment() {
	// 1. Prioritas utama: .env di current working directory
	if err := godotenv.Load(); err == nil {
		return
	}

	// 2. Prioritas kedua: Standar XDG Base Directory
	configDir, err := os.UserConfigDir()
	if err == nil {
		xdgConfigFile := filepath.Join(configDir, "scrobbles", "config.env")
		_ = godotenv.Load(xdgConfigFile)
	}

	// 3. Fallback: variabel env sistem bawaan akan otomatis dibaca via os.Getenv()
}

// validate memastikan semua variabel wajib terisi. LastfmSharedSecret
// sengaja TIDAK divalidasi wajib untuk sekarang karena belum dipakai
// di alur sync (baru dibutuhkan kalau ada fitur yang butuh signed request).
func (c *Config) validate() error {
	missing := []string{}

	if c.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if c.DatabaseAuthToken == "" {
		missing = append(missing, "DATABASE_AUTH_TOKEN")
	}
	if c.LastfmAPIKey == "" {
		missing = append(missing, "LASTFM_API_KEY")
	}
	if c.LastfmUsername == "" {
		missing = append(missing, "LASTFM_USERNAME")
	}

	if len(missing) > 0 {
		return fmt.Errorf("variabel konfigurasi berikut wajib diisi (cek .env atau ~/.config/scrobbles/config.env): %s", strings.Join(missing, ", "))
	}

	return nil
}
