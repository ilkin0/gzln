package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ilkin0/gzln/internal/repository/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockCleanupQuerier struct {
	mock.Mock
}

func (m *MockCleanupQuerier) GetExpiredFiles(ctx context.Context) ([]sqlc.GetExpiredFilesRow, error) {
	args := m.Called(ctx)
	return args.Get(0).([]sqlc.GetExpiredFilesRow), args.Error(1)
}

func (m *MockCleanupQuerier) ExpireFilesByIds(ctx context.Context, ids []pgtype.UUID) error {
	args := m.Called(ctx, ids)
	return args.Error(0)
}

func createTestUUIDFrom(t *testing.T, uuidStr string) pgtype.UUID {
	t.Helper()
	var id pgtype.UUID
	err := id.Scan(uuidStr)
	require.NoError(t, err)
	return id
}

func TestCleanupExpiredFiles_NoExpiredFiles(t *testing.T) {
	mockQueries := new(MockCleanupQuerier)
	ctx := context.Background()

	mockQueries.On("GetExpiredFiles", ctx).
		Return([]sqlc.GetExpiredFilesRow{}, nil)

	expiredFiles, err := mockQueries.GetExpiredFiles(ctx)

	require.NoError(t, err)
	assert.Len(t, expiredFiles, 0)
	mockQueries.AssertExpectations(t)
}

func TestCleanupExpiredFiles_GetExpiredFilesError(t *testing.T) {
	mockQueries := new(MockCleanupQuerier)
	ctx := context.Background()

	expectedErr := errors.New("database connection failed")
	mockQueries.On("GetExpiredFiles", ctx).
		Return([]sqlc.GetExpiredFilesRow{}, expectedErr)

	_, err := mockQueries.GetExpiredFiles(ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "database connection failed")
	mockQueries.AssertExpectations(t)
}

func TestCleanupExpiredFiles_ExpireFilesByIdsError(t *testing.T) {
	mockQueries := new(MockCleanupQuerier)
	ctx := context.Background()

	testIDs := []pgtype.UUID{
		createTestUUIDFrom(t, "550e8400-e29b-41d4-a716-446655440001"),
		createTestUUIDFrom(t, "550e8400-e29b-41d4-a716-446655440002"),
	}

	expectedErr := errors.New("failed to update files")
	mockQueries.On("ExpireFilesByIds", ctx, testIDs).
		Return(expectedErr)

	err := mockQueries.ExpireFilesByIds(ctx, testIDs)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update files")
	mockQueries.AssertExpectations(t)
}

func TestCleanupExpiredFiles_ExpireFilesByIdsSuccess(t *testing.T) {
	mockQueries := new(MockCleanupQuerier)
	ctx := context.Background()

	testIDs := []pgtype.UUID{
		createTestUUIDFrom(t, "550e8400-e29b-41d4-a716-446655440001"),
		createTestUUIDFrom(t, "550e8400-e29b-41d4-a716-446655440002"),
	}

	mockQueries.On("ExpireFilesByIds", ctx, testIDs).
		Return(nil)

	err := mockQueries.ExpireFilesByIds(ctx, testIDs)

	require.NoError(t, err)
	mockQueries.AssertExpectations(t)
}

func TestDeleteFileChunks_GeneratesCorrectObjectNames(t *testing.T) {
	fileID := "550e8400-e29b-41d4-a716-446655440001"
	chunkCount := int32(5)

	expectedObjects := []string{
		"550e8400-e29b-41d4-a716-446655440001/0.enc",
		"550e8400-e29b-41d4-a716-446655440001/1.enc",
		"550e8400-e29b-41d4-a716-446655440001/2.enc",
		"550e8400-e29b-41d4-a716-446655440001/3.enc",
		"550e8400-e29b-41d4-a716-446655440001/4.enc",
	}

	var generatedObjects []string
	for i := int32(0); i < chunkCount; i++ {
		objectName := fmt.Sprintf("%s/%d.enc", fileID, i)
		generatedObjects = append(generatedObjects, objectName)
	}

	assert.Equal(t, expectedObjects, generatedObjects)
}

func TestCollectExpiredFileIds(t *testing.T) {
	expiredFiles := []sqlc.GetExpiredFilesRow{
		{ID: createTestUUIDFrom(t, "550e8400-e29b-41d4-a716-446655440001"), ChunkCount: 5},
		{ID: createTestUUIDFrom(t, "550e8400-e29b-41d4-a716-446655440002"), ChunkCount: 3},
		{ID: createTestUUIDFrom(t, "550e8400-e29b-41d4-a716-446655440003"), ChunkCount: 10},
	}

	expiredIds := make([]pgtype.UUID, len(expiredFiles))
	for i, file := range expiredFiles {
		expiredIds[i] = file.ID
	}

	assert.Len(t, expiredIds, 3)
	assert.Equal(t, expiredFiles[0].ID, expiredIds[0])
	assert.Equal(t, expiredFiles[1].ID, expiredIds[1])
	assert.Equal(t, expiredFiles[2].ID, expiredIds[2])
}
