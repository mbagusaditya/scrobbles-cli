package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config menampung seluruh konfigurasi aplikasi yang diambil dari .env.
type Config struct {
	DatabaseURL        string
	DatabaseAuthToken  string
	LastfmAPIKey       string
	LastfmSharedSecret string
	LastfmUsername     string
}

// Load membaca file .env (kalau ada) lalu memuat environment variable
// ke dalam struct Config. Mengembalikan error kalau ada variabel wajib
// yang kosong.
func Load() (*Config, error) {
	// Abaikan error kalau file .env tidak ditemukan -> kita tetap coba
	// baca dari environment asli (berguna untuk deployment yang inject
	// env var langsung, bukan lewat file).
	_ = godotenv.Load()

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
		return fmt.Errorf("variabel .env berikut wajib diisi: %s", strings.Join(missing, ", "))
	}

	return nil
}
