package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ilkin0/gzln/internal/api/types"
	"github.com/ilkin0/gzln/internal/database"
	"github.com/ilkin0/gzln/internal/middleware"
	"github.com/ilkin0/gzln/internal/repository/sqlc"
	"github.com/ilkin0/gzln/internal/service"
	"github.com/ilkin0/gzln/internal/testutil"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRateLimitTest(t *testing.T) (chi.Router, *database.Database, func()) {
	t.Helper()

	t.Setenv("RATE_LIMIT_UPLOAD_INIT", "3")
	t.Setenv("RATE_LIMIT_CHUNK_UPLOAD", "5")
	t.Setenv("RATE_LIMIT_UPLOAD_FINALIZE", "3")
	t.Setenv("RATE_LIMIT_METADATA", "4")
	t.Setenv("RATE_LIMIT_CHUNK_DOWNLOAD", "6")
	t.Setenv("RATE_LIMIT_DOWNLOAD_COMPLETE", "3")
	t.Setenv("RATE_LIMIT_WINDOW_SECONDS", "2") // 2 second window for testing

	middleware.ReloadConfig()

	containers := testutil.SetupTestContainers(t)

	runTx := database.NewTxRunner(containers.Database.Pool)
	fileService := service.NewFileService(containers.Database.Queries, runTx, containers.MinioClient.Client)
	chunkService := service.NewChunkService(containers.Database.Queries, containers.MinioClient.Client, containers.MinioClient.BucketName)

	r := chi.NewRouter()
	r.Mount("/api/v1/files", FileRoutes(fileService, chunkService, containers.MinioClient.BucketName))
	r.Mount("/api/v1/download", DownloadRoutes(fileService, chunkService, containers.MinioClient.BucketName))

	return r, containers.Database, containers.Cleanup
}

func TestRateLimit_UploadInit(t *testing.T) {
	r, db, cleanup := setupRateLimitTest(t)
	defer cleanup()

	ctx := context.Background()
	defer db.Pool.Exec(ctx, "TRUNCATE TABLE files CASCADE")

	createRequest := func() *http.Request {
		reqBody := types.InitUploadRequest{
			EncryptedFilename: "test.enc",
			EncryptedMimeType: "application/octet-stream",
			Salt:              fmt.Sprintf("salt%d", time.Now().UnixNano()%1000000),
			Pbkdf2Iterations:  100000,
			TotalSize:         1024,
			ChunkCount:        1,
			ChunkSize:         1024,
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/v1/files/upload/init", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.0.2.1:1234" // Same IP for rate limiting
		return req
	}

	for i := 0; i < 3; i++ {
		req := createRequest()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
		time.Sleep(10 * time.Millisecond) // Small delay to ensure requests are in the same window
	}

	req := createRequest()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code, "Request 4 should be rate limited")

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.False(t, response["success"].(bool))
	assert.Contains(t, response["message"], "Rate limit exceeded")
	assert.NotEmpty(t, w.Header().Get("Retry-After"))
}

func TestRateLimit_ChunkUpload(t *testing.T) {
	r, db, cleanup := setupRateLimitTest(t)
	defer cleanup()

	ctx := context.Background()
	defer db.Pool.Exec(ctx, "TRUNCATE TABLE files CASCADE")

	file, err := db.Queries.CreateFile(ctx, sqlc.CreateFileParams{
		ShareID:           "testshare",
		EncryptedFilename: "test.enc",
		EncryptedMimeType: "application/octet-stream",
		Salt:              "test-salt",
		Pbkdf2Iterations:  100000,
		TotalSize:         1024,
		ChunkCount:        10,
		ChunkSize:         1024,
		ExpiresAt:         pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
		MaxDownloads:      5,
		UploaderIp:        netip.MustParseAddr("192.0.2.1"),
	})
	require.NoError(t, err)

	createRequest := func(chunkIndex int) *http.Request {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		part, _ := writer.CreateFormFile("chunk", "chunk.enc")
		io.WriteString(part, "test chunk data")

		writer.WriteField("chunk_index", fmt.Sprintf("%d", chunkIndex))
		writer.WriteField("hash", "34fa0947d659ce6343cbfe6be3a1ca882f6b21b35232210f194791d545440c40")
		writer.Close()

		req := httptest.NewRequest("POST", "/api/v1/files/"+file.ID.String()+"/chunks", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer test-token")
		req.RemoteAddr = "192.0.2.1:1234"
		return req
	}

	for i := 0; i < 5; i++ {
		req := createRequest(i)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
	}

	req := createRequest(5)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code, "Request 6 should be rate limited")
}

func TestRateLimit_Metadata(t *testing.T) {
	r, db, cleanup := setupRateLimitTest(t)
	defer cleanup()

	ctx := context.Background()
	defer db.Pool.Exec(ctx, "TRUNCATE TABLE files CASCADE")

	file, err := db.Queries.CreateFile(ctx, sqlc.CreateFileParams{
		ShareID:           "meta-test-id",
		EncryptedFilename: "test.enc",
		EncryptedMimeType: "application/octet-stream",
		Salt:              "test-salt",
		Pbkdf2Iterations:  100000,
		TotalSize:         1024,
		ChunkCount:        1,
		ChunkSize:         1024,
		ExpiresAt:         pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
		MaxDownloads:      5,
		UploaderIp:        netip.MustParseAddr("192.0.2.1"),
	})
	require.NoError(t, err)

	_, err = db.Queries.UpdateFileStatus(ctx, sqlc.UpdateFileStatusParams{
		ID:     file.ID,
		Status: "ready",
	})
	require.NoError(t, err)

	createRequest := func() *http.Request {
		req := httptest.NewRequest("GET", "/api/v1/download/"+file.ShareID+"/metadata", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		return req
	}

	for i := 0; i < 4; i++ {
		req := createRequest()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
	}

	req := createRequest()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code, "Request 5 should be rate limited")
}

func TestRateLimit_ChunkDownload(t *testing.T) {
	r, db, cleanup := setupRateLimitTest(t)
	defer cleanup()

	ctx := context.Background()
	defer db.Pool.Exec(ctx, "TRUNCATE TABLE files CASCADE")

	file, err := db.Queries.CreateFile(ctx, sqlc.CreateFileParams{
		ShareID:           "down-test-id",
		EncryptedFilename: "test.enc",
		EncryptedMimeType: "application/octet-stream",
		Salt:              "test-salt",
		Pbkdf2Iterations:  100000,
		TotalSize:         1024,
		ChunkCount:        1,
		ChunkSize:         1024,
		ExpiresAt:         pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
		MaxDownloads:      100, // High limit to prevent download limit from interfering
		UploaderIp:        netip.MustParseAddr("192.0.2.1"),
	})
	require.NoError(t, err)

	_, err = db.Queries.UpdateFileStatus(ctx, sqlc.UpdateFileStatusParams{
		ID:     file.ID,
		Status: "ready",
	})
	require.NoError(t, err)

	_, err = db.Queries.CreateChunk(ctx, sqlc.CreateChunkParams{
		FileID:        file.ID,
		ChunkIndex:    0,
		StoragePath:   file.ID.String() + "::0",
		EncryptedSize: 1024,
		ChunkHash:     "test-hash",
	})
	require.NoError(t, err)

	createRequest := func() *http.Request {
		req := httptest.NewRequest("GET", "/api/v1/download/"+file.ShareID+"/chunks/0", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		return req
	}

	for i := 0; i < 6; i++ {
		req := createRequest()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.NotEqual(t, http.StatusTooManyRequests, w.Code, "Request %d should not be rate limited", i+1)
	}

	req := createRequest()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code, "Request 7 should be rate limited")
}

func TestRateLimit_DifferentIPsNotAffected(t *testing.T) {
	r, db, cleanup := setupRateLimitTest(t)
	defer cleanup()

	ctx := context.Background()
	defer db.Pool.Exec(ctx, "TRUNCATE TABLE files CASCADE")

	createRequest := func(ip string) *http.Request {
		reqBody := types.InitUploadRequest{
			EncryptedFilename: "test.enc",
			EncryptedMimeType: "application/octet-stream",
			Salt:              fmt.Sprintf("salt%d", time.Now().UnixNano()%1000000),
			Pbkdf2Iterations:  100000,
			TotalSize:         1024,
			ChunkCount:        1,
			ChunkSize:         1024,
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/v1/files/upload/init", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = ip + ":1234"
		return req
	}

	for i := 0; i < 3; i++ {
		req := createRequest("192.0.2.1")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	req := createRequest("192.0.2.1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	req = createRequest("192.0.2.2")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "Different IP should not be affected by rate limit")
}

func TestRateLimit_ResetAfterWindow(t *testing.T) {
	r, db, cleanup := setupRateLimitTest(t)
	defer cleanup()

	ctx := context.Background()
	defer db.Pool.Exec(ctx, "TRUNCATE TABLE files CASCADE")

	createRequest := func() *http.Request {
		reqBody := types.InitUploadRequest{
			EncryptedFilename: "test.enc",
			EncryptedMimeType: "application/octet-stream",
			Salt:              fmt.Sprintf("salt%d", time.Now().UnixNano()%1000000), // Keep under 24 chars
			Pbkdf2Iterations:  100000,
			TotalSize:         1024,
			ChunkCount:        1,
			ChunkSize:         1024,
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/v1/files/upload/init", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.0.2.1:1234"
		return req
	}

	for i := 0; i < 3; i++ {
		req := createRequest()
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	req := createRequest()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	time.Sleep(2500 * time.Millisecond)

	req = createRequest()
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "Rate limit should reset after time window")
}
