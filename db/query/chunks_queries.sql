-- name: ChunkExistsByFileIdAndIndex :one
SELECT EXISTS(
  SELECT 1
  FROM chunks
  WHERE file_id = $1 and chunk_index = $2
);

-- name: CreateChunk :one
INSERT INTO chunks (
    file_id,
    chunk_index,
    storage_path,
    encrypted_size,
    chunk_hash
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING id;

-- name: FileExistsByIdAndStatus :one
SELECT EXISTS(
  SELECT 1
  FROM files
  WHERE id = $1 and status = $2
);

-- name: CountChunksByFileId :one
SELECT
    COUNT(ID)
FROM chunks
WHERE file_id = $1;

-- name: GetChunkByIndexAndFileShareID :one
SELECT
    f.max_downloads,
    f.download_count,
    c.storage_path
FROM chunks c
JOIN files f on f.id = c.file_id
WHERE f.share_id = $1 and c.chunk_index = $2
  AND f.status = 'ready' AND f.expires_at > NOW();