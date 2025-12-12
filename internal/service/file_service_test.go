package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ilkin0/gzln/internal/api/types"
	"github.com/ilkin0/gzln/internal/repository/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockQuerier struct {
	mock.Mock
}

func (m *MockQuerier) CreateFile(ctx context.Context, arg sqlc.CreateFileParams) (sqlc.File, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.File), args.Error(1)
}

func (m *MockQuerier) GetFileByID(ctx context.Context, id pgtype.UUID) (sqlc.File, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(sqlc.File), args.Error(1)
}

func (m *MockQuerier) GetFileByShareID(ctx context.Context, shareID string) (sqlc.File, error) {
	args := m.Called(ctx, shareID)
	return args.Get(0).(sqlc.File), args.Error(1)
}

func (m *MockQuerier) UpdateFileStatus(ctx context.Context, arg sqlc.UpdateFileStatusParams) (sqlc.File, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.File), args.Error(1)
}

func (m *MockQuerier) IncrementDownloadCount(ctx context.Context, id pgtype.UUID) (sqlc.File, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(sqlc.File), args.Error(1)
}

func (m *MockQuerier) DeleteExpiredFiles(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func createValidRequest() types.InitUploadRequest {
	return types.InitUploadRequest{
		Salt:              "random-salt-value",
		EncryptedFilename: "encrypted-file-name",
		EncryptedMimeType: "encrypted-mime-type",
		TotalSize:         1024 * 1024, // 1MB
		ChunkCount:        10,
		ChunkSize:         1024,
		Pbkdf2Iterations:  100000,
	}
}

func TestInitFileUpload_Success(t *testing.T) {
	mockRepo := new(MockQuerier)
	service := NewFileService(mockRepo, nil)

	req := createValidRequest()
	ctx := context.Background()
	clientIP := "192.168.1.1"

	testFileID := pgtype.UUID{Valid: true}
	_ = testFileID.Scan("550e8400-e29b-41d4-a716-446655440000")

	mockRepo.On("CreateFile", ctx, mock.AnythingOfType("sqlc.CreateFileParams")).
		Return(sqlc.File{ID: testFileID}, nil)

	resp, err := service.InitFileUpload(ctx, req, clientIP)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.FileID)
	assert.NotEmpty(t, resp.ShareID)
	assert.NotEmpty(t, resp.UploadToken)
	assert.NotEmpty(t, resp.ExpiresAt)

	assert.Len(t, resp.ShareID, 12)

	mockRepo.AssertExpectations(t)
}

func TestInitFileUpload_WithDefaults(t *testing.T) {
	mockRepo := new(MockQuerier)
	service := NewFileService(mockRepo, nil)

	req := createValidRequest()
	req.MaxDownloads = 0
	req.ExpiresInHours = 0

	ctx := context.Background()
	clientIP := "192.168.1.1"

	var capturedParams sqlc.CreateFileParams
	mockRepo.On("CreateFile", ctx, mock.AnythingOfType("sqlc.CreateFileParams")).
		Run(func(args mock.Arguments) {
			capturedParams = args.Get(1).(sqlc.CreateFileParams)
		}).
		Return(sqlc.File{}, nil)

	resp, err := service.InitFileUpload(ctx, req, clientIP)

	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, int32(100), capturedParams.MaxDownloads)

	expiryTime, parseErr := time.Parse(time.RFC3339, resp.ExpiresAt)
	require.NoError(t, parseErr)
	expectedExpiry := time.Now().Add(72 * time.Hour)
	assert.WithinDuration(t, expectedExpiry, expiryTime, 5*time.Second)

	mockRepo.AssertExpectations(t)
}

func TestInitFileUpload_CustomMaxDownloadsAndExpiry(t *testing.T) {
	mockRepo := new(MockQuerier)
	service := NewFileService(mockRepo, nil)

	req := createValidRequest()
	req.MaxDownloads = 5
	req.ExpiresInHours = 24

	ctx := context.Background()
	clientIP := "192.168.1.1"

	var capturedParams sqlc.CreateFileParams
	mockRepo.On("CreateFile", ctx, mock.AnythingOfType("sqlc.CreateFileParams")).
		Run(func(args mock.Arguments) {
			capturedParams = args.Get(1).(sqlc.CreateFileParams)
		}).
		Return(sqlc.File{}, nil)

	resp, err := service.InitFileUpload(ctx, req, clientIP)

	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, int32(5), capturedParams.MaxDownloads)

	expiryTime, parseErr := time.Parse(time.RFC3339, resp.ExpiresAt)
	require.NoError(t, parseErr)
	expectedExpiry := time.Now().Add(24 * time.Hour)
	assert.WithinDuration(t, expectedExpiry, expiryTime, 5*time.Second)

	mockRepo.AssertExpectations(t)
}

func TestInitFileUpload_InvalidIP(t *testing.T) {
	mockRepo := new(MockQuerier)
	service := NewFileService(mockRepo, nil)

	req := createValidRequest()
	ctx := context.Background()
	invalidIP := "invalid-ip-address"

	var capturedParams sqlc.CreateFileParams
	mockRepo.On("CreateFile", ctx, mock.AnythingOfType("sqlc.CreateFileParams")).
		Run(func(args mock.Arguments) {
			capturedParams = args.Get(1).(sqlc.CreateFileParams)
		}).
		Return(sqlc.File{}, nil)

	resp, err := service.InitFileUpload(ctx, req, invalidIP)

	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, "127.0.0.1", capturedParams.UploaderIp.String())

	mockRepo.AssertExpectations(t)
}

func TestInitFileUpload_RepositoryError(t *testing.T) {
	mockRepo := new(MockQuerier)
	service := NewFileService(mockRepo, nil)

	req := createValidRequest()
	ctx := context.Background()
	clientIP := "192.168.1.1"

	expectedErr := errors.New("database connection failed")
	mockRepo.On("CreateFile", ctx, mock.AnythingOfType("sqlc.CreateFileParams")).
		Return(sqlc.File{}, expectedErr)

	resp, err := service.InitFileUpload(ctx, req, clientIP)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to create file record")
	assert.Contains(t, err.Error(), "database connection failed")

	mockRepo.AssertExpectations(t)
}

func TestValidateUploadRequest(t *testing.T) {
	service := NewFileService(nil, nil)

	tests := []struct {
		name        string
		req         types.InitUploadRequest
		expectError string
	}{
		{
			name:        "missing salt",
			req:         func() types.InitUploadRequest { r := createValidRequest(); r.Salt = ""; return r }(),
			expectError: "salt is required",
		},
		{
			name:        "missing encrypted filename",
			req:         func() types.InitUploadRequest { r := createValidRequest(); r.EncryptedFilename = ""; return r }(),
			expectError: "encrypted_filename is required",
		},
		{
			name:        "missing encrypted mime type",
			req:         func() types.InitUploadRequest { r := createValidRequest(); r.EncryptedMimeType = ""; return r }(),
			expectError: "encrypted_mime_type is required",
		},
		{
			name:        "zero total size",
			req:         func() types.InitUploadRequest { r := createValidRequest(); r.TotalSize = 0; return r }(),
			expectError: "total_size must be positive",
		},
		{
			name:        "negative total size",
			req:         func() types.InitUploadRequest { r := createValidRequest(); r.TotalSize = -100; return r }(),
			expectError: "total_size must be positive",
		},
		{
			name:        "zero chunk count",
			req:         func() types.InitUploadRequest { r := createValidRequest(); r.ChunkCount = 0; return r }(),
			expectError: "chunk_count must be positive",
		},
		{
			name:        "negative chunk count",
			req:         func() types.InitUploadRequest { r := createValidRequest(); r.ChunkCount = -5; return r }(),
			expectError: "chunk_count must be positive",
		},
		{
			name:        "zero chunk size",
			req:         func() types.InitUploadRequest { r := createValidRequest(); r.ChunkSize = 0; return r }(),
			expectError: "chunk_size must be positive",
		},
		{
			name:        "negative chunk size",
			req:         func() types.InitUploadRequest { r := createValidRequest(); r.ChunkSize = -1024; return r }(),
			expectError: "chunk_size must be positive",
		},
		{
			name:        "zero pbkdf2 iterations",
			req:         func() types.InitUploadRequest { r := createValidRequest(); r.Pbkdf2Iterations = 0; return r }(),
			expectError: "pbkdf2_iterations must be positive",
		},
		{
			name:        "negative pbkdf2 iterations",
			req:         func() types.InitUploadRequest { r := createValidRequest(); r.Pbkdf2Iterations = -1000; return r }(),
			expectError: "pbkdf2_iterations must be positive",
		},
		{
			name:        "file size exceeds 5GB",
			req:         func() types.InitUploadRequest { r := createValidRequest(); r.TotalSize = 6 << 30; return r }(), // 6GB
			expectError: "file size",
		},
		{
			name:        "valid request",
			req:         createValidRequest(),
			expectError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateUploadRequest(tt.req)

			if tt.expectError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGetFileByShareID(t *testing.T) {
	mockRepo := new(MockQuerier)
	service := NewFileService(mockRepo, nil)

	ctx := context.Background()
	shareID := "test-share-id"
	expectedFile := sqlc.File{
		ShareID:           shareID,
		EncryptedFilename: "encrypted-name",
	}

	mockRepo.On("GetFileByShareID", ctx, shareID).
		Return(expectedFile, nil)

	result, err := service.GetFileByShareID(ctx, shareID)

	require.NoError(t, err)
	assert.Equal(t, expectedFile, result)
	mockRepo.AssertExpectations(t)
}

func TestGetFileByShareID_NotFound(t *testing.T) {
	mockRepo := new(MockQuerier)
	service := NewFileService(mockRepo, nil)

	ctx := context.Background()
	shareID := "non-existent"

	expectedErr := errors.New("no rows in result set")
	mockRepo.On("GetFileByShareID", ctx, shareID).
		Return(sqlc.File{}, expectedErr)

	result, err := service.GetFileByShareID(ctx, shareID)

	require.Error(t, err)
	assert.Equal(t, sqlc.File{}, result)
	assert.Contains(t, err.Error(), "no rows in result set")
	mockRepo.AssertExpectations(t)
}

func TestUpdateFileStatus(t *testing.T) {
	mockRepo := new(MockQuerier)
	service := NewFileService(mockRepo, nil)

	ctx := context.Background()
	fileID := pgtype.UUID{Valid: true}
	newStatus := "ready"
	expectedFile := sqlc.File{
		Status: newStatus,
	}

	mockRepo.On("UpdateFileStatus", ctx, mock.AnythingOfType("sqlc.UpdateFileStatusParams")).
		Return(expectedFile, nil)

	result, err := service.UpdateFileStatus(ctx, fileID, newStatus)

	require.NoError(t, err)
	assert.Equal(t, newStatus, result.Status)
	mockRepo.AssertExpectations(t)
}

func TestUpdateFileStatus_Error(t *testing.T) {
	mockRepo := new(MockQuerier)
	service := NewFileService(mockRepo, nil)

	ctx := context.Background()
	fileID := pgtype.UUID{Valid: true}
	newStatus := "ready"

	expectedErr := errors.New("update failed")
	mockRepo.On("UpdateFileStatus", ctx, mock.AnythingOfType("sqlc.UpdateFileStatusParams")).
		Return(sqlc.File{}, expectedErr)

	result, err := service.UpdateFileStatus(ctx, fileID, newStatus)

	require.Error(t, err)
	assert.Equal(t, sqlc.File{}, result)
	assert.Contains(t, err.Error(), "update failed")
	mockRepo.AssertExpectations(t)
}

func TestGenerateShareID(t *testing.T) {
	shareID1 := generateShareID()
	assert.Len(t, shareID1, 12)

	shareID2 := generateShareID()
	assert.Len(t, shareID2, 12)
	assert.NotEqual(t, shareID1, shareID2)

	validChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for _, char := range shareID1 {
		assert.Contains(t, validChars, string(char))
	}
}
