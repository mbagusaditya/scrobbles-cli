package service

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
)

type InfoFilter struct {
	Query  string
	Artist string // Opsional
	From   time.Time
	To     time.Time
}

// GetArtistInfo mengambil detail artist dari DB lokal dan Last.fm API.
func (s *ScrobbleService) GetArtistInfo(ctx context.Context, filter InfoFilter) (*model.EntityInfoResult, error) {
	artistName := strings.TrimSpace(filter.Query)
	if artistName == "" {
		return nil, fmt.Errorf("nama artis tidak boleh kosong")
	}

	// 1. Hitung statistik personal dari DB lokal
	stat, err := s.calcLocalStats(ctx, "raw_artist = ?", []any{artistName}, filter.From, filter.To)
	if err != nil {
		return nil, err
	}

	res := &model.EntityInfoResult{
		Type:          "Artist",
		Name:          artistName,
		UserPlaycount: stat.UserPlays,
		TotalLibrary:  stat.TotalLibrary,
		ShareRatio:    stat.ShareRatio,
		FirstHeard:    stat.FirstHeard,
		LastHeard:     stat.LastHeard,
	}

	// 2. Fetch metadata global dari Last.fm API
	// Menggunakan method client Last.fm
	apiResp, err := s.client.GetArtistInfo(ctx, artistName)
	if err == nil && apiResp != nil {
		listeners, _ := strconv.Atoi(apiResp.Artist.Stats.Listeners)
		playcount, _ := strconv.Atoi(apiResp.Artist.Stats.Playcount)
		res.GlobalListeners = listeners
		res.GlobalPlaycount = playcount

		for _, tag := range apiResp.Artist.Tags.Tag {
			res.Tags = append(res.Tags, tag.Name)
		}
		res.BioOrSummary = cleanWikiSummary(apiResp.Artist.Bio.Summary)
	}

	return res, nil
}

// GetTrackInfo mengambil detail track, otomatis menebak artis jika tidak diisi.
func (s *ScrobbleService) GetTrackInfo(ctx context.Context, filter InfoFilter) (*model.EntityInfoResult, error) {
	trackName := strings.TrimSpace(filter.Query)
	artistName := strings.TrimSpace(filter.Artist)

	if trackName == "" {
		return nil, fmt.Errorf("nama track tidak boleh kosong")
	}

	// Jika artis tidak diisi, coba tebak dari database lokal
	if artistName == "" {
		detectedArtist, err := s.resolveSingleArtistForTrack(ctx, trackName)
		if err != nil {
			return nil, err
		}
		artistName = detectedArtist
	}

	var whereConditions []string
	var args []any

	whereConditions = append(whereConditions, "raw_title = ? COLLATE NOCASE")
	args = append(args, trackName)

	if artistName != "" {
		whereConditions = append(whereConditions, "raw_artist = ? COLLATE NOCASE")
		args = append(args, artistName)
	}

	stat, err := s.calcLocalStats(ctx, strings.Join(whereConditions, " AND "), args, filter.From, filter.To)
	if err != nil {
		return nil, err
	}

	res := &model.EntityInfoResult{
		Type:          "Track",
		Name:          trackName,
		Artist:        artistName,
		UserPlaycount: stat.UserPlays,
		TotalLibrary:  stat.TotalLibrary,
		ShareRatio:    stat.ShareRatio,
		FirstHeard:    stat.FirstHeard,
		LastHeard:     stat.LastHeard,
	}

	// Fetch metadata Last.fm jika artis berhasil diidentifikasi
	if artistName != "" {
		apiResp, err := s.client.GetTrackInfo(ctx, artistName, trackName)
		if err == nil && apiResp != nil {
			listeners, _ := strconv.Atoi(apiResp.Track.Listeners)
			playcount, _ := strconv.Atoi(apiResp.Track.Playcount)
			durMs, _ := strconv.Atoi(apiResp.Track.Duration)

			res.GlobalListeners = listeners
			res.GlobalPlaycount = playcount
			res.DurationSec = durMs / 1000
			res.Album = apiResp.Track.Album.Title

			for _, tag := range apiResp.Track.TopTags.Tag {
				res.Tags = append(res.Tags, tag.Name)
			}
			res.BioOrSummary = cleanWikiSummary(apiResp.Track.Wiki.Summary)
		}
	}

	return res, nil
}

// GetAlbumInfo mengambil detail album beserta tracklist/deskripsinya.
func (s *ScrobbleService) GetAlbumInfo(ctx context.Context, filter InfoFilter) (*model.EntityInfoResult, error) {
	albumName := strings.TrimSpace(filter.Query)
	artistName := strings.TrimSpace(filter.Artist)

	if albumName == "" {
		return nil, fmt.Errorf("nama album tidak boleh kosong")
	}

	if artistName == "" {
		detectedArtist, err := s.resolveSingleArtistForAlbum(ctx, albumName)
		if err != nil {
			return nil, err
		}
		artistName = detectedArtist
	}

	var whereConditions []string
	var args []any

	whereConditions = append(whereConditions, "raw_album = ? COLLATE NOCASE")
	args = append(args, albumName)

	if artistName != "" {
		whereConditions = append(whereConditions, "raw_artist = ? COLLATE NOCASE")
		args = append(args, artistName)
	}

	stat, err := s.calcLocalStats(ctx, strings.Join(whereConditions, " AND "), args, filter.From, filter.To)
	if err != nil {
		return nil, err
	}

	res := &model.EntityInfoResult{
		Type:          "Album",
		Name:          albumName,
		Artist:        artistName,
		UserPlaycount: stat.UserPlays,
		TotalLibrary:  stat.TotalLibrary,
		ShareRatio:    stat.ShareRatio,
		FirstHeard:    stat.FirstHeard,
		LastHeard:     stat.LastHeard,
	}

	if artistName != "" {
		apiResp, err := s.client.GetAlbumInfo(ctx, artistName, albumName)
		if err == nil && apiResp != nil {
			listeners, _ := strconv.Atoi(apiResp.Album.Listeners)
			playcount, _ := strconv.Atoi(apiResp.Album.Playcount)
			res.GlobalListeners = listeners
			res.GlobalPlaycount = playcount

			for _, tag := range apiResp.Album.Tags.Tag {
				res.Tags = append(res.Tags, tag.Name)
			}
			res.BioOrSummary = cleanWikiSummary(apiResp.Album.Wiki.Summary)
		}
	}

	return res, nil
}

// Helper: Menghitung playcount entitas dan perbandingannya dengan total scrobble lokal
type localStat struct {
	UserPlays    int
	TotalLibrary int
	ShareRatio   float64
	FirstHeard   time.Time
	LastHeard    time.Time
}

func (s *ScrobbleService) calcLocalStats(ctx context.Context, entityClause string, args []any, from, to time.Time) (*localStat, error) {
	// Query total scrobble di pustaka pada rentang tanggal yang sama
	var timeConds []string
	var timeArgs []any

	if !from.IsZero() {
		timeConds = append(timeConds, "played_at >= ?")
		timeArgs = append(timeArgs, from.Unix())
	}
	if !to.IsZero() {
		timeConds = append(timeConds, "played_at <= ?")
		timeArgs = append(timeArgs, to.Unix())
	}

	totalWhere := ""
	if len(timeConds) > 0 {
		totalWhere = "WHERE " + strings.Join(timeConds, " AND ")
	}

	var totalLib int
	countLibQuery := fmt.Sprintf("SELECT COUNT(*) FROM scrobble_logs %s", totalWhere)
	if err := s.db.QueryRowContext(ctx, countLibQuery, timeArgs...).Scan(&totalLib); err != nil {
		return nil, fmt.Errorf("gagal menghitung total scrobbles pustaka: %w", err)
	}

	// Query data spesifik entitas
	fullWhere := "WHERE " + entityClause
	fullArgs := append([]any{}, args...)
	if len(timeConds) > 0 {
		fullWhere += " AND " + strings.Join(timeConds, " AND ")
		fullArgs = append(fullArgs, timeArgs...)
	}

	query := fmt.Sprintf(`
		SELECT COUNT(*), MIN(played_at), MAX(played_at)
		FROM scrobble_logs
		%s
	`, fullWhere)

	var userPlays int
	var minPlayed, maxPlayed sql.NullInt64
	if err := s.db.QueryRowContext(ctx, query, fullArgs...).Scan(&userPlays, &minPlayed, &maxPlayed); err != nil {
		return nil, fmt.Errorf("gagal query statistik entitas: %w", err)
	}

	res := &localStat{
		UserPlays:    userPlays,
		TotalLibrary: totalLib,
	}

	if totalLib > 0 {
		res.ShareRatio = (float64(userPlays) / float64(totalLib)) * 100.0
	}

	if minPlayed.Valid {
		res.FirstHeard = time.Unix(minPlayed.Int64, 0).Local()
	}
	if maxPlayed.Valid {
		res.LastHeard = time.Unix(maxPlayed.Int64, 0).Local()
	}

	return res, nil
}

// resolveSingleArtistForTrack mendeteksi artis jika tidak diisi user
func (s *ScrobbleService) resolveSingleArtistForTrack(ctx context.Context, title string) (string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT DISTINCT raw_artist FROM scrobble_logs WHERE raw_title = ? COLLATE NOCASE", title)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var artists []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err == nil {
			artists = append(artists, a)
		}
	}

	if len(artists) == 0 {
		return "", nil // Tidak ditemukan di DB lokal, biarkan pencarian Last.fm mencari apa adanya
	}
	if len(artists) > 1 {
		return "", fmt.Errorf("ditemukan %d artis berbeda untuk lagu %q (%s). Mohon spesifikasikan artis dengan flag --artist",
			len(artists), title, strings.Join(artists, ", "))
	}

	return artists[0], nil
}

// resolveSingleArtistForAlbum mendeteksi artis album jika tidak diisi user
func (s *ScrobbleService) resolveSingleArtistForAlbum(ctx context.Context, album string) (string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT DISTINCT raw_artist FROM scrobble_logs WHERE raw_album = ? COLLATE NOCASE", album)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var artists []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err == nil {
			artists = append(artists, a)
		}
	}

	if len(artists) == 0 {
		return "", nil
	}
	if len(artists) > 1 {
		return "", fmt.Errorf("ditemukan %d artis berbeda untuk album %q. Mohon spesifikasikan artis dengan flag --artist", len(artists), album)
	}

	return artists[0], nil
}

func cleanWikiSummary(summary string) string {
	// Menghapus tag tautan Last.fm default '<a href="...">Read more on Last.fm</a>'
	if idx := strings.Index(summary, "<a href"); idx != -1 {
		summary = summary[:idx]
	}
	return strings.TrimSpace(summary)
}
