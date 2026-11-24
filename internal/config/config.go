package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	LocalDBPath        string
	DatabaseURL        string
	DatabaseAuthToken  string
	LastfmAPIKey       string
	LastfmSharedSecret string
	LastfmUsername     string
}

func Load() (*Config, error) {
	loadEnvironment()

	localPath := os.Getenv("LOCAL_DB_PATH")
	if localPath == "" {
		localPath = defaultLocalDBPath()
	}

	cfg := &Config{
		LocalDBPath:        localPath,
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		DatabaseAuthToken:  os.Getenv("DATABASE_AUTH_TOKEN"),
		LastfmAPIKey:       os.Getenv("LASTFM_API_KEY"),
		LastfmSharedSecret: os.Getenv("LASTFM_SHARED_SECRET"),
		LastfmUsername:     os.Getenv("LASTFM_USERNAME"),
	}

	if err := cfg.validateLocal(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func defaultLocalDBPath() string {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		if home, err := os.UserHomeDir(); err == nil {
			dataHome = filepath.Join(home, ".local", "share")
		}
	}
	if dataHome != "" {
		return filepath.Join(dataHome, "scrobbles", "scrobbles.db")
	}
	return "scrobbles.db"
}

func loadEnvironment() {
	if err := godotenv.Load(); err == nil {
		return
	}
	if configDir, err := os.UserConfigDir(); err == nil {
		_ = godotenv.Load(filepath.Join(configDir, "scrobbles", "config.env"))
	}
}

func (c *Config) validateLocal() error {
	var missing []string
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

// ValidateRemote dipanggil secara lazy hanya ketika menjalankan command push / pull ke Turso.
func (c *Config) ValidateRemote() error {
	var missing []string
	if c.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if c.DatabaseAuthToken == "" {
		missing = append(missing, "DATABASE_AUTH_TOKEN")
	}
	if len(missing) > 0 {
		return fmt.Errorf("konfigurasi Turso belum lengkap (wajib untuk push/pull): %s", strings.Join(missing, ", "))
	}
	return nil
}
