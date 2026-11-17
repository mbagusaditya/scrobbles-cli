package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
)

const aliasFileName = "aliases.json"

// ResolveAliasPath menentukan lokasi file aliases.json berdasarkan hierarki prioritas:
// 1. ./aliases.json di current working directory (jika file sudah ada)
// 2. Standar XDG Base Directory: <UserConfigDir>/scrobbles/aliases.json
func ResolveAliasPath() (string, error) {
	// 1. Prioritas utama: aliases.json di current working directory
	cwdPath := aliasFileName
	if _, err := os.Stat(cwdPath); err == nil {
		return cwdPath, nil
	}

	// 2. Prioritas kedua: Standar XDG Base Directory
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("gagal mendapatkan direktori konfigurasi user: %w", err)
	}

	return filepath.Join(configDir, "scrobbles", aliasFileName), nil
}

// LoadAliasConfig membaca file aliases.json dari path yang diberikan.
// Jika path kosong, otomatis mencarinya menggunakan ResolveAliasPath().
// Jika file belum ada di lokasi manapun, mengembalikan struct kosong tanpa error.
func LoadAliasConfig(path string) (*model.AliasConfig, error) {
	if path == "" {
		var err error
		path, err = ResolveAliasPath()
		if err != nil {
			return nil, err
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &model.AliasConfig{Tracks: []model.TrackAlias{}}, nil
		}
		return nil, fmt.Errorf("gagal membaca file alias %s: %w", path, err)
	}

	if len(data) == 0 {
		return &model.AliasConfig{Tracks: []model.TrackAlias{}}, nil
	}

	var cfg model.AliasConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("format JSON pada %s tidak valid: %w", path, err)
	}

	if cfg.Tracks == nil {
		cfg.Tracks = []model.TrackAlias{}
	}

	return &cfg, nil
}

// SaveAliasConfig menulis model.AliasConfig ke disk dalam format JSON yang rapi.
func SaveAliasConfig(path string, cfg *model.AliasConfig) error {
	if path == "" {
		var err error
		path, err = ResolveAliasPath()
		if err != nil {
			return err
		}
	}

	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("gagal membuat direktori config %s: %w", dir, err)
		}
	}

	if cfg.Tracks == nil {
		cfg.Tracks = []model.TrackAlias{}
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("gagal encode JSON alias: %w", err)
	}

	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("gagal menyimpan file alias %s: %w", path, err)
	}

	return nil
}
