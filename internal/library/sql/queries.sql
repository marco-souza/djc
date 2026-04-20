-- name: ListSongs :many
SELECT id, name, format, status, progress, file_path, source_url, created_at
FROM songs
ORDER BY datetime(created_at) DESC, id DESC;

-- name: CreateSong :execresult
INSERT INTO songs(name, format, status, progress, file_path, source_url, created_at, updated_at)
VALUES (?, ?, 'downloading', 0, '', ?, ?, ?);

-- name: GetSong :one
SELECT id, name, format, status, progress, file_path, source_url, created_at
FROM songs
WHERE id = ?;

-- name: UpdateDownload :exec
UPDATE songs
SET name = ?, file_path = ?, status = ?, progress = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteSong :execresult
DELETE FROM songs WHERE id = ?;
