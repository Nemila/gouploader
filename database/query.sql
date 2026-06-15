-- name: GetPendingFile :many
SELECT * FROM files WHERE status = 'PENDING'
ORDER BY id LIMIT ? OFFSET ?;

-- name: AddFile :exec
INSERT INTO files (file_path) VALUES (?);

-- name: AddUpload :exec
INSERT INTO upload_jobs (file_id, host_name, status, last_error, slug) VALUES (?, ?, ?, ?, ?);

-- name: FindFileByPath :one
SELECT * FROM files WHERE file_path=? LIMIT 1;

-- name: GetFileUploads :many
SELECT * FROM upload_jobs WHERE file_id=?; 

-- name: UpdateFileStatus :exec
UPDATE files SET status = ? WHERE id = ?;

-- name: FailUpload :exec
UPDATE upload_jobs SET status = "FAILED", last_error = ? WHERE id = ?;

-- name: CompleteUpload :exec
UPDATE upload_jobs SET status = "DONE", slug = ? WHERE id = ?;

-- name: ResetProcessingStatuses :exec
UPDATE files SET status = 'PENDING' WHERE status = 'PROCESSING';