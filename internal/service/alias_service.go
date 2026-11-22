package service

import (
	"fmt"
	"strings"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
	"github.com/mbagusaditya/scrobbles-cli/internal/repository"
)

const maxAliasResolutionHops = 5

type aliasIndex map[string]string

func newAliasIndex(tracks []model.TrackAlias) aliasIndex {
	idx := make(aliasIndex, len(tracks))
	for _, item := range tracks {
		idx[aliasKey(item.Artist, item.From)] = strings.TrimSpace(item.To)
	}
	return idx
}

func aliasKey(artist, title string) string {
	return strings.ToLower(strings.TrimSpace(artist)) + "\x00" + strings.ToLower(strings.TrimSpace(title))
}

func (idx aliasIndex) resolve(artist, title string) string {
	cleanTitle := strings.TrimSpace(title)
	if len(idx) == 0 {
		return cleanTitle
	}

	current := cleanTitle
	for range maxAliasResolutionHops {
		next, found := idx[aliasKey(artist, current)]
		if !found || strings.EqualFold(next, current) {
			break
		}
		current = next
	}

	return current
}

// AliasService mengelola aturan normalisasi judul lagu dan pencarian kanonik in-memory.
type AliasService struct {
	repo  repository.AliasRepository
	cfg   *model.AliasConfig
	index aliasIndex
}

func NewAliasService(repo repository.AliasRepository) (*AliasService, error) {
	if repo == nil {
		return nil, fmt.Errorf("alias repository tidak boleh nil")
	}

	svc := &AliasService{
		repo:  repo,
		index: make(aliasIndex),
	}

	if err := svc.Reload(); err != nil {
		return nil, err
	}

	return svc, nil
}

func (s *AliasService) ConfigPath() string {
	return s.repo.Location()
}

func (s *AliasService) Reload() error {
	cfg, err := s.repo.Load()
	if err != nil {
		return err
	}

	s.cfg = cfg
	s.index = newAliasIndex(cfg.Tracks)
	return nil
}

// ResolveTrackTitle mengembalikan judul kanonik untuk pasangan (artist, title).
func (s *AliasService) ResolveTrackTitle(artist, title string) string {
	if s == nil {
		return strings.TrimSpace(title)
	}
	return s.index.resolve(artist, title)
}

// ExpandTrackTitles mengembalikan judul kanonik beserta seluruh varian judul alias
// yang mengarah ke judul kanonik tersebut (dipakai untuk agregasi info track).
func (s *AliasService) ExpandTrackTitles(artist, title string) (canonical string, variants []string) {
	canonical = s.ResolveTrackTitle(artist, title)
	if canonical == "" {
		return "", nil
	}

	seen := map[string]struct{}{
		strings.ToLower(canonical): {},
	}
	variants = []string{canonical}

	if s == nil || s.cfg == nil {
		return canonical, variants
	}

	cleanArtist := strings.TrimSpace(artist)
	for _, rule := range s.cfg.Tracks {
		if cleanArtist != "" && !strings.EqualFold(strings.TrimSpace(rule.Artist), cleanArtist) {
			continue
		}
		if strings.EqualFold(s.index.resolve(rule.Artist, rule.From), canonical) {
			fromClean := strings.TrimSpace(rule.From)
			lowerFrom := strings.ToLower(fromClean)
			if _, exists := seen[lowerFrom]; !exists && fromClean != "" {
				seen[lowerFrom] = struct{}{}
				variants = append(variants, fromClean)
			}
		}
	}

	return canonical, variants
}

func (s *AliasService) ListTrackAliases() []model.TrackAlias {
	if s.cfg == nil {
		return nil
	}
	res := make([]model.TrackAlias, len(s.cfg.Tracks))
	copy(res, s.cfg.Tracks)
	return res
}

func (s *AliasService) AddTrackAlias(artist, fromTitle, toTitle string) error {
	artist = strings.TrimSpace(artist)
	fromTitle = strings.TrimSpace(fromTitle)
	toTitle = strings.TrimSpace(toTitle)

	if err := s.validateRule(artist, fromTitle, toTitle); err != nil {
		return err
	}

	s.upsertRule(artist, fromTitle, toTitle)

	if err := s.repo.Save(s.cfg); err != nil {
		return err
	}

	s.index = newAliasIndex(s.cfg.Tracks)
	return nil
}

func (s *AliasService) RemoveTrackAlias(index int) (*model.TrackAlias, error) {
	if index < 1 || index > len(s.cfg.Tracks) {
		return nil, fmt.Errorf("indeks %d tidak valid (pilih antara 1 sampai %d)", index, len(s.cfg.Tracks))
	}

	removed := s.cfg.Tracks[index-1]
	s.cfg.Tracks = append(s.cfg.Tracks[:index-1], s.cfg.Tracks[index:]...)

	if err := s.repo.Save(s.cfg); err != nil {
		return nil, err
	}

	s.index = newAliasIndex(s.cfg.Tracks)
	return &removed, nil
}

func (s *AliasService) validateRule(artist, fromTitle, toTitle string) error {
	if artist == "" || fromTitle == "" || toTitle == "" {
		return fmt.Errorf("artist, judul asal (--from), dan judul target (--to) tidak boleh kosong")
	}

	if strings.EqualFold(fromTitle, toTitle) {
		return fmt.Errorf("judul asal dan judul target tidak boleh sama (%q)", fromTitle)
	}

	resolvedTarget := s.index.resolve(artist, toTitle)
	if strings.EqualFold(resolvedTarget, fromTitle) {
		return fmt.Errorf("terdeteksi circular alias: %q sudah mengarah kembali ke %q", toTitle, fromTitle)
	}

	return nil
}

func (s *AliasService) upsertRule(artist, fromTitle, toTitle string) {
	for i, existing := range s.cfg.Tracks {
		if strings.EqualFold(strings.TrimSpace(existing.Artist), artist) &&
			strings.EqualFold(strings.TrimSpace(existing.From), fromTitle) {
			s.cfg.Tracks[i].Artist = artist
			s.cfg.Tracks[i].From = fromTitle
			s.cfg.Tracks[i].To = toTitle
			return
		}
	}

	s.cfg.Tracks = append(s.cfg.Tracks, model.TrackAlias{
		Artist: artist,
		From:   fromTitle,
		To:     toTitle,
	})
}
