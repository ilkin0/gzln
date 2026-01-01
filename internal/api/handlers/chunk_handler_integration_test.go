package handlers

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/ilkin0/gzln/internal/api/types"
	"github.com/ilkin0/gzln/internal/database"
	"github.com/ilkin0/gzln/internal/service"
	"github.com/ilkin0/gzln/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestChunkHandler(t *testing.T) (*ChunkHandler, *service.FileService, func()) {
	t.Helper()

	containers := testutil.SetupTestContainers(t)

	chunkService := service.NewChunkService(containers.Database.Queries, containers.MinioClient.Client, containers.MinioClient.BucketName)
	txRunner := database.NewTxRunner(containers.Database.Pool)
	fileService := service.NewFileService(containers.Database.Queries, txRunner, containers.MinioClient.Client)
	handler := NewChunkHandler(chunkService, containers.MinioClient.BucketName)

	return handler, fileService, containers.Cleanup
}

func createTestFile(t *testing.T, fileService *service.FileService) (string, string) {
	ctx := context.Background()
	req := types.InitUploadRequest{
		Salt:              "test-salt-value",
		EncryptedFilename: "encrypted-filename",
		EncryptedMimeType: "encrypted-mime-type",
		TotalSize:         1024 * 1024,
		ChunkCount:        4,          // ceil(1MB / 256KB) = 4
		ChunkSize:         256 * 1024, // 256KB
		Pbkdf2Iterations:  100000,
		MaxDownloads:      5,
		ExpiresInHours:    24,
	}

	resp, err := fileService.InitFileUpload(ctx, req, "192.168.1.1")
	require.NoError(t, err)

	return resp.FileID, resp.UploadToken
}

func TestHandleChunkUpload_Integration_MissingAuthorization(t *testing.T) {
	handler, _, cleanup := setupTestChunkHandler(t)
	defer cleanup()

	httpReq := httptest.NewRequest("POST", "/upload/chunk/550e8400-e29b-41d4-a716-446655440000", nil)
	httpReq.Header.Set("Content-Type", "multipart/form-data")
	w := httptest.NewRecorder()

	handler.HandleChunkUpload(w, httpReq)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Authorization required")
}

func TestHandleChunkUpload_Integration_InvalidFileID(t *testing.T) {
	handler, _, cleanup := setupTestChunkHandler(t)
	defer cleanup()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("chunk", "chunk.enc")
	require.NoError(t, err)
	_, err = io.WriteString(part, "test chunk data")
	require.NoError(t, err)

	err = writer.WriteField("chunk_index", "0")
	require.NoError(t, err)
	err = writer.WriteField("hash", "34fa0947d659ce6343cbfe6be3a1ca882f6b21b35232210f194791d545440c40")
	require.NoError(t, err)

	writer.Close()

	httpReq := httptest.NewRequest("POST", "/upload/chunk/invalid-uuid", body)
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Authorization", "Bearer test-token")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("fileID", "invalid-uuid")
	httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()

	handler.HandleChunkUpload(w, httpReq)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid file ID")
}

func TestHandleChunkUpload_Integration_MissingChunkFile(t *testing.T) {
	handler, fileService, cleanup := setupTestChunkHandler(t)
	defer cleanup()

	fileID, _ := createTestFile(t, fileService)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	err := writer.WriteField("chunk_index", "0")
	require.NoError(t, err)
	err = writer.WriteField("hash", "34fa0947d659ce6343cbfe6be3a1ca882f6b21b35232210f194791d545440c40")
	require.NoError(t, err)

	writer.Close()

	httpReq := httptest.NewRequest("POST", "/upload/chunk/"+fileID, body)
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Authorization", "Bearer test-token")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("fileID", fileID)
	httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()

	handler.HandleChunkUpload(w, httpReq)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "File chunk is missing")
}

func TestHandleChunkUpload_Integration_InvalidChunkIndex(t *testing.T) {
	handler, fileService, cleanup := setupTestChunkHandler(t)
	defer cleanup()

	fileID, _ := createTestFile(t, fileService)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("chunk", "chunk.enc")
	require.NoError(t, err)
	_, err = io.WriteString(part, "test chunk data")
	require.NoError(t, err)

	err = writer.WriteField("chunk_index", "invalid")
	require.NoError(t, err)
	err = writer.WriteField("hash", "34fa0947d659ce6343cbfe6be3a1ca882f6b21b35232210f194791d545440c40")
	require.NoError(t, err)

	writer.Close()

	httpReq := httptest.NewRequest("POST", "/upload/chunk/"+fileID, body)
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Authorization", "Bearer test-token")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("fileID", fileID)
	httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()

	handler.HandleChunkUpload(w, httpReq)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid chunk index")
}

func TestHandleChunkUpload_Integration_Success(t *testing.T) {
	handler, fileService, cleanup := setupTestChunkHandler(t)
	defer cleanup()

	fileID, token := createTestFile(t, fileService)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("chunk", "chunk.enc")
	require.NoError(t, err)
	_, err = io.WriteString(part, "test chunk data")
	require.NoError(t, err)

	err = writer.WriteField("chunk_index", "0")
	require.NoError(t, err)
	err = writer.WriteField("hash", "34fa0947d659ce6343cbfe6be3a1ca882f6b21b35232210f194791d545440c40")
	require.NoError(t, err)

	writer.Close()

	httpReq := httptest.NewRequest("POST", "/upload/chunk/"+fileID, body)
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Authorization", "Bearer "+token)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("fileID", fileID)
	httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()

	handler.HandleChunkUpload(w, httpReq)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "uploaded")
	assert.Contains(t, w.Body.String(), "34fa0947d659ce6343cbfe6be3a1ca882f6b21b35232210f194791d545440c40")
}

func TestHandleChunkUpload_Integration_HashMismatch(t *testing.T) {
	handler, fileService, cleanup := setupTestChunkHandler(t)
	defer cleanup()

	fileID, token := createTestFile(t, fileService)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("chunk", "chunk.enc")
	require.NoError(t, err)
	_, err = io.WriteString(part, "test chunk data")
	require.NoError(t, err)

	err = writer.WriteField("chunk_index", "0")
	require.NoError(t, err)
	err = writer.WriteField("hash", "wrong-hash-value")
	require.NoError(t, err)

	writer.Close()

	httpReq := httptest.NewRequest("POST", "/upload/chunk/"+fileID, body)
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Authorization", "Bearer "+token)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("fileID", fileID)
	httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()

	handler.HandleChunkUpload(w, httpReq)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "hash mismatch")
}

func TestHandleChunkUpload_Integration_ChunkAlreadyExists(t *testing.T) {
	handler, fileService, cleanup := setupTestChunkHandler(t)
	defer cleanup()

	fileID, token := createTestFile(t, fileService)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("chunk", "chunk.enc")
	require.NoError(t, err)
	_, err = io.WriteString(part, "test chunk data")
	require.NoError(t, err)

	err = writer.WriteField("chunk_index", "0")
	require.NoError(t, err)
	err = writer.WriteField("hash", "34fa0947d659ce6343cbfe6be3a1ca882f6b21b35232210f194791d545440c40")
	require.NoError(t, err)

	writer.Close()

	httpReq := httptest.NewRequest("POST", "/upload/chunk/"+fileID, body)
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Authorization", "Bearer "+token)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("fileID", fileID)
	httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()

	handler.HandleChunkUpload(w, httpReq)
	assert.Equal(t, http.StatusOK, w.Code)

	body2 := &bytes.Buffer{}
	writer2 := multipart.NewWriter(body2)

	part2, err := writer2.CreateFormFile("chunk", "chunk.enc")
	require.NoError(t, err)
	_, err = io.WriteString(part2, "test chunk data")
	require.NoError(t, err)

	err = writer2.WriteField("chunk_index", "0")
	require.NoError(t, err)
	err = writer2.WriteField("hash", "34fa0947d659ce6343cbfe6be3a1ca882f6b21b35232210f194791d545440c40")
	require.NoError(t, err)

	writer2.Close()

	httpReq2 := httptest.NewRequest("POST", "/upload/chunk/"+fileID, body2)
	httpReq2.Header.Set("Content-Type", writer2.FormDataContentType())
	httpReq2.Header.Set("Authorization", "Bearer "+token)

	rctx2 := chi.NewRouteContext()
	rctx2.URLParams.Add("fileID", fileID)
	httpReq2 = httpReq2.WithContext(context.WithValue(httpReq2.Context(), chi.RouteCtxKey, rctx2))

	w2 := httptest.NewRecorder()

	handler.HandleChunkUpload(w2, httpReq2)

	assert.Equal(t, http.StatusConflict, w2.Code)
	assert.Contains(t, w2.Body.String(), "already uploaded")
}

func TestHandleChunkUpload_Integration_FileNotFound(t *testing.T) {
	handler, _, cleanup := setupTestChunkHandler(t)
	defer cleanup()

	fileID := "550e8400-e29b-41d4-a716-446655440000"

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("chunk", "chunk.enc")
	require.NoError(t, err)
	_, err = io.WriteString(part, "test chunk data")
	require.NoError(t, err)

	err = writer.WriteField("chunk_index", "0")
	require.NoError(t, err)
	err = writer.WriteField("hash", "34fa0947d659ce6343cbfe6be3a1ca882f6b21b35232210f194791d545440c40")
	require.NoError(t, err)

	writer.Close()

	httpReq := httptest.NewRequest("POST", "/upload/chunk/"+fileID, body)
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Authorization", "Bearer test-token")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("fileID", fileID)
	httpReq = httpReq.WithContext(context.WithValue(httpReq.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()

	handler.HandleChunkUpload(w, httpReq)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "not in uploading state")
}
