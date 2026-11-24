package db

import (
	"database/sql"
	"fmt"
)

const schemaDDL = `
CREATE TABLE IF NOT EXISTS artists (
	id text PRIMARY KEY NOT NULL,
	name text NOT NULL,
	mbid text,
	image_url text,
	created_at integer NOT NULL
);

CREATE TABLE IF NOT EXISTS tracks (
	id text PRIMARY KEY NOT NULL,
	artist_id text NOT NULL,
	title text NOT NULL,
	album_name text,
	duration_ms integer,
	created_at integer NOT NULL,
	cover_url text,
	CONSTRAINT fk_tracks_artist_id_artists_id_fk FOREIGN KEY (artist_id) REFERENCES artists(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS scrobble_logs (
	id text PRIMARY KEY NOT NULL,
	track_id text,
	raw_title text NOT NULL,
	raw_artist text NOT NULL,
	raw_album text,
	played_at integer NOT NULL,
	created_at integer NOT NULL,
	CONSTRAINT fk_scrobble_logs_track_id_tracks_id_fk FOREIGN KEY (track_id) REFERENCES tracks(id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_artists_name ON artists (name);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tracks_artist_title_album ON tracks (artist_id, title, album_name);
CREATE INDEX IF NOT EXISTS idx_tracks_artist_id ON tracks (artist_id);
CREATE INDEX IF NOT EXISTS idx_scrobble_logs_valid_albums ON scrobble_logs (raw_album);
CREATE INDEX IF NOT EXISTS idx_scrobble_logs_unresolved ON scrobble_logs (raw_artist, raw_title);
CREATE INDEX IF NOT EXISTS idx_scrobble_logs_artist_played ON scrobble_logs (raw_artist, played_at);
CREATE INDEX IF NOT EXISTS idx_scrobble_logs_artist_album ON scrobble_logs (raw_artist, raw_album);
CREATE INDEX IF NOT EXISTS idx_scrobble_logs_artist_title ON scrobble_logs (raw_artist, raw_title);
CREATE INDEX IF NOT EXISTS idx_scrobble_logs_raw_artist ON scrobble_logs (raw_artist);
CREATE INDEX IF NOT EXISTS idx_scrobble_logs_track_id ON scrobble_logs (track_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_scrobble_logs_unique_event ON scrobble_logs (raw_artist, raw_title, played_at);
CREATE INDEX IF NOT EXISTS idx_scrobble_logs_played_at ON scrobble_logs (played_at);
`

func EnsureSchema(database *sql.DB) error {
	if _, err := database.Exec(schemaDDL); err != nil {
		return fmt.Errorf("gagal mengeksekusi DDL skema: %w", err)
	}
	return nil
}
