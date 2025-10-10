-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS files (
    id UUID PRIMARY KEY,
    share_id VARCHAR(12) NOT NULL UNIQUE,
    encrypted_filename TEXT NOT NULL,
    encrypted_mime_type TEXT NOT NULL,
    total_size BIGINT NOT NULL,
    chunk_count INTEGER NOT NULL,
    chunk_size INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'uploading',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ,
    max_downloads INTEGER,
    download_count INTEGER NOT NULL DEFAULT 0,
    last_downloaded_at TIMESTAMPTZ,
    deletion_token_hash VARCHAR(64),
    uploader_ip INET
);

CREATE INDEX idx_files_share_id ON files (share_id);

CREATE INDEX idx_files_expires_at ON files (expires_at);

CREATE INDEX idx_files_status_created ON files (status, created_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS files;
-- +goose StatementEnd
