package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// EntityPlayStats menampung ringkasan statistik putar personal sebuah entitas
// dibandingkan terhadap total scrobble pada rentang waktu yang sama.
type EntityPlayStats struct {
	UserPlays    int
	TotalLibrary int
	FirstHeard   time.Time
	LastHeard    time.Time
}

// InfoRepository mendefinisikan kontrak kueri statistik personal & deteksi artis
// untuk subcommand info (track, artist, album).
type InfoRepository interface {
	GetArtistPlayStats(ctx context.Context, artist string, from, to time.Time) (*EntityPlayStats, error)
	GetTrackPlayStats(ctx context.Context, artist string, titleVariants []string, from, to time.Time) (*EntityPlayStats, error)
	GetAlbumPlayStats(ctx context.Context, artist, album string, from, to time.Time) (*EntityPlayStats, error)
	FindArtistsByTrackTitles(ctx context.Context, titleVariants []string) ([]string, error)
	FindArtistsByAlbum(ctx context.Context, album string) ([]string, error)
}

var _ InfoRepository = (*sqlInfoRepository)(nil)

type sqlInfoRepository struct {
	db *sql.DB
}

// NewInfoRepository membuat implementasi InfoRepository berbasis *sql.DB (SQLite/Turso).
func NewInfoRepository(db *sql.DB) InfoRepository {
	return &sqlInfoRepository{db: db}
}

func (r *sqlInfoRepository) GetArtistPlayStats(ctx context.Context, artist string, from, to time.Time) (*EntityPlayStats, error) {
	return r.queryEntityStats(ctx, "raw_artist = ? COLLATE NOCASE", []any{artist}, from, to)
}

func (r *sqlInfoRepository) GetTrackPlayStats(ctx context.Context, artist string, titleVariants []string, from, to time.Time) (*EntityPlayStats, error) {
	if len(titleVariants) == 0 {
		return &EntityPlayStats{}, nil
	}

	placeholders := make([]string, len(titleVariants))
	args := make([]any, 0, len(titleVariants)+1)
	for i, title := range titleVariants {
		placeholders[i] = "?"
		args = append(args, title)
	}

	conditions := []string{
		fmt.Sprintf("raw_title COLLATE NOCASE IN (%s)", strings.Join(placeholders, ", ")),
	}

	if artist != "" {
		conditions = append(conditions, "raw_artist = ? COLLATE NOCASE")
		args = append(args, artist)
	}

	return r.queryEntityStats(ctx, strings.Join(conditions, " AND "), args, from, to)
}

func (r *sqlInfoRepository) GetAlbumPlayStats(ctx context.Context, artist, album string, from, to time.Time) (*EntityPlayStats, error) {
	conditions := []string{"raw_album = ? COLLATE NOCASE"}
	args := []any{album}

	if artist != "" {
		conditions = append(conditions, "raw_artist = ? COLLATE NOCASE")
		args = append(args, artist)
	}

	return r.queryEntityStats(ctx, strings.Join(conditions, " AND "), args, from, to)
}

// queryEntityStats mengeksekusi 1 kueri agregasi kondisional (CASE WHEN) untuk menghitung
// total pustaka dan statistik entitas sekaligus dalam satu round-trip database.
func (r *sqlInfoRepository) queryEntityStats(ctx context.Context, matchExpr string, matchArgs []any, from, to time.Time) (*EntityPlayStats, error) {
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

	whereClause := ""
	if len(timeConds) > 0 {
		whereClause = "WHERE " + strings.Join(timeConds, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT
			COUNT(*) AS total_lib,
			COALESCE(SUM(CASE WHEN %s THEN 1 ELSE 0 END), 0) AS user_plays,
			MIN(CASE WHEN %s THEN played_at END) AS min_played,
			MAX(CASE WHEN %s THEN played_at END) AS max_played
		FROM scrobble_logs
		%s
	`, matchExpr, matchExpr, matchExpr, whereClause)

	// Susun argumen sesuai urutan kemunculan placeholder (?) di SELECT lalu WHERE
	args := make([]any, 0, len(matchArgs)*3+len(timeArgs))
	args = append(args, matchArgs...)
	args = append(args, matchArgs...)
	args = append(args, matchArgs...)
	args = append(args, timeArgs...)

	var totalLib, userPlays int
	var minPlayed, maxPlayed sql.NullInt64

	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&totalLib, &userPlays, &minPlayed, &maxPlayed); err != nil {
		return nil, fmt.Errorf("gagal query statistik entitas: %w", err)
	}

	stats := &EntityPlayStats{
		UserPlays:    userPlays,
		TotalLibrary: totalLib,
	}
	if minPlayed.Valid {
		stats.FirstHeard = time.Unix(minPlayed.Int64, 0).Local()
	}
	if maxPlayed.Valid {
		stats.LastHeard = time.Unix(maxPlayed.Int64, 0).Local()
	}

	return stats, nil
}

func (r *sqlInfoRepository) FindArtistsByTrackTitles(ctx context.Context, titleVariants []string) ([]string, error) {
	if len(titleVariants) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(titleVariants))
	args := make([]any, len(titleVariants))
	for i, t := range titleVariants {
		placeholders[i] = "?"
		args[i] = t
	}

	query := fmt.Sprintf(`
		SELECT DISTINCT raw_artist
		FROM scrobble_logs
		WHERE raw_title COLLATE NOCASE IN (%s)
	`, strings.Join(placeholders, ", "))

	return r.queryDistinctStrings(ctx, query, args...)
}

func (r *sqlInfoRepository) FindArtistsByAlbum(ctx context.Context, album string) ([]string, error) {
	const query = `
		SELECT DISTINCT raw_artist
		FROM scrobble_logs
		WHERE raw_album = ? COLLATE NOCASE
	`
	return r.queryDistinctStrings(ctx, query, album)
}

func (r *sqlInfoRepository) queryDistinctStrings(ctx context.Context, query string, args ...any) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var val string
		if err := rows.Scan(&val); err != nil {
			return nil, err
		}
		results = append(results, val)
	}

	return results, rows.Err()
}
