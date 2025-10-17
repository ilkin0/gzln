-- name: CreateFile :one
INSERT INTO files (
    share_id,
    encrypted_filename,
    encrypted_mime_type,
    salt,
    pbkdf2_iterations,
    total_size,
    chunk_count,
    chunk_size,
    expires_at,
    max_downloads,
    deletion_token_hash,
    uploader_ip
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
) RETURNING *;

-- name: GetFileByID :one
SELECT * FROM files
WHERE id = $1;

-- name: GetFileByShareID :one
SELECT * FROM files
WHERE share_id = $1;

-- name: UpdateFileStatus :one
UPDATE files
SET status = $2
WHERE id = $1
RETURNING *;

-- name: IncrementDownloadCount :one
UPDATE files
SET
    download_count = download_count + 1,
    last_downloaded_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteExpiredFiles :exec
DELETE FROM files
WHERE expires_at < now() OR (max_downloads <= download_count AND status = 'ready');
