package service

import (
	"context"
	"errors"
	"testing"

	"github.com/ilkin0/gzln/internal/api/types"
	"github.com/ilkin0/gzln/internal/repository/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func (m *MockQuerier) ChunkExistsByFileIdAndIndex(ctx context.Context, arg sqlc.ChunkExistsByFileIdAndIndexParams) (bool, error) {
	args := m.Called(ctx, arg)
	return args.Bool(0), args.Error(1)
}

func (m *MockQuerier) CreateChunk(ctx context.Context, arg sqlc.CreateChunkParams) (int64, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockQuerier) FileExistsByIdAndStatus(ctx context.Context, arg sqlc.FileExistsByIdAndStatusParams) (bool, error) {
	args := m.Called(ctx, arg)
	return args.Bool(0), args.Error(1)
}

func (m *MockQuerier) GetChunkByIndexAndFileShareID(ctx context.Context, arg sqlc.GetChunkByIndexAndFileShareIDParams) (sqlc.GetChunkByIndexAndFileShareIDRow, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.GetChunkByIndexAndFileShareIDRow), args.Error(1)
}

func createTestUUID() pgtype.UUID {
	uuid := pgtype.UUID{}
	_ = uuid.Scan("550e8400-e29b-41d4-a716-446655440000")
	return uuid
}

func createValidChunkRequest() types.ChunkUploadRequest {
	return types.ChunkUploadRequest{
		FileID:       createTestUUID(),
		ChunkIndex:   0,
		ChunkData:    []byte("test chunk data"),
		ExpectedHash: "34fa0947d659ce6343cbfe6be3a1ca882f6b21b35232210f194791d545440c40", // SHA256 of "test chunk data"
		ContentType:  "application/octet-stream",
		Filename:     "test.txt",
	}
}

func TestProcessChunkUpload_Success(t *testing.T) {
	t.Skip("Skipping MinIO integration test - requires actual MinIO instance or testcontainers")
}

func TestProcessChunkUpload_ChunkAlreadyExists(t *testing.T) {
	mockRepo := new(MockQuerier)
	service := NewChunkService(mockRepo, nil, "test-bucket")
	ctx := context.Background()
	req := createValidChunkRequest()

	mockRepo.On("ChunkExistsByFileIdAndIndex", ctx, mock.AnythingOfType("sqlc.ChunkExistsByFileIdAndIndexParams")).
		Return(true, nil)

	result, err := service.ProcessChunkUpload(ctx, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "already uploaded")
	assert.Equal(t, types.ChunkUploadResponse{}, result)

	mockRepo.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "FileExistsByIdAndStatus")
	mockRepo.AssertNotCalled(t, "CreateChunk")
}

func TestProcessChunkUpload_FileNotFound(t *testing.T) {
	mockRepo := new(MockQuerier)
	service := NewChunkService(mockRepo, nil, "test-bucket")
	ctx := context.Background()
	req := createValidChunkRequest()

	mockRepo.On("ChunkExistsByFileIdAndIndex", ctx, mock.AnythingOfType("sqlc.ChunkExistsByFileIdAndIndexParams")).
		Return(false, nil)

	mockRepo.On("FileExistsByIdAndStatus", ctx, mock.AnythingOfType("sqlc.FileExistsByIdAndStatusParams")).
		Return(false, nil)

	result, err := service.ProcessChunkUpload(ctx, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist or is not in uploading state")
	assert.Equal(t, types.ChunkUploadResponse{}, result)

	mockRepo.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "CreateChunk")
}

func TestProcessChunkUpload_HashMismatch(t *testing.T) {
	mockRepo := new(MockQuerier)
	service := NewChunkService(mockRepo, nil, "test-bucket")
	ctx := context.Background()
	req := createValidChunkRequest()
	req.ExpectedHash = "wrong-hash-value"

	mockRepo.On("ChunkExistsByFileIdAndIndex", ctx, mock.AnythingOfType("sqlc.ChunkExistsByFileIdAndIndexParams")).
		Return(false, nil)

	mockRepo.On("FileExistsByIdAndStatus", ctx, mock.AnythingOfType("sqlc.FileExistsByIdAndStatusParams")).
		Return(true, nil)

	result, err := service.ProcessChunkUpload(ctx, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "hash mismatch")
	assert.Equal(t, types.ChunkUploadResponse{}, result)

	mockRepo.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "CreateChunk")
}

func TestProcessChunkUpload_DatabaseFailure(t *testing.T) {
	mockRepo := new(MockQuerier)
	service := NewChunkService(mockRepo, nil, "test-bucket")
	ctx := context.Background()
	req := createValidChunkRequest()

	mockRepo.On("ChunkExistsByFileIdAndIndex", ctx, mock.AnythingOfType("sqlc.ChunkExistsByFileIdAndIndexParams")).
		Return(false, nil)

	mockRepo.On("FileExistsByIdAndStatus", ctx, mock.AnythingOfType("sqlc.FileExistsByIdAndStatusParams")).
		Return(false, errors.New("database connection error"))

	result, err := service.ProcessChunkUpload(ctx, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to verify file status")
	assert.Equal(t, types.ChunkUploadResponse{}, result)

	mockRepo.AssertExpectations(t)
}

func TestValidateChunkHash_Success(t *testing.T) {
	service := &ChunkService{}

	data := []byte("test chunk data")
	expectedHash := "34fa0947d659ce6343cbfe6be3a1ca882f6b21b35232210f194791d545440c40"

	err := service.validateChunkHash(data, expectedHash)

	assert.NoError(t, err)
}

func TestValidateChunkHash_Mismatch(t *testing.T) {
	service := &ChunkService{}

	data := []byte("test chunk data")
	wrongHash := "wrong-hash-value"

	err := service.validateChunkHash(data, wrongHash)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "hash mismatch")
}

func TestValidateChunkHash_EmptyData(t *testing.T) {
	service := &ChunkService{}

	data := []byte("")
	// SHA256 of empty string
	expectedHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	err := service.validateChunkHash(data, expectedHash)

	assert.NoError(t, err)
}

func TestValidateChunkUpload_ChunkExistsCheckError(t *testing.T) {
	mockRepo := new(MockQuerier)
	service := NewChunkService(mockRepo, nil, "test-bucket")
	ctx := context.Background()
	fileID := createTestUUID()

	mockRepo.On("ChunkExistsByFileIdAndIndex", ctx, mock.AnythingOfType("sqlc.ChunkExistsByFileIdAndIndexParams")).
		Return(false, errors.New("database error"))

	err := service.validateChunkUpload(ctx, fileID, 0)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check chunk existence")
	mockRepo.AssertExpectations(t)
}

func TestDownloadChunk_ChunkNotFound(t *testing.T) {
	mockRepo := new(MockQuerier)
	service := NewChunkService(mockRepo, nil, "test-bucket")

	ctx := context.Background()
	shareID := "abc123def456"
	chunkIndex := int64(0)

	expectedErr := errors.New("no rows in result set")
	mockRepo.On("GetChunkByIndexAndFileShareID", ctx, mock.AnythingOfType("sqlc.GetChunkByIndexAndFileShareIDParams")).
		Return(sqlc.GetChunkByIndexAndFileShareIDRow{}, expectedErr)

	result, err := service.DownloadChunk(ctx, shareID, chunkIndex)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get chunk storage path")
	mockRepo.AssertExpectations(t)
}

func TestDownloadChunk_DownloadLimitReached(t *testing.T) {
	mockRepo := new(MockQuerier)
	service := NewChunkService(mockRepo, nil, "test-bucket")

	ctx := context.Background()
	shareID := "abc123def456"
	chunkIndex := int64(0)

	chunkDetails := sqlc.GetChunkByIndexAndFileShareIDRow{
		StoragePath:   "file-id/0.enc",
		DownloadCount: 100,
		MaxDownloads:  100,
	}

	mockRepo.On("GetChunkByIndexAndFileShareID", ctx, mock.AnythingOfType("sqlc.GetChunkByIndexAndFileShareIDParams")).
		Return(chunkDetails, nil)

	result, err := service.DownloadChunk(ctx, shareID, chunkIndex)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "chunk download limit reached")
	mockRepo.AssertExpectations(t)
}

func TestDownloadChunk_Success(t *testing.T) {
	t.Skip("Skipping MinIO integration test - requires actual MinIO instance")
}

func TestDownloadChunk_ValidParams(t *testing.T) {
	t.Skip("Skipping MinIO integration test - requires actual MinIO instance")
}

func TestDownloadChunk_DifferentChunkIndices(t *testing.T) {
	t.Skip("Skipping MinIO integration test - requires actual MinIO instance")
}

func TestDownloadChunk_DownloadLimitEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		downloadCount int32
		maxDownloads  int32
		expectError   bool
	}{
		{
			name:          "at limit",
			downloadCount: 100,
			maxDownloads:  100,
			expectError:   true,
		},
		{
			name:          "over limit",
			downloadCount: 101,
			maxDownloads:  100,
			expectError:   true,
		},
		{
			name:          "one over limit",
			downloadCount: 1001,
			maxDownloads:  1000,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockQuerier)
			service := NewChunkService(mockRepo, nil, "test-bucket")
			ctx := context.Background()

			chunkDetails := sqlc.GetChunkByIndexAndFileShareIDRow{
				StoragePath:   "file-id/0.enc",
				DownloadCount: tt.downloadCount,
				MaxDownloads:  tt.maxDownloads,
			}

			mockRepo.On("GetChunkByIndexAndFileShareID", ctx, mock.AnythingOfType("sqlc.GetChunkByIndexAndFileShareIDParams")).
				Return(chunkDetails, nil)

			_, err := service.DownloadChunk(ctx, "test-share", 0)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "limit reached")
			mockRepo.AssertExpectations(t)
		})
	}
}
