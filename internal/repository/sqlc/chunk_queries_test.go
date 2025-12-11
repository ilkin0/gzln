package sqlc

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestFileForChunks(t *testing.T, ctx context.Context) File {
	shareID := fmt.Sprintf("tst%09d", time.Now().UnixNano()%1000000000)
	params := createTestFileParams(shareID)
	file, err := testQueries.CreateFile(ctx, params)
	require.NoError(t, err)
	return file
}

func createTestChunkParams(fileID pgtype.UUID, chunkIndex int32) CreateChunkParams {
	return CreateChunkParams{
		FileID:        fileID,
		ChunkIndex:    chunkIndex,
		StoragePath:   "test/path/chunk.enc",
		EncryptedSize: 1024,
		ChunkHash:     "test-hash-value",
	}
}

func TestCreateChunk_Success(t *testing.T) {
	if testQueries == nil {
		t.Skip("Database not available")
	}

	ctx := context.Background()
	cleanupFiles(t)

	file := createTestFileForChunks(t, ctx)

	params := createTestChunkParams(file.ID, 0)
	chunkID, err := testQueries.CreateChunk(ctx, params)

	require.NoError(t, err)
	assert.Greater(t, chunkID, int64(0))
}

func TestCreateChunk_DuplicateChunkIndex(t *testing.T) {
	if testQueries == nil {
		t.Skip("Database not available")
	}

	ctx := context.Background()
	cleanupFiles(t)

	file := createTestFileForChunks(t, ctx)

	params := createTestChunkParams(file.ID, 0)
	_, err := testQueries.CreateChunk(ctx, params)
	require.NoError(t, err)

	_, err = testQueries.CreateChunk(ctx, params)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unique")
}

func TestCreateChunk_InvalidFileID(t *testing.T) {
	if testQueries == nil {
		t.Skip("Database not available")
	}

	ctx := context.Background()
	cleanupFiles(t)

	nonExistentFileID := pgtype.UUID{Valid: true}
	_ = nonExistentFileID.Scan("550e8400-e29b-41d4-a716-446655440000")

	params := createTestChunkParams(nonExistentFileID, 0)
	_, err := testQueries.CreateChunk(ctx, params)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "foreign key")
}

func TestChunkExistsByFileIdAndIndex_Exists(t *testing.T) {
	if testQueries == nil {
		t.Skip("Database not available")
	}

	ctx := context.Background()
	cleanupFiles(t)

	file := createTestFileForChunks(t, ctx)
	params := createTestChunkParams(file.ID, 0)
	_, err := testQueries.CreateChunk(ctx, params)
	require.NoError(t, err)

	// Query for the chunk
	exists, err := testQueries.ChunkExistsByFileIdAndIndex(ctx, ChunkExistsByFileIdAndIndexParams{
		FileID:     file.ID,
		ChunkIndex: 0,
	})

	require.NoError(t, err)
	assert.True(t, exists)
}

func TestChunkExistsByFileIdAndIndex_NotExists(t *testing.T) {
	if testQueries == nil {
		t.Skip("Database not available")
	}

	ctx := context.Background()
	cleanupFiles(t)

	file := createTestFileForChunks(t, ctx)

	exists, err := testQueries.ChunkExistsByFileIdAndIndex(ctx, ChunkExistsByFileIdAndIndexParams{
		FileID:     file.ID,
		ChunkIndex: 999,
	})

	require.NoError(t, err)
	assert.False(t, exists)
}

func TestFileExistsByIdAndStatus_UploadingStatus(t *testing.T) {
	if testQueries == nil {
		t.Skip("Database not available")
	}

	ctx := context.Background()
	cleanupFiles(t)

	file := createTestFileForChunks(t, ctx)

	exists, err := testQueries.FileExistsByIdAndStatus(ctx, FileExistsByIdAndStatusParams{
		ID:     file.ID,
		Status: "uploading",
	})

	require.NoError(t, err)
	assert.True(t, exists)
}

func TestFileExistsByIdAndStatus_WrongStatus(t *testing.T) {
	if testQueries == nil {
		t.Skip("Database not available")
	}

	ctx := context.Background()
	cleanupFiles(t)

	file := createTestFileForChunks(t, ctx)

	_, err := testQueries.UpdateFileStatus(ctx, UpdateFileStatusParams{
		ID:     file.ID,
		Status: "ready",
	})
	require.NoError(t, err)

	exists, err := testQueries.FileExistsByIdAndStatus(ctx, FileExistsByIdAndStatusParams{
		ID:     file.ID,
		Status: "uploading",
	})

	require.NoError(t, err)
	assert.False(t, exists)
}

func TestFileExistsByIdAndStatus_NotFound(t *testing.T) {
	if testQueries == nil {
		t.Skip("Database not available")
	}

	ctx := context.Background()
	cleanupFiles(t)

	nonExistentFileID := pgtype.UUID{Valid: true}
	_ = nonExistentFileID.Scan("550e8400-e29b-41d4-a716-446655440000")

	exists, err := testQueries.FileExistsByIdAndStatus(ctx, FileExistsByIdAndStatusParams{
		ID:     nonExistentFileID,
		Status: "uploading",
	})

	require.NoError(t, err)
	assert.False(t, exists)
}

func TestCascadeDelete(t *testing.T) {
	if testQueries == nil {
		t.Skip("Database not available")
	}

	ctx := context.Background()
	cleanupFiles(t)

	file := createTestFileForChunks(t, ctx)

	for i := 0; i < 3; i++ {
		params := createTestChunkParams(file.ID, int32(i))
		_, err := testQueries.CreateChunk(ctx, params)
		require.NoError(t, err)
	}

	for i := 0; i < 3; i++ {
		exists, err := testQueries.ChunkExistsByFileIdAndIndex(ctx, ChunkExistsByFileIdAndIndexParams{
			FileID:     file.ID,
			ChunkIndex: int32(i),
		})
		require.NoError(t, err)
		assert.True(t, exists)
	}

	_, err := testPool.Exec(ctx, "DELETE FROM files WHERE id = $1", file.ID)
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		exists, err := testQueries.ChunkExistsByFileIdAndIndex(ctx, ChunkExistsByFileIdAndIndexParams{
			FileID:     file.ID,
			ChunkIndex: int32(i),
		})
		require.NoError(t, err)
		assert.False(t, exists, "Chunk %d should be deleted via CASCADE", i)
	}
}

func TestCreateChunk_VerifyAllFields(t *testing.T) {
	if testQueries == nil {
		t.Skip("Database not available")
	}

	ctx := context.Background()
	cleanupFiles(t)

	file := createTestFileForChunks(t, ctx)

	params := createTestChunkParams(file.ID, 5)
	params.StoragePath = "custom/storage/path/chunk-5.enc"
	params.EncryptedSize = 2048
	params.ChunkHash = "abc123def456"

	chunkID, err := testQueries.CreateChunk(ctx, params)
	require.NoError(t, err)

	exists, err := testQueries.ChunkExistsByFileIdAndIndex(ctx, ChunkExistsByFileIdAndIndexParams{
		FileID:     file.ID,
		ChunkIndex: 5,
	})
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Greater(t, chunkID, int64(0))
}
