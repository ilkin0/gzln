package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ilkin0/gzln/internal/api/types"
	"github.com/ilkin0/gzln/internal/database"
	"github.com/ilkin0/gzln/internal/repository/sqlc"
	"github.com/ilkin0/gzln/internal/service"
	"github.com/ilkin0/gzln/internal/testutil"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestHandler(t *testing.T) (*FileHandler, *database.Database, func()) {
	t.Helper()

	containers := testutil.SetupTestContainers(t)

	txRunner := database.NewTxRunner(containers.Database.Pool)
	fileService := service.NewFileService(containers.Database.Queries, txRunner, containers.MinioClient.Client)
	handler := NewFileHandler(fileService, containers.MinioClient.BucketName)

	return handler, containers.Database, containers.Cleanup
}

func TestInitUpload_Integration_Success(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	req := types.InitUploadRequest{
		Salt:              "test-salt-value",
		EncryptedFilename: "encrypted-filename",
		EncryptedMimeType: "encrypted-mime-type",
		TotalSize:         1024 * 1024, // 1MB
		ChunkCount:        4,           // ceil(1MB / 256KB) = 4
		ChunkSize:         256 * 1024,  // 256KB
		Pbkdf2Iterations:  100000,
		MaxDownloads:      5,
		ExpiresInHours:    24,
	}

	body, err := json.Marshal(req)
	require.NoError(t, err)

	httpReq := httptest.NewRequest("POST", "/upload/init", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.InitUpload(w, httpReq)

	assert.Equal(t, http.StatusOK, w.Code)

	var wrappedResp struct {
		Success bool                     `json:"success"`
		Data    types.InitUploadResponse `json:"data"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &wrappedResp)
	require.NoError(t, err)
	require.True(t, wrappedResp.Success)

	resp := wrappedResp.Data
	assert.NotEmpty(t, resp.FileID)
	assert.NotEmpty(t, resp.ShareID)
	assert.NotEmpty(t, resp.UploadToken)
	assert.NotEmpty(t, resp.ExpiresAt)

	assert.Len(t, resp.ShareID, 12)

	expiryTime, err := time.Parse(time.RFC3339, resp.ExpiresAt)
	require.NoError(t, err)
	expectedExpiry := time.Now().Add(24 * time.Hour)
	assert.WithinDuration(t, expectedExpiry, expiryTime, 5*time.Second)
}

func TestInitUpload_Integration_InvalidJSON(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	invalidJSON := []byte(`{"invalid": json}`)

	httpReq := httptest.NewRequest("POST", "/upload/init", bytes.NewReader(invalidJSON))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.InitUpload(w, httpReq)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to parse request body")
}

func TestInitUpload_Integration_MissingRequiredFields(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	tests := []struct {
		name     string
		req      types.InitUploadRequest
		errorMsg string
	}{
		{
			name: "missing salt",
			req: types.InitUploadRequest{
				EncryptedFilename: "encrypted-filename",
				EncryptedMimeType: "encrypted-mime-type",
				TotalSize:         1024,
				ChunkCount:        4,          // ceil(1MB / 256KB) = 4
				ChunkSize:         256 * 1024, // 256KB
				Pbkdf2Iterations:  100000,
			},
			errorMsg: "salt is required",
		},
		{
			name: "missing encrypted filename",
			req: types.InitUploadRequest{
				Salt:              "test-salt",
				EncryptedMimeType: "encrypted-mime-type",
				TotalSize:         1024,
				ChunkCount:        4,          // ceil(1MB / 256KB) = 4
				ChunkSize:         256 * 1024, // 256KB
				Pbkdf2Iterations:  100000,
			},
			errorMsg: "encrypted_filename is required",
		},
		{
			name: "file size exceeds limit",
			req: types.InitUploadRequest{
				Salt:              "test-salt",
				EncryptedFilename: "encrypted-filename",
				EncryptedMimeType: "encrypted-mime-type",
				TotalSize:         6 << 30,    // 6GB
				ChunkCount:        24576,      // ceil(6GB / 256KB) = 24576
				ChunkSize:         256 * 1024, // 256KB
				Pbkdf2Iterations:  100000,
			},
			errorMsg: "file size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(tt.req)
			require.NoError(t, err)

			httpReq := httptest.NewRequest("POST", "/upload/init", bytes.NewReader(body))
			httpReq.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.InitUpload(w, httpReq)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), tt.errorMsg)
		})
	}
}

func TestInitUpload_Integration_IPExtraction(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	req := types.InitUploadRequest{
		Salt:              "test-salt-value",
		EncryptedFilename: "encrypted-filename",
		EncryptedMimeType: "encrypted-mime-type",
		TotalSize:         1024 * 1024,
		ChunkCount:        4,          // ceil(1MB / 256KB) = 4
		ChunkSize:         256 * 1024, // 256KB
		Pbkdf2Iterations:  100000,
	}

	tests := []struct {
		name   string
		header string
		value  string
	}{
		{
			name:   "X-Forwarded-For header",
			header: "X-Forwarded-For",
			value:  "203.0.113.1",
		},
		{
			name:   "X-Real-IP header",
			header: "X-Real-IP",
			value:  "203.0.113.2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(req)
			require.NoError(t, err)

			httpReq := httptest.NewRequest("POST", "/upload/init", bytes.NewReader(body))
			httpReq.Header.Set("Content-Type", "application/json")
			httpReq.Header.Set(tt.header, tt.value)
			w := httptest.NewRecorder()

			handler.InitUpload(w, httpReq)

			assert.Equal(t, http.StatusOK, w.Code)

			var wrappedResp struct {
				Success bool                     `json:"success"`
				Data    types.InitUploadResponse `json:"data"`
			}
			err = json.Unmarshal(w.Body.Bytes(), &wrappedResp)
			require.NoError(t, err)
			require.True(t, wrappedResp.Success)

			resp := wrappedResp.Data
			assert.NotEmpty(t, resp.ShareID)
		})
	}
}

func TestInitUpload_Integration_DefaultValues(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	req := types.InitUploadRequest{
		Salt:              "test-salt-value",
		EncryptedFilename: "encrypted-filename",
		EncryptedMimeType: "encrypted-mime-type",
		TotalSize:         1024 * 1024,
		ChunkCount:        4,          // ceil(1MB / 256KB) = 4
		ChunkSize:         256 * 1024, // 256KB
		Pbkdf2Iterations:  100000,
	}

	body, err := json.Marshal(req)
	require.NoError(t, err)

	httpReq := httptest.NewRequest("POST", "/upload/init", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.InitUpload(w, httpReq)

	assert.Equal(t, http.StatusOK, w.Code)

	var wrappedResp struct {
		Success bool                     `json:"success"`
		Data    types.InitUploadResponse `json:"data"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &wrappedResp)
	require.NoError(t, err)
	require.True(t, wrappedResp.Success)

	resp := wrappedResp.Data
	expiryTime, err := time.Parse(time.RFC3339, resp.ExpiresAt)
	require.NoError(t, err)
	expectedExpiry := time.Now().Add(72 * time.Hour) // default expiryTime is 72 hrs
	assert.WithinDuration(t, expectedExpiry, expiryTime, 5*time.Second)
}

func TestFinalizeUpload_Integration_Success(t *testing.T) {
	handler, db, cleanup := setupTestHandler(t)
	defer cleanup()

	initReq := types.InitUploadRequest{
		Salt:              "test-salt-value",
		EncryptedFilename: "encrypted-filename",
		EncryptedMimeType: "encrypted-mime-type",
		TotalSize:         1024 * 1024, // 1MB
		ChunkCount:        4,           // ceil(1MB / 256KB) = 4
		ChunkSize:         256 * 1024,  // 256KB
		Pbkdf2Iterations:  100000,
	}

	body, err := json.Marshal(initReq)
	require.NoError(t, err)

	httpReq := httptest.NewRequest("POST", "/upload/init", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.InitUpload(w, httpReq)
	require.Equal(t, http.StatusOK, w.Code)

	var wrappedResp struct {
		Success bool                     `json:"success"`
		Data    types.InitUploadResponse `json:"data"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &wrappedResp)
	require.NoError(t, err)
	require.True(t, wrappedResp.Success)

	initResp := wrappedResp.Data
	require.NotEmpty(t, initResp.FileID)

	ctx := context.Background()
	var fileID pgtype.UUID
	err = fileID.Scan(initResp.FileID)
	require.NoError(t, err, "Failed to parse file ID: %s", initResp.FileID)

	for i := int64(0); i < 4; i++ {
		_, err := db.Queries.CreateChunk(ctx, sqlc.CreateChunkParams{
			FileID:        fileID,
			ChunkIndex:    int32(i),
			StoragePath:   fmt.Sprintf("chunks/%s/%d.enc", initResp.FileID, i),
			EncryptedSize: 15,
			ChunkHash:     "34fa0947d659ce6343cbfe6be3a1ca882f6b21b35232210f194791d545440c40",
		})
		require.NoError(t, err, "Failed to create chunk %d", i)
	}

	httpReq2 := httptest.NewRequest("POST", "/"+initResp.FileID+"/finalize", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("fileID", initResp.FileID)
	httpReq2 = httpReq2.WithContext(context.WithValue(httpReq2.Context(), chi.RouteCtxKey, rctx))
	w2 := httptest.NewRecorder()

	handler.FinalizeFileUpload(w2, httpReq2)

	assert.Equal(t, http.StatusOK, w2.Code)

	var wrappedFinalResp struct {
		Success bool                         `json:"success"`
		Data    types.FinalizeUploadResponse `json:"data"`
	}
	err = json.Unmarshal(w2.Body.Bytes(), &wrappedFinalResp)
	require.NoError(t, err)
	require.True(t, wrappedFinalResp.Success)

	finalResp := wrappedFinalResp.Data
	assert.Equal(t, initResp.ShareID, finalResp.ShareID)
	assert.NotEmpty(t, finalResp.DeletionToken)
}

func TestFinalizeUpload_Integration_InvalidFileID(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	httpReq := httptest.NewRequest("POST", "/invalid-uuid/finalize", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("fileID", "invalid-uuid")
	httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.FinalizeFileUpload(w, httpReq)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid file ID")
}

func TestFinalizeUpload_Integration_FileNotFound(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	nonExistentFileID := "550e8400-e29b-41d4-a716-446655440000"

	httpReq := httptest.NewRequest("POST", "/"+nonExistentFileID+"/finalize", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("fileID", nonExistentFileID)
	httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.FinalizeFileUpload(w, httpReq)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "failed to get file metadata")
}

func TestFinalizeUpload_Integration_ChunkCountMismatch(t *testing.T) {
	handler, db, cleanup := setupTestHandler(t)
	defer cleanup()

	initReq := types.InitUploadRequest{
		Salt:              "test-salt-value",
		EncryptedFilename: "encrypted-filename",
		EncryptedMimeType: "encrypted-mime-type",
		TotalSize:         1024 * 1024, // 1MB
		ChunkCount:        4,           // ceil(1MB / 256KB) = 4
		ChunkSize:         256 * 1024,  // 256KB
		Pbkdf2Iterations:  100000,
	}

	body, err := json.Marshal(initReq)
	require.NoError(t, err)

	httpReq := httptest.NewRequest("POST", "/upload/init", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.InitUpload(w, httpReq)
	require.Equal(t, http.StatusOK, w.Code)

	var wrappedResp struct {
		Success bool                     `json:"success"`
		Data    types.InitUploadResponse `json:"data"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &wrappedResp)
	require.NoError(t, err)
	require.True(t, wrappedResp.Success)

	initResp := wrappedResp.Data
	require.NotEmpty(t, initResp.FileID)

	ctx := context.Background()
	var fileID pgtype.UUID
	err = fileID.Scan(initResp.FileID)
	require.NoError(t, err)

	for i := int64(0); i < 2; i++ {
		_, err := db.Queries.CreateChunk(ctx, sqlc.CreateChunkParams{
			FileID:        fileID,
			ChunkIndex:    int32(i),
			StoragePath:   fmt.Sprintf("chunks/%s/%d.enc", initResp.FileID, i),
			EncryptedSize: 15,
			ChunkHash:     "34fa0947d659ce6343cbfe6be3a1ca882f6b21b35232210f194791d545440c40",
		})
		require.NoError(t, err, "Failed to create chunk %d", i)
	}

	httpReq2 := httptest.NewRequest("POST", "/"+initResp.FileID+"/finalize", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("fileID", initResp.FileID)
	httpReq2 = httpReq2.WithContext(context.WithValue(httpReq2.Context(), chi.RouteCtxKey, rctx))
	w2 := httptest.NewRecorder()

	handler.FinalizeFileUpload(w2, httpReq2)

	assert.Equal(t, http.StatusInternalServerError, w2.Code)
	assert.Contains(t, w2.Body.String(), "chunk count does not match")
}
