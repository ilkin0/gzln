package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ilkin0/gzln/internal/database"
	"github.com/ilkin0/gzln/internal/repository/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cleanupTestFiles(t *testing.T, db *database.Database) {
	_, err := db.Pool.Exec(context.Background(), "TRUNCATE TABLE files CASCADE")
	require.NoError(t, err)
}

func TestGetFileMetadata_Integration_Success(t *testing.T) {
	handler, db, cleanup := setupTestHandler(t)
	defer cleanup()
	cleanupTestFiles(t, db)

	ctx := context.Background()

	file, err := db.Queries.CreateFile(ctx, sqlc.CreateFileParams{
		ShareID:           "testshare12",
		EncryptedFilename: "encrypted-filename",
		EncryptedMimeType: "encrypted-mime",
		Salt:              "test-salt",
		Pbkdf2Iterations:  100000,
		TotalSize:         1024 * 1024,
		ChunkCount:        4,
		ChunkSize:         256 * 1024,
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(24 * time.Hour),
			Valid: true,
		},
		MaxDownloads:      5,
		DeletionTokenHash: pgtype.Text{String: "token-hash", Valid: true},
		UploaderIp:        netip.MustParseAddr("192.168.1.1"),
	})
	require.NoError(t, err)

	_, err = db.Queries.UpdateFileStatus(ctx, sqlc.UpdateFileStatusParams{
		ID:     file.ID,
		Status: "ready",
	})
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/"+file.ShareID+"/metadata", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("shareID", file.ShareID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.GetFileMetadata(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response["success"].(bool))
	data := response["data"].(map[string]interface{})
	assert.Equal(t, "test-salt", data["salt"])
	assert.Equal(t, "encrypted-filename", data["encrypted_filename"])
	assert.Equal(t, "encrypted-mime", data["encrypted_mime_type"])
	assert.Equal(t, float64(1024*1024), data["total_size"])
	assert.Equal(t, float64(4), data["chunk_count"])
	assert.Equal(t, float64(5), data["max_downloads"])
	assert.Equal(t, float64(0), data["download_count"])
}

func TestGetFileMetadata_Integration_FileNotFound(t *testing.T) {
	handler, db, cleanup := setupTestHandler(t)
	defer cleanup()
	cleanupTestFiles(t, db)

	req := httptest.NewRequest("GET", "/nonexistent/metadata", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("shareID", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.GetFileMetadata(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetFileMetadata_Integration_FileExpired(t *testing.T) {
	handler, db, cleanup := setupTestHandler(t)
	defer cleanup()
	cleanupTestFiles(t, db)

	ctx := context.Background()

	file, err := db.Queries.CreateFile(ctx, sqlc.CreateFileParams{
		ShareID:           "expired123",
		EncryptedFilename: "encrypted-filename",
		EncryptedMimeType: "encrypted-mime",
		Salt:              "test-salt",
		Pbkdf2Iterations:  100000,
		TotalSize:         1024 * 1024,
		ChunkCount:        4,
		ChunkSize:         256 * 1024,
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(24 * time.Hour),
			Valid: true,
		},
		MaxDownloads:      5,
		DeletionTokenHash: pgtype.Text{String: "token-hash", Valid: true},
		UploaderIp:        netip.MustParseAddr("192.168.1.1"),
	})
	require.NoError(t, err)

	now := time.Now()
	_, err = db.Pool.Exec(ctx, `
		UPDATE files
		SET created_at = $1, expires_at = $2
		WHERE id = $3
	`, now.Add(-2*time.Hour), now.Add(-1*time.Hour), file.ID)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/"+file.ShareID+"/metadata", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("shareID", file.ShareID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.GetFileMetadata(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response["success"].(bool))

	data := response["data"].(map[string]interface{})
	assert.NotNil(t, data["expires_at"])
}

func TestCompleteDownload_Integration_Success(t *testing.T) {
	handler, db, cleanup := setupTestHandler(t)
	defer cleanup()
	cleanupTestFiles(t, db)

	ctx := context.Background()

	file, err := db.Queries.CreateFile(ctx, sqlc.CreateFileParams{
		ShareID:           "complete123",
		EncryptedFilename: "encrypted-filename",
		EncryptedMimeType: "encrypted-mime",
		Salt:              "test-salt",
		Pbkdf2Iterations:  100000,
		TotalSize:         1024 * 1024,
		ChunkCount:        4,
		ChunkSize:         256 * 1024,
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(24 * time.Hour),
			Valid: true,
		},
		MaxDownloads:      5,
		DeletionTokenHash: pgtype.Text{String: "token-hash", Valid: true},
		UploaderIp:        netip.MustParseAddr("192.168.1.1"),
	})
	require.NoError(t, err)

	_, err = db.Queries.UpdateFileStatus(ctx, sqlc.UpdateFileStatusParams{
		ID:     file.ID,
		Status: "ready",
	})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/"+file.ShareID+"/complete", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("shareID", file.ShareID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.CompleteDownload(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response["success"].(bool))

	updatedFile, err := db.Queries.GetFileByShareID(ctx, file.ShareID)
	require.NoError(t, err)
	assert.Equal(t, int32(1), updatedFile.DownloadCount)
}

func TestCompleteDownload_Integration_FileNotFound(t *testing.T) {
	handler, db, cleanup := setupTestHandler(t)
	defer cleanup()
	cleanupTestFiles(t, db)

	req := httptest.NewRequest("POST", "/nonexistent/complete", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("shareID", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.CompleteDownload(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "not found")
}

func TestCompleteDownload_Integration_FileExpired(t *testing.T) {
	handler, db, cleanup := setupTestHandler(t)
	defer cleanup()
	cleanupTestFiles(t, db)

	ctx := context.Background()

	file, err := db.Queries.CreateFile(ctx, sqlc.CreateFileParams{
		ShareID:           "expiredcmp",
		EncryptedFilename: "encrypted-filename",
		EncryptedMimeType: "encrypted-mime",
		Salt:              "test-salt",
		Pbkdf2Iterations:  100000,
		TotalSize:         1024 * 1024,
		ChunkCount:        4,
		ChunkSize:         256 * 1024,
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(24 * time.Hour),
			Valid: true,
		},
		MaxDownloads:      5,
		DeletionTokenHash: pgtype.Text{String: "token-hash", Valid: true},
		UploaderIp:        netip.MustParseAddr("192.168.1.1"),
	})
	require.NoError(t, err)

	_, err = db.Queries.UpdateFileStatus(ctx, sqlc.UpdateFileStatusParams{
		ID:     file.ID,
		Status: "ready",
	})
	require.NoError(t, err)

	now := time.Now()
	_, err = db.Pool.Exec(ctx, `
		UPDATE files
		SET created_at = $1, expires_at = $2
		WHERE id = $3
	`, now.Add(-2*time.Hour), now.Add(-1*time.Hour), file.ID)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/"+file.ShareID+"/complete", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("shareID", file.ShareID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.CompleteDownload(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "expired")
}

func TestCompleteDownload_Integration_LimitReached(t *testing.T) {
	handler, db, cleanup := setupTestHandler(t)
	defer cleanup()
	cleanupTestFiles(t, db)

	ctx := context.Background()

	file, err := db.Queries.CreateFile(ctx, sqlc.CreateFileParams{
		ShareID:           "limitreach1",
		EncryptedFilename: "encrypted-filename",
		EncryptedMimeType: "encrypted-mime",
		Salt:              "test-salt",
		Pbkdf2Iterations:  100000,
		TotalSize:         1024 * 1024,
		ChunkCount:        4,
		ChunkSize:         256 * 1024,
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(24 * time.Hour),
			Valid: true,
		},
		MaxDownloads:      1,
		DeletionTokenHash: pgtype.Text{String: "token-hash", Valid: true},
		UploaderIp:        netip.MustParseAddr("192.168.1.1"),
	})
	require.NoError(t, err)

	_, err = db.Queries.UpdateFileStatus(ctx, sqlc.UpdateFileStatusParams{
		ID:     file.ID,
		Status: "ready",
	})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/"+file.ShareID+"/complete", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("shareID", file.ShareID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.CompleteDownload(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	updatedFile, err := db.Queries.GetFileByShareID(ctx, file.ShareID)
	require.NoError(t, err)
	assert.Equal(t, int32(1), updatedFile.DownloadCount)
	assert.Equal(t, "exhausted", updatedFile.Status)

	req2 := httptest.NewRequest("POST", "/"+file.ShareID+"/complete", nil)
	req2 = req2.WithContext(context.WithValue(req2.Context(), chi.RouteCtxKey, rctx))
	w2 := httptest.NewRecorder()

	handler.CompleteDownload(w2, req2)
	assert.Equal(t, http.StatusBadRequest, w2.Code)
	assert.Contains(t, w2.Body.String(), "download limit")
}

func TestCompleteDownload_Integration_NotReady(t *testing.T) {
	handler, db, cleanup := setupTestHandler(t)
	defer cleanup()
	cleanupTestFiles(t, db)

	ctx := context.Background()

	file, err := db.Queries.CreateFile(ctx, sqlc.CreateFileParams{
		ShareID:           "notready123",
		EncryptedFilename: "encrypted-filename",
		EncryptedMimeType: "encrypted-mime",
		Salt:              "test-salt",
		Pbkdf2Iterations:  100000,
		TotalSize:         1024 * 1024,
		ChunkCount:        4,
		ChunkSize:         256 * 1024,
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(24 * time.Hour),
			Valid: true,
		},
		MaxDownloads:      5,
		DeletionTokenHash: pgtype.Text{String: "token-hash", Valid: true},
		UploaderIp:        netip.MustParseAddr("192.168.1.1"),
	})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/"+file.ShareID+"/complete", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("shareID", file.ShareID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.CompleteDownload(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "not ready")
}

func TestCompleteDownload_Integration_MultipleDownloads(t *testing.T) {
	handler, db, cleanup := setupTestHandler(t)
	defer cleanup()
	cleanupTestFiles(t, db)

	ctx := context.Background()

	file, err := db.Queries.CreateFile(ctx, sqlc.CreateFileParams{
		ShareID:           "multi123",
		EncryptedFilename: "encrypted-filename",
		EncryptedMimeType: "encrypted-mime",
		Salt:              "test-salt",
		Pbkdf2Iterations:  100000,
		TotalSize:         1024 * 1024,
		ChunkCount:        4,
		ChunkSize:         256 * 1024,
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(24 * time.Hour),
			Valid: true,
		},
		MaxDownloads:      3,
		DeletionTokenHash: pgtype.Text{String: "token-hash", Valid: true},
		UploaderIp:        netip.MustParseAddr("192.168.1.1"),
	})
	require.NoError(t, err)

	_, err = db.Queries.UpdateFileStatus(ctx, sqlc.UpdateFileStatusParams{
		ID:     file.ID,
		Status: "ready",
	})
	require.NoError(t, err)

	for i := 1; i <= 3; i++ {
		req := httptest.NewRequest("POST", "/"+file.ShareID+"/complete", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("shareID", file.ShareID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		handler.CompleteDownload(w, req)
		assert.Equal(t, http.StatusOK, w.Code, fmt.Sprintf("Download %d should succeed", i))

		updatedFile, err := db.Queries.GetFileByShareID(ctx, file.ShareID)
		require.NoError(t, err)
		assert.Equal(t, int32(i), updatedFile.DownloadCount)
	}

	finalFile, err := db.Queries.GetFileByShareID(ctx, file.ShareID)
	require.NoError(t, err)
	assert.Equal(t, "exhausted", finalFile.Status)

	req4 := httptest.NewRequest("POST", "/"+file.ShareID+"/complete", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("shareID", file.ShareID)
	req4 = req4.WithContext(context.WithValue(req4.Context(), chi.RouteCtxKey, rctx))
	w4 := httptest.NewRecorder()

	handler.CompleteDownload(w4, req4)
	assert.Equal(t, http.StatusBadRequest, w4.Code)
	assert.Contains(t, w4.Body.String(), "download limit")
}
