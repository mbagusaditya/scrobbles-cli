package service

import (
	// "database/sql"
	"fmt"
	"strings"

	"github.com/mbagusaditya/scrobbles-cli/internal/config"
	"github.com/mbagusaditya/scrobbles-cli/internal/model"
)

type AliasService struct {
	// db         *sql.DB
	configPath string
	cfg        *model.AliasConfig
	lookup     map[string]string // key: "lower(artist)\x00lower(from)" -> value: canonical "To"
}

// NewAliasService menginisialisasi AliasService dan memuat aliases.json ke memori.
// Parameter db boleh nil jika hanya dipakai untuk operasi file lokal tanpa DB backfill.
func NewAliasService(configPath string) (*AliasService, error) {
	if configPath == "" {
		var err error
		configPath, err = config.ResolveAliasPath()
		if err != nil {
			return nil, err
		}
	}

	svc := &AliasService{
		configPath: configPath,
		lookup:     make(map[string]string),
	}

	if err := svc.Reload(); err != nil {
		return nil, err
	}

	return svc, nil
}

// ConfigPath mengembalikan lokasi file aliases.json yang sedang aktif.
func (s *AliasService) ConfigPath() string {
	return s.configPath
}

// Reload memuat ulang aliases.json dari disk dan membangun kembali lookup map.
func (s *AliasService) Reload() error {
	cfg, err := config.LoadAliasConfig(s.configPath)
	if err != nil {
		return err
	}

	s.cfg = cfg
	s.rebuildLookup()
	return nil
}

func aliasLookupKey(artist, title string) string {
	return strings.ToLower(strings.TrimSpace(artist)) + "\x00" + strings.ToLower(strings.TrimSpace(title))
}

func (s *AliasService) rebuildLookup() {
	s.lookup = make(map[string]string, len(s.cfg.Tracks))
	for _, item := range s.cfg.Tracks {
		key := aliasLookupKey(item.Artist, item.From)
		s.lookup[key] = strings.TrimSpace(item.To)
	}
}

// ResolveTrackTitle mengembalikan judul kanonik jika pasangan (artist, title)
// terdaftar sebagai alias. Jika tidak terdaftar, mengembalikan title yang sudah di-trim.
func (s *AliasService) ResolveTrackTitle(artist, title string) string {
	cleanTitle := strings.TrimSpace(title)
	if s == nil || len(s.lookup) == 0 {
		return cleanTitle
	}

	// Telusuri maksimal 5 hop untuk mendukung transitive alias (A -> B -> C)
	current := cleanTitle
	for i := 0; i < 5; i++ {
		next, found := s.lookup[aliasLookupKey(artist, current)]
		if !found || strings.EqualFold(next, current) {
			break
		}
		current = next
	}

	return current
}

// ListTrackAliases mengembalikan seluruh daftar aturan alias yang tersimpan.
func (s *AliasService) ListTrackAliases() []model.TrackAlias {
	if s.cfg == nil {
		return nil
	}
	res := make([]model.TrackAlias, len(s.cfg.Tracks))
	copy(res, s.cfg.Tracks)
	return res
}

// AddTrackAlias menambahkan atau memperbarui aturan alias baru ke aliases.json.
func (s *AliasService) AddTrackAlias(artist, fromTitle, toTitle string) error {
	artist = strings.TrimSpace(artist)
	fromTitle = strings.TrimSpace(fromTitle)
	toTitle = strings.TrimSpace(toTitle)

	if artist == "" || fromTitle == "" || toTitle == "" {
		return fmt.Errorf("artist, judul asal (--from), dan judul target (--to) tidak boleh kosong")
	}

	if strings.EqualFold(fromTitle, toTitle) {
		return fmt.Errorf("judul asal dan judul target tidak boleh sama (%q)", fromTitle)
	}

	// Cek potensi circular alias (misal sudah ada rule toTitle -> fromTitle)
	resolvedTarget := s.ResolveTrackTitle(artist, toTitle)
	if strings.EqualFold(resolvedTarget, fromTitle) {
		return fmt.Errorf("terdeteksi circular alias: %q sudah mengarah kembali ke %q", toTitle, fromTitle)
	}

	// Jika aturan untuk (artist, fromTitle) sudah ada, perbarui targetnya
	updated := false
	for i, existing := range s.cfg.Tracks {
		if strings.EqualFold(strings.TrimSpace(existing.Artist), artist) &&
			strings.EqualFold(strings.TrimSpace(existing.From), fromTitle) {
			s.cfg.Tracks[i].Artist = artist
			s.cfg.Tracks[i].From = fromTitle
			s.cfg.Tracks[i].To = toTitle
			updated = true
			break
		}
	}

	if !updated {
		s.cfg.Tracks = append(s.cfg.Tracks, model.TrackAlias{
			Artist: artist,
			From:   fromTitle,
			To:     toTitle,
		})
	}

	if err := config.SaveAliasConfig(s.configPath, s.cfg); err != nil {
		return err
	}

	s.rebuildLookup()
	return nil
}

// RemoveTrackAlias menghapus aturan alias berdasarkan nomor indeks 1-based (sesuai tampilan CLI).
func (s *AliasService) RemoveTrackAlias(index int) (*model.TrackAlias, error) {
	if index < 1 || index > len(s.cfg.Tracks) {
		return nil, fmt.Errorf("indeks %d tidak valid (pilih antara 1 sampai %d)", index, len(s.cfg.Tracks))
	}

	removed := s.cfg.Tracks[index-1]
	s.cfg.Tracks = append(s.cfg.Tracks[:index-1], s.cfg.Tracks[index:]...)

	if err := config.SaveAliasConfig(s.configPath, s.cfg); err != nil {
		return nil, err
	}

	s.rebuildLookup()
	return &removed, nil
}

// ApplyToDatabase memperbarui data historis di tabel scrobble_logs sesuai aturan aliases.json.
// Mengembalikan jumlah total baris scrobble yang berhasil dinormalisasi.
// func (s *AliasService) ApplyToDatabase(ctx context.Context) (int64, error) {
// 	if s.db == nil {
// 		return 0, fmt.Errorf("koneksi database tidak tersedia")
// 	}

// 	if len(s.cfg.Tracks) == 0 {
// 		return 0, nil
// 	}

// 	tx, err := s.db.BeginTx(ctx, nil)
// 	if err != nil {
// 		return 0, fmt.Errorf("gagal memulai transaksi database: %w", err)
// 	}
// 	defer tx.Rollback()

// 	const query = `
// 		UPDATE scrobble_logs
// 		SET raw_title = ?
// 		WHERE LOWER(TRIM(raw_artist)) = LOWER(TRIM(?))
// 		  AND LOWER(TRIM(raw_title)) = LOWER(TRIM(?))
// 		  AND raw_title != ?
// 	`

// 	var totalUpdated int64
// 	for _, rule := range s.cfg.Tracks {
// 		target := s.ResolveTrackTitle(rule.Artist, rule.From)
// 		res, err := tx.ExecContext(ctx, query, target, rule.Artist, rule.From, target)
// 		if err != nil {
// 			return 0, fmt.Errorf("gagal menerapkan alias %q -> %q: %w", rule.From, target, err)
// 		}

// 		affected, err := res.RowsAffected()
// 		if err == nil {
// 			totalUpdated += affected
// 		}
// 	}

// 	if err := tx.Commit(); err != nil {
// 		return 0, fmt.Errorf("gagal commit perubahan alias ke database: %w", err)
// 	}

// 	return totalUpdated, nil
// }
