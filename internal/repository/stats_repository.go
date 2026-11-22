package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// ScrobbleStatRow adalah proyeksi ringan satu baris scrobble untuk kalkulasi statistik.
type ScrobbleStatRow struct {
	RawArtist string
	RawAlbum  string
	RawTitle  string
	PlayedAt  int64 // Unix epoch detik
}

// StatsRepository menyediakan sumber data scrobble teroptimasi untuk kalkulasi statistik.
type StatsRepository interface {
	StreamScrobbleRows(ctx context.Context, from, to time.Time, fn func(row ScrobbleStatRow) error) error
}

var _ StatsRepository = (*sqlStatsRepository)(nil)

type sqlStatsRepository struct {
	db *sql.DB
}

func NewStatsRepository(db *sql.DB) StatsRepository {
	return &sqlStatsRepository{db: db}
}

func (r *sqlStatsRepository) StreamScrobbleRows(ctx context.Context, from, to time.Time, fn func(row ScrobbleStatRow) error) error {
	var conditions []string
	var args []any

	if !from.IsZero() {
		conditions = append(conditions, "played_at >= ?")
		args = append(args, from.Unix())
	}
	if !to.IsZero() {
		conditions = append(conditions, "played_at <= ?")
		args = append(args, to.Unix())
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT raw_artist, COALESCE(raw_album, ''), raw_title, played_at
		FROM scrobble_logs
		%s
		ORDER BY played_at ASC
	`, whereClause)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("gagal query scrobble logs untuk statistik: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var row ScrobbleStatRow
		if err := rows.Scan(&row.RawArtist, &row.RawAlbum, &row.RawTitle, &row.PlayedAt); err != nil {
			return fmt.Errorf("gagal membaca baris statistik: %w", err)
		}
		if err := fn(row); err != nil {
			return err
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterasi baris statistik: %w", err)
	}
	return nil
}
