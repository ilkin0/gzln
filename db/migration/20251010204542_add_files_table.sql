-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    share_id VARCHAR(12) NOT NULL UNIQUE,
    encrypted_filename TEXT NOT NULL,
    encrypted_mime_type TEXT NOT NULL,
    salt VARCHAR(24) NOT NULL,
    pbkdf2_iterations INTEGER NOT NULL,
    total_size BIGINT NOT NULL,
    chunk_count INTEGER NOT NULL,
    chunk_size INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'uploading',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    last_downloaded_at TIMESTAMPTZ,
    max_downloads INTEGER NOT NULL DEFAULT 10,
    download_count INTEGER NOT NULL DEFAULT 0,
    deletion_token_hash VARCHAR(64),
    uploader_ip INET NOT NULL,
    CONSTRAINT chk_chunk_count CHECK (chunk_count > 0),
    CONSTRAINT chk_download_count CHECK (download_count >= 0),
    CONSTRAINT chk_max_downloads CHECK (max_downloads > 0),
    CONSTRAINT chk_expires_after_created CHECK (expires_at > created_at)
);

CREATE INDEX idx_files_share_id ON files (share_id);

CREATE INDEX idx_files_expires_at ON files (expires_at) WHERE status = 'ready';

CREATE INDEX idx_files_status_created ON files (status, created_at);

CREATE INDEX idx_files_uploader_ip ON files (uploader_ip, created_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS files CASCADE;
-- +goose StatementEnd
