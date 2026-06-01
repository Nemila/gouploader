-- name: GetPendingFile :many
SELECT * FROM files WHERE status = 'PENDING'
ORDER BY id LIMIT ? OFFSET ?;

-- name: AddFile :exec
INSERT INTO files (file_path) VALUES (?);

-- name: FindFileByPath :one
SELECT * FROM files WHERE file_path=? LIMIT 1;

-- name: GetFileUploads :many
SELECT * FROM upload_jobs WHERE file_id=? ORDER BY updated_at DESC; 

-- name: UpdateFileStatus :exec
UPDATE files SET status = ?, error = ? WHERE id = ?;