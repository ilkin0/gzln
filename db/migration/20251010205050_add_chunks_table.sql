-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS chunks (
    id BIGSERIAL PRIMARY KEY,
    file_id UUID NOT NULL REFERENCES files (id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,
    storage_path TEXT NOT NULL,
    encrpyted_size INTEGER NOT NULL,
    chunk_hash VARCHAR(64) NOT NULL,
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_file_chunk UNIQUE (file_id, chunk_index)
);


CREATE INDEX idx_chunks_fileid_chunkindex ON chunks (file_id, chunk_index);

CREATE INDEX idx_chunks_fileid ON chunks (file_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS chunks;
-- +goose StatementEnd
