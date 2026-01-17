-- name: CreateFile :one
INSERT INTO files (share_id,
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
                   uploader_ip)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: GetFileByID :one
SELECT *
FROM files
WHERE id = $1;

-- name: GetFileByShareID :one
SELECT *
FROM files
WHERE share_id = $1;

-- name: UpdateFileStatus :one
UPDATE files
SET status = $2
WHERE id = $1
RETURNING *;

-- name: GetFileSaltByShareId :one
SELECT salt
FROM files
WHERE share_id = $1;

-- name: GetFileMetadataByShareId :one
SELECT encrypted_filename,
       encrypted_mime_type,
       salt,
       total_size,
       chunk_count,
       expires_at,
       max_downloads,
       download_count
FROM files
WHERE share_id = $1;

-- name: CompleteFileDownloadByShareId :one
WITH updated AS (
    UPDATE files
        SET
            download_count = download_count + 1,
            last_downloaded_at = now()
        WHERE share_id = $1
            AND status = 'ready'
            AND (max_downloads = 0 OR download_count < max_downloads)
            AND (expires_at IS NULL OR expires_at > now())
        RETURNING id, share_id, download_count, max_downloads, expires_at)
SELECT u.id,
       u.share_id,
       u.download_count,
       u.max_downloads,
       (u.max_downloads > 0 AND u.download_count = u.max_downloads) AS reached_limit,
       expires_at
FROM updated u;


-- name: GetExpiredFiles :many
SELECT id, chunk_count
FROM files
WHERE status != 'expired'
  AND (
    expires_at <= now()
        OR (max_downloads > 0 AND download_count >= max_downloads));

-- name: ExpireFilesByIds :exec
UPDATE files
SET status = 'expired'
WHERE id = ANY ($1::uuid[]);