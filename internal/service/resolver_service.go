package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ResolveProgress struct {
	BatchProcessed int
	Remaining      int
	NewArtists     int
	NewTracks      int
	UpdatedLogs    int
}

type ResolveProgressFunc func(p ResolveProgress)

type ResolveResult struct {
	TotalUpdatedLogs int
	TotalNewArtists  int
	TotalNewTracks   int
}

type unresolvedItem struct {
	RawArtist string
	RawTitle  string
	RawAlbum  string
}

// ResolveUnlinkedScrobbles mengambil scrobble yang track_id-nya masih NULL,
// memastikan entitas artist dan track ada di database, lalu mengupdate
// track_id pada scrobble_logs. Diproses per batch agar hemat memori dan transaksi aman.
func (s *ScrobbleService) ResolveUnlinkedScrobbles(ctx context.Context, batchSize int, onProgress ResolveProgressFunc) (*ResolveResult, error) {
	if batchSize <= 0 {
		batchSize = 500
	}

	result := &ResolveResult{}

	for {
		// 1. Cek berapa banyak scrobble yang belum ter-resolve
		var remainingCount int
		err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM scrobble_logs WHERE track_id IS NULL").Scan(&remainingCount)
		if err != nil {
			return result, fmt.Errorf("gagal menghitung sisa scrobble unlinked: %w", err)
		}

		if remainingCount == 0 {
			break
		}

		// 2. Ambil kombinasi unik (artist, title, album) dari baris yang track_id IS NULL
		// Memanfaatkan partial index: idx_scrobble_logs_unresolved
		query := `
					SELECT DISTINCT raw_artist, raw_title, COALESCE(raw_album, '') AS raw_album
					FROM scrobble_logs
					WHERE track_id IS NULL
					LIMIT ?
				`
		rows, err := s.db.QueryContext(ctx, query, batchSize)
		if err != nil {
			return result, fmt.Errorf("gagal query unresolved scrobbles: %w", err)
		}

		var items []unresolvedItem
		for rows.Next() {
			var it unresolvedItem
			if err := rows.Scan(&it.RawArtist, &it.RawTitle, &it.RawAlbum); err != nil {
				rows.Close()
				return result, fmt.Errorf("gagal scan baris unresolved: %w", err)
			}
			items = append(items, it)
		}

		// Periksa error yang mungkin terjadi selama iterasi rows.Next()
		if err := rows.Err(); err != nil {
			rows.Close()
			return result, fmt.Errorf("error saat membaca baris unresolved scrobbles: %w", err)
		}
		rows.Close()

		if len(items) == 0 {
			break
		}

		// Eksekusi pemetaan dalam satu transaksi per batch
		newArtists, newTracks, updatedLogs, err := s.processBatch(ctx, items)
		if err != nil {
			return result, fmt.Errorf("gagal memproses batch resolusi: %w", err)
		}

		result.TotalNewArtists += newArtists
		result.TotalNewTracks += newTracks
		result.TotalUpdatedLogs += updatedLogs

		remainingAfter := remainingCount - updatedLogs
		if remainingAfter < 0 {
			remainingAfter = 0
		}

		if onProgress != nil {
			onProgress(ResolveProgress{
				BatchProcessed: len(items),
				Remaining:      remainingAfter,
				NewArtists:     newArtists,
				NewTracks:      newTracks,
				UpdatedLogs:    updatedLogs,
			})
		}
	}

	return result, nil
}

func (s *ScrobbleService) processBatch(ctx context.Context, items []unresolvedItem) (int, int, int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, 0, err
	}
	defer tx.Rollback()

	now := time.Now().Unix()

	// A. Ekstrak artis unik dan pastikan ada di tabel artists
	artistMap := make(map[string]string) // name -> id
	var uniqueArtists []string
	for _, it := range items {
		if _, exists := artistMap[it.RawArtist]; !exists {
			artistMap[it.RawArtist] = ""
			uniqueArtists = append(uniqueArtists, it.RawArtist)
		}
	}

	newArtistsCount := 0
	insertArtistStmt, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO artists (id, name, created_at)
		VALUES (?, ?, ?)
	`)
	if err != nil {
		return 0, 0, 0, err
	}
	defer insertArtistStmt.Close()

	for _, artistName := range uniqueArtists {
		newID := uuid.NewString()
		res, err := insertArtistStmt.ExecContext(ctx, newID, artistName, now)
		if err != nil {
			return 0, 0, 0, err
		}
		rowsAffected, _ := res.RowsAffected()
		if rowsAffected > 0 {
			newArtistsCount++
		}
	}

	// Ambil ID artis yang valid dari database untuk semua artis dalam batch
	for _, artistName := range uniqueArtists {
		var id string
		err := tx.QueryRowContext(ctx, "SELECT id FROM artists WHERE name = ?", artistName).Scan(&id)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("gagal query id artis '%s': %w", artistName, err)
		}
		artistMap[artistName] = id
	}

	// B. Pastikan tracks ada di tabel tracks
	// Key komposit track: artistID + "|" + title + "|" + album
	trackMap := make(map[string]string)
	newTracksCount := 0

	insertTrackStmt, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO tracks (id, artist_id, title, album_name, created_at)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return 0, 0, 0, err
	}
	defer insertTrackStmt.Close()

	for _, it := range items {
		artistID := artistMap[it.RawArtist]
		trackKey := makeTrackKey(artistID, it.RawTitle, it.RawAlbum)

		if _, exists := trackMap[trackKey]; !exists {
			trackMap[trackKey] = ""

			newTrackID := uuid.NewString()
			var albumVal sql.NullString
			if strings.TrimSpace(it.RawAlbum) != "" {
				albumVal = sql.NullString{String: it.RawAlbum, Valid: true}
			}

			res, err := insertTrackStmt.ExecContext(ctx, newTrackID, artistID, it.RawTitle, albumVal, now)
			if err != nil {
				return 0, 0, 0, err
			}
			rowsAffected, _ := res.RowsAffected()
			if rowsAffected > 0 {
				newTracksCount++
			}
		}
	}

	// Ambil track ID yang valid dari database
	for trackKey := range trackMap {
		artistID, title, album := parseTrackKey(trackKey)

		var trackID string
		var query string
		var args []any

		if album == "" {
			query = "SELECT id FROM tracks WHERE artist_id = ? AND title = ? AND album_name IS NULL"
			args = []any{artistID, title}
		} else {
			query = "SELECT id FROM tracks WHERE artist_id = ? AND title = ? AND album_name = ?"
			args = []any{artistID, title, album}
		}

		err := tx.QueryRowContext(ctx, query, args...).Scan(&trackID)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("gagal query id track '%s': %w", title, err)
		}
		trackMap[trackKey] = trackID
	}

	// C. Update track_id pada scrobble_logs
	updateLogStmt, err := tx.PrepareContext(ctx, `
		UPDATE scrobble_logs
		SET track_id = ?
		WHERE raw_artist = ?
		  AND raw_title = ?
		  AND (raw_album = ? OR (raw_album IS NULL AND ? = ''))
		  AND track_id IS NULL
	`)
	if err != nil {
		return 0, 0, 0, err
	}
	defer updateLogStmt.Close()

	totalUpdatedLogs := 0
	for _, it := range items {
		artistID := artistMap[it.RawArtist]
		trackKey := makeTrackKey(artistID, it.RawTitle, it.RawAlbum)
		trackID := trackMap[trackKey]

		res, err := updateLogStmt.ExecContext(ctx, trackID, it.RawArtist, it.RawTitle, it.RawAlbum, it.RawAlbum)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("gagal update track_id di scrobble_logs: %w", err)
		}

		rowsAffected, _ := res.RowsAffected()
		totalUpdatedLogs += int(rowsAffected)
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, 0, err
	}

	return newArtistsCount, newTracksCount, totalUpdatedLogs, nil
}

func makeTrackKey(artistID, title, album string) string {
	return fmt.Sprintf("%s\x00%s\x00%s", artistID, title, album)
}

func parseTrackKey(key string) (string, string, string) {
	parts := strings.Split(key, "\x00")
	if len(parts) == 3 {
		return parts[0], parts[1], parts[2]
	}
	return "", "", ""
}
