package library

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Song struct {
	ID        int64
	Name      string
	Format    string
	Status    string
	Progress  int
	FilePath  string
	SourceURL string
	CreatedAt time.Time
}

type Repository struct {
	db *sql.DB
}

func Open(path string) (*Repository, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create database dir: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// SQLite supports a single writer at a time, so limiting the pool avoids
	// "database is locked" errors during concurrent updates from download jobs.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	repo := &Repository{db: db}
	if err := repo.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return repo, nil
}

func (r *Repository) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

func (r *Repository) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS songs (
id INTEGER PRIMARY KEY AUTOINCREMENT,
name TEXT NOT NULL,
format TEXT NOT NULL,
status TEXT NOT NULL,
progress INTEGER NOT NULL DEFAULT 0,
file_path TEXT NOT NULL DEFAULT '',
source_url TEXT NOT NULL,
created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_songs_created_at ON songs(created_at DESC);
`

	if _, err := r.db.Exec(schema); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

func (r *Repository) ListSongs() ([]Song, error) {
	rows, err := r.db.Query(`
SELECT id, name, format, status, progress, file_path, source_url, created_at
FROM songs
ORDER BY datetime(created_at) DESC, id DESC
`)
	if err != nil {
		return nil, fmt.Errorf("list songs: %w", err)
	}
	defer rows.Close()

	var songs []Song
	for rows.Next() {
		song, err := scanSong(rows)
		if err != nil {
			return nil, err
		}
		songs = append(songs, song)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate songs: %w", err)
	}

	return songs, nil
}

func (r *Repository) CreateSong(url, format string) (Song, error) {
	name := url
	createdAt := time.Now().UTC()

	res, err := r.db.Exec(`
INSERT INTO songs(name, format, status, progress, file_path, source_url, created_at, updated_at)
VALUES (?, ?, 'downloading', 0, '', ?, ?, ?)
`, name, format, url, createdAt.Format(time.RFC3339Nano), createdAt.Format(time.RFC3339Nano))
	if err != nil {
		return Song{}, fmt.Errorf("insert song: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return Song{}, fmt.Errorf("last insert id: %w", err)
	}

	return Song{
		ID:        id,
		Name:      name,
		Format:    format,
		Status:    "downloading",
		Progress:  0,
		FilePath:  "",
		SourceURL: url,
		CreatedAt: createdAt,
	}, nil
}

func (r *Repository) UpdateDownload(id int64, name, filePath, status string, progress int) error {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}

	if strings.TrimSpace(name) == "" {
		name = "unknown"
	}

	_, err := r.db.Exec(`
UPDATE songs
SET name = ?, file_path = ?, status = ?, progress = ?, updated_at = ?
WHERE id = ?
`, name, filePath, status, progress, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return fmt.Errorf("update song: %w", err)
	}
	return nil
}

func (r *Repository) DeleteSong(id int64) error {
	res, err := r.db.Exec(`DELETE FROM songs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete song: %w", err)
	}

	count, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete song rows affected: %w", err)
	}

	if count == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *Repository) GetSong(id int64) (Song, error) {
	row := r.db.QueryRow(`
SELECT id, name, format, status, progress, file_path, source_url, created_at
FROM songs
WHERE id = ?
`, id)

	song, err := scanSong(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Song{}, sql.ErrNoRows
		}
		return Song{}, err
	}

	return song, nil
}

type songScanner interface {
	Scan(dest ...any) error
}

func scanSong(scanner songScanner) (Song, error) {
	var song Song
	var createdAtRaw string

	err := scanner.Scan(
		&song.ID,
		&song.Name,
		&song.Format,
		&song.Status,
		&song.Progress,
		&song.FilePath,
		&song.SourceURL,
		&createdAtRaw,
	)
	if err != nil {
		return Song{}, err
	}

	if ts, err := time.Parse(time.RFC3339Nano, createdAtRaw); err == nil {
		song.CreatedAt = ts
	} else if ts, err := time.Parse("2006-01-02 15:04:05", createdAtRaw); err == nil {
		song.CreatedAt = ts
	} else {
		song.CreatedAt = time.Now().UTC()
	}

	return song, nil
}
