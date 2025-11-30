-- name: ChunkExistsByFileIdAndIndex :one
SELECT EXISTS(
  SELECT 1
  FROM chunks
  WHERE file_id = $1 and chunk_index = $2
);
