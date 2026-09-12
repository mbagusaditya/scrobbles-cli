package model

import "database/sql"

// toNullString mengubah string kosong "" menjadi sql.NullString{Valid: false},
// dan string non-kosong menjadi sql.NullString{Valid: true}.
// Dipakai di semua constructor entity untuk konsistensi handling field nullable.
func toNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
