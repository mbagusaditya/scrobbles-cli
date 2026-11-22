package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mbagusaditya/scrobbles-cli/internal/lastfm"
	"github.com/mbagusaditya/scrobbles-cli/internal/model"
	"github.com/mbagusaditya/scrobbles-cli/internal/repository"
)

// InfoService menggabungkan metadata publik dari Last.fm dengan statistik riwayat personal.
type InfoService struct {
	repo   repository.InfoRepository
	client *lastfm.Client
	alias  *AliasService
}

func NewInfoService(repo repository.InfoRepository, client *lastfm.Client, alias *AliasService) *InfoService {
	return &InfoService{
		repo:   repo,
		client: client,
		alias:  alias,
	}
}

// GetArtistInfo mengambil detail artist dari repository lokal dan Last.fm API.
func (s *InfoService) GetArtistInfo(ctx context.Context, filter model.InfoFilter) (*model.EntityInfoResult, error) {
	artistName := strings.TrimSpace(filter.Query)
	if artistName == "" {
		return nil, fmt.Errorf("nama artis tidak boleh kosong")
	}

	stat, err := s.repo.GetArtistPlayStats(ctx, artistName, filter.From, filter.To)
	if err != nil {
		return nil, err
	}

	res := buildBaseInfoResult("Artist", artistName, "", stat)

	if apiResp, err := s.client.GetArtistInfo(ctx, artistName); err == nil && apiResp != nil {
		applyArtistAPIInfo(res, apiResp)
	}

	return res, nil
}

// GetTrackInfo mengambil detail track, mendukung resolusi alias dan deteksi artis otomatis.
func (s *InfoService) GetTrackInfo(ctx context.Context, filter model.InfoFilter) (*model.EntityInfoResult, error) {
	rawQuery := strings.TrimSpace(filter.Query)
	artistName := strings.TrimSpace(filter.Artist)

	if rawQuery == "" {
		return nil, fmt.Errorf("nama track tidak boleh kosong")
	}

	canonicalTitle, titleVariants := s.expandTrackVariants(artistName, rawQuery)

	if artistName == "" {
		candidates, err := s.repo.FindArtistsByTrackTitles(ctx, titleVariants)
		if err != nil {
			return nil, err
		}
		detected, err := pickSingleArtist("lagu", canonicalTitle, candidates)
		if err != nil {
			return nil, err
		}
		artistName = detected

		// Perbarui varian setelah artis teridentifikasi spesifik
		if artistName != "" {
			canonicalTitle, titleVariants = s.expandTrackVariants(artistName, rawQuery)
		}
	}

	stat, err := s.repo.GetTrackPlayStats(ctx, artistName, titleVariants, filter.From, filter.To)
	if err != nil {
		return nil, err
	}

	res := buildBaseInfoResult("Track", canonicalTitle, artistName, stat)

	if artistName != "" {
		if apiResp, err := s.client.GetTrackInfo(ctx, artistName, canonicalTitle); err == nil && apiResp != nil {
			applyTrackAPIInfo(res, apiResp)
		}
	}

	return res, nil
}

// GetAlbumInfo mengambil detail album beserta statistik putar personalnya.
func (s *InfoService) GetAlbumInfo(ctx context.Context, filter model.InfoFilter) (*model.EntityInfoResult, error) {
	albumName := strings.TrimSpace(filter.Query)
	artistName := strings.TrimSpace(filter.Artist)

	if albumName == "" {
		return nil, fmt.Errorf("nama album tidak boleh kosong")
	}

	if artistName == "" {
		candidates, err := s.repo.FindArtistsByAlbum(ctx, albumName)
		if err != nil {
			return nil, err
		}
		detected, err := pickSingleArtist("album", albumName, candidates)
		if err != nil {
			return nil, err
		}
		artistName = detected
	}

	stat, err := s.repo.GetAlbumPlayStats(ctx, artistName, albumName, filter.From, filter.To)
	if err != nil {
		return nil, err
	}

	res := buildBaseInfoResult("Album", albumName, artistName, stat)

	if artistName != "" {
		if apiResp, err := s.client.GetAlbumInfo(ctx, artistName, albumName); err == nil && apiResp != nil {
			applyAlbumAPIInfo(res, apiResp)
		}
	}

	return res, nil
}

func (s *InfoService) expandTrackVariants(artist, title string) (string, []string) {
	if s.alias != nil {
		return s.alias.ExpandTrackTitles(artist, title)
	}
	clean := strings.TrimSpace(title)
	return clean, []string{clean}
}

func pickSingleArtist(entityLabel, entityName string, candidates []string) (string, error) {
	if len(candidates) == 0 {
		return "", nil
	}
	if len(candidates) > 1 {
		return "", fmt.Errorf("ditemukan %d artis berbeda untuk %s %q (%s). Mohon spesifikasikan artis dengan flag --artist",
			len(candidates), entityLabel, entityName, strings.Join(candidates, ", "))
	}
	return candidates[0], nil
}

func buildBaseInfoResult(entityType, name, artist string, stat *repository.EntityPlayStats) *model.EntityInfoResult {
	var shareRatio float64
	if stat.TotalLibrary > 0 {
		shareRatio = (float64(stat.UserPlays) / float64(stat.TotalLibrary)) * 100.0
	}

	return &model.EntityInfoResult{
		Type:          entityType,
		Name:          name,
		Artist:        artist,
		UserPlaycount: stat.UserPlays,
		TotalLibrary:  stat.TotalLibrary,
		ShareRatio:    shareRatio,
		FirstHeard:    stat.FirstHeard,
		LastHeard:     stat.LastHeard,
	}
}

func applyArtistAPIInfo(res *model.EntityInfoResult, apiResp *lastfm.ArtistInfoResponse) {
	res.GlobalListeners, _ = strconv.Atoi(apiResp.Artist.Stats.Listeners)
	res.GlobalPlaycount, _ = strconv.Atoi(apiResp.Artist.Stats.Playcount)
	for _, tag := range apiResp.Artist.Tags.Tag {
		res.Tags = append(res.Tags, tag.Name)
	}
	res.BioOrSummary = cleanWikiSummary(apiResp.Artist.Bio.Summary)
}

func applyTrackAPIInfo(res *model.EntityInfoResult, apiResp *lastfm.TrackInfoResponse) {
	res.GlobalListeners, _ = strconv.Atoi(apiResp.Track.Listeners)
	res.GlobalPlaycount, _ = strconv.Atoi(apiResp.Track.Playcount)
	durMs, _ := strconv.Atoi(apiResp.Track.Duration)
	res.DurationSec = durMs / 1000
	res.Album = apiResp.Track.Album.Title

	for _, tag := range apiResp.Track.TopTags.Tag {
		res.Tags = append(res.Tags, tag.Name)
	}
	res.BioOrSummary = cleanWikiSummary(apiResp.Track.Wiki.Summary)
}

func applyAlbumAPIInfo(res *model.EntityInfoResult, apiResp *lastfm.AlbumInfoResponse) {
	res.GlobalListeners, _ = strconv.Atoi(apiResp.Album.Listeners)
	res.GlobalPlaycount, _ = strconv.Atoi(apiResp.Album.Playcount)
	for _, tag := range apiResp.Album.Tags.Tag {
		res.Tags = append(res.Tags, tag.Name)
	}
	res.BioOrSummary = cleanWikiSummary(apiResp.Album.Wiki.Summary)
}

func cleanWikiSummary(summary string) string {
	if idx := strings.Index(summary, "<a href"); idx != -1 {
		summary = summary[:idx]
	}
	return strings.TrimSpace(summary)
}
