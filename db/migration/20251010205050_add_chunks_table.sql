-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS chunks (
    id BIGSERIAL PRIMARY KEY,
    file_id UUID NOT NULL REFERENCES files (id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,
    storage_path TEXT NOT NULL,
    encrypted_size BIGINT NOT NULL,
    chunk_hash VARCHAR(64) NOT NULL,
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_file_chunk UNIQUE (file_id, chunk_index),
    CONSTRAINT chk_chunk_index CHECK (chunk_index >= 0),
    CONSTRAINT chk_encrypted_size CHECK (encrypted_size > 0)
);


CREATE INDEX idx_chunks_fileid_chunkindex ON chunks (file_id, chunk_index);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS chunks;
-- +goose StatementEnd
