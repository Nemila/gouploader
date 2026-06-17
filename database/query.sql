-- name: GetFilesByStatus :many
SELECT * FROM files WHERE status = ? ORDER BY id;

-- name: GetUnArchivedFiles :many
SELECT * FROM files WHERE archived = FALSE;

-- name: UpsertFile :exec
INSERT INTO files (file_path, status, archived) VALUES (?, ?, ?) 
ON CONFLICT (file_path) 
DO UPDATE SET 
    status = COALESCE(excluded.status, status),
    archived = COALESCE(excluded.archived, archived);

-- name: InsertFile :exec
INSERT OR IGNORE INTO files (file_path, status) VALUES (?, 'pending');

-- name: UpdateFileStatus :exec
UPDATE files SET status = ? WHERE file_path = ?;

-- name: GetFileUploads :many
SELECT * FROM upload_jobs WHERE file_id=?;

-- name: ResetProcessingStatuses :exec
UPDATE files SET status = 'pending' WHERE status = 'processing';

-- name: UpsertUpload :exec
INSERT INTO upload_jobs (file_id, host_name, status, slug, last_error)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (file_id, host_name)
DO UPDATE SET
    status      = COALESCE(excluded.status, status),
    slug        = COALESCE(excluded.slug, slug),
    last_error  = COALESCE(excluded.last_error, last_error),
    retry_count = retry_count + 1;