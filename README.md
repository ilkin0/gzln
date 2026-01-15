# gzln - Secure File Sharing Service

![CI](https://github.com/ilkin0/gzln/workflows/CI/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/ilkin0/gzln)](https://goreportcard.com/report/github.com/ilkin0/gzln)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A production-ready, self-hosted file sharing service with client-side encryption, chunked uploads, and automatic expiration. Built with Go, PostgreSQL, MinIO, and SvelteKit.

## Features

-  **Client-Side Encryption** - Files are encrypted in the browser before upload
-  **Chunked Upload/Download** - Large file support with resumable transfers
-  **Automatic Expiration** - Files auto-delete after expiration time
-  **Download Limits** - Control how many times a file can be downloaded
-  **Rate Limiting** - Built-in per-IP rate limiting for all endpoints
-  **Docker Ready** - Full Docker Compose setup with automated migrations

## Quick Start

### Prerequisites

- Docker and Docker Compose
- Go 1.25+ (for local development)
- Node.js 18+ (for frontend development)
- PostgreSQL 18+ (if running outside Docker)
- MinIO or S3-compatible storage

### Installation

1. **Clone the repository**
   ```bash
   git clone https://github.com/ilkin0/gzln.git
   cd gzln
   ```

2. **Create environment configuration**
   ```bash
   cp .env.example .env
   # Edit .env and update passwords and configuration
   ```

3. **Start services with Docker Compose**
   ```bash
   docker compose up -d
   ```

4. **Access the application**
   - Web UI: http://localhost:3000
   - MinIO Console: http://localhost:9001
   - API: http://localhost:8080/api/v1

### Development Setup

For local development without Docker:

```bash
# Install dependencies
go mod download
cd web && npm install

# Start database and MinIO
docker compose up -d db minio

# Run migrations
make goose-up

# Start development servers (backend + frontend)
make dev
```

## Architecture

### System Overview

```
┌─────────────┐
│   Browser   │ (Client-side encryption)
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   Nginx     │ (Optional: Load balancer)
└──────┬──────┘
       │
       ▼
┌─────────────┐     ┌──────────────┐
│  Go Server  │────▶│  PostgreSQL  │ (Metadata)
│   (API)     │     └──────────────┘
└──────┬──────┘
       │
       ▼
┌─────────────┐
│    MinIO    │ (Encrypted chunks)
└─────────────┘
```

### Technology Stack

**Backend:**
- Go 1.25 - Server implementation
- Chi - HTTP router
- SQLC - Type-safe SQL query code generation
- Goose - Database migrations
- pgx/v5 - PostgreSQL driver
- MinIO Go SDK - Object storage
- slog - Structured logging

**Frontend:**
- SvelteKit - Web framework
- TypeScript - Type safety
- Vite - Build tool

**Infrastructure:**
- PostgreSQL 18 - Metadata storage
- MinIO - Object storage for encrypted chunks
- Docker Compose - Container orchestration

## API Documentation

### Upload Flow

1. **Initialize Upload**
   ```
   POST /api/v1/files/upload/init
   ```
   Request:
   ```json
   {
     "salt": "base64-encoded-salt",
     "encrypted_filename": "encrypted-name",
     "encrypted_mime_type": "encrypted-mime",
     "total_size": 1048576,
     "chunk_count": 4,
     "chunk_size": 262144,
     "pbkdf2_iterations": 100000,
     "max_downloads": 5,
     "expires_in_hours": 24
   }
   ```
   Response:
   ```json
   {
     "file_id": "uuid",
     "share_id": "short-id",
     "upload_token": "auth-token",
     "expires_at": "2024-01-01T00:00:00Z"
   }
   ```

2. **Upload Chunks**
   ```
   POST /api/v1/files/{fileID}/chunks
   Content-Type: multipart/form-data
   Authorization: Bearer {upload_token}

   chunk_index: 0
   hash: sha256-hash
   file: binary-data
   ```

3. **Finalize Upload**
   ```
   POST /api/v1/files/{fileID}/finalize
   Authorization: Bearer {upload_token}
   ```

### Download Flow

1. **Get Metadata**
   ```
   GET /api/v1/download/{shareID}
   ```

2. **Download Chunks**
   ```
   GET /api/v1/download/{shareID}/chunk/{chunkIndex}
   ```

3. **Complete Download**
   ```
   POST /api/v1/download/{shareID}/complete
   ```

## Configuration

All configuration is done via environment variables. See [.env.example](.env.example) for details.

### Key Configuration Options

| Variable | Description | Default |
|----------|-------------|---------|
| `APP_ENV` | Environment (development/production) | `development` |
| `LOG_LEVEL` | Logging level (debug/info/warn/error) | `debug` |
| `SERVER_PORT` | HTTP server port | `8080` |
| `DB_PASSWORD` | PostgreSQL password | **Must set!** |
| `MINIO_ROOT_PASSWORD` | MinIO password | **Must set!** |
| `MAX_FILE_SIZE` | Maximum file size in bytes | `10485760` (10MB) |
| `RATE_LIMIT_*` | Rate limiting configuration | See .env.example |

## Development

### Available Make Commands

```bash
# Development
make dev                 # Start backend + frontend
make dev-backend         # Start backend with Air (live reload)
make dev-frontend        # Start frontend dev server

# Database
make goose-up            # Run migrations
make goose-down          # Rollback last migration
make goose-status        # Show migration status
make goose-create name=x # Create new migration
make sqlc                # Generate Go code from SQL

# Build & Run
make build               # Build the server binary
make run                 # Run the server

# Testing
make test                # Run Go tests with coverage
make test-short          # Run short tests only
make test-frontend       # Run frontend tests
make test-all            # Run all tests

# Code quality
make fmt                 # Format code
make vet                 # Run go vet
make tidy                # Tidy modules
```

### Running Tests

```bash
# Run all Go tests with coverage
make test

# Run short tests only (skip integration)
make test-short

# Run frontend tests
make test-frontend

# Run all tests (Go + frontend)
make test-all
```

### Adding New Migrations

```bash
# Create a new migration
make goose-create name=add_new_feature

# Then run migrations
make goose-up
```

## Deployment

### Docker Compose (Recommended)

1. **Configure environment**
   ```bash
   cp .env.example .env
   # Edit .env and set production values
   ```

2. **Update security settings**
   - Change all default passwords
   - Set `APP_ENV=production`
   - Set `LOG_LEVEL=info`
   - Configure `CORS_ALLOWED_ORIGINS` for your domain

3. **Deploy**
   ```bash
   docker compose up -d
   ```

### Manual Deployment

1. **Build the binary**
   ```bash
   make build
   # Binary will be at ./bin/server
   ```

2. **Setup PostgreSQL and MinIO**
   - Install and configure PostgreSQL 18+
   - Install and configure MinIO
   - Create database and run migrations

3. **Run the server**
   ```bash
   export DB_PASSWORD=your_password
   export MINIO_ROOT_PASSWORD=your_password
   # ... other env vars
   ./bin/server
   ```

## Security

### Client-Side Encryption

- All files are encrypted in the browser before upload using AES-GCM
- Encryption key is derived from the user password using PBKDF2
- Key never leaves the browser
- Server only stores encrypted chunks

## Monitoring

## Troubleshooting

### Common Issues


### Development Workflow

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests and linting
5. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

## Acknowledgments


---

Made with ❤️
