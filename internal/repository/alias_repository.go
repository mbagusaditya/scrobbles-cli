package repository

import (
	"github.com/mbagusaditya/scrobbles-cli/internal/config"
	"github.com/mbagusaditya/scrobbles-cli/internal/model"
)

// AliasRepository mendefinisikan kontrak persistensi untuk konfigurasi alias judul track.
type AliasRepository interface {
	Location() string
	Load() (*model.AliasConfig, error)
	Save(cfg *model.AliasConfig) error
}

var _ AliasRepository = (*jsonAliasRepository)(nil)

type jsonAliasRepository struct {
	path string
}

// NewAliasRepository membuat implementasi AliasRepository berbasis file JSON lokal.
// Jika path kosong, lokasi file otomatis ditentukan melalui config.ResolveAliasPath().
func NewAliasRepository(path string) (AliasRepository, error) {
	if path == "" {
		var err error
		path, err = config.ResolveAliasPath()
		if err != nil {
			return nil, err
		}
	}

	return &jsonAliasRepository{path: path}, nil
}

func (r *jsonAliasRepository) Location() string {
	return r.path
}

func (r *jsonAliasRepository) Load() (*model.AliasConfig, error) {
	return config.LoadAliasConfig(r.path)
}

func (r *jsonAliasRepository) Save(cfg *model.AliasConfig) error {
	return config.SaveAliasConfig(r.path, cfg)
}
