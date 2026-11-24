package replica

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/mbagusaditya/scrobbles-cli/internal/model"
)

const inClauseChunkSize = 200

// Repository adalah base struct untuk seluruh operasi pembacaan dan penulisan batch SQL
// pada satu instance database (baik SQLite lokal maupun Turso Cloud).
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// GetSummary mengambil ringkasan jumlah entitas dan timestamp scrobble terakhir.
func (r *Repository) GetSummary(ctx context.Context) (model.DBStatsSummary, error) {
	var sum model.DBStatsSummary

	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM artists`).Scan(&sum.ArtistsCount); err != nil {
		return sum, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tracks`).Scan(&sum.TracksCount); err != nil {
		return sum, err
	}

	var maxPlayed sql.NullInt64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*), MAX(played_at) FROM scrobble_logs`).Scan(&sum.LogsCount, &maxPlayed); err != nil {
		return sum, err
	}
	if maxPlayed.Valid && maxPlayed.Int64 > 0 {
		sum.LatestPlayed = time.Unix(maxPlayed.Int64, 0).Local()
	}

	return sum, nil
}

// GetMaxPlayedAt mengembalikan unix timestamp scrobble terbaru di database (0 jika kosong).
func (r *Repository) GetMaxPlayedAt(ctx context.Context) (int64, error) {
	var maxPlayed sql.NullInt64
	if err := r.db.QueryRowContext(ctx, `SELECT MAX(played_at) FROM scrobble_logs`).Scan(&maxPlayed); err != nil {
		return 0, err
	}
	if !maxPlayed.Valid {
		return 0, nil
	}
	return maxPlayed.Int64, nil
}

func toNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("?,", n-1) + "?"
}

func toAnySlice(items []string) []any {
	res := make([]any, len(items))
	for i, v := range items {
		res[i] = v
	}
	return res
}
