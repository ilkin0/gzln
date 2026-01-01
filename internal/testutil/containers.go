package testutil

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ilkin0/gzln/internal/database"
	"github.com/ilkin0/gzln/internal/storage"
	"github.com/minio/minio-go/v7"
	"github.com/testcontainers/testcontainers-go"
	miniocontainer "github.com/testcontainers/testcontainers-go/modules/minio"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type TestContainers struct {
	PostgresContainer *postgres.PostgresContainer
	MinioContainer    *miniocontainer.MinioContainer
	Database          *database.Database
	MinioClient       *storage.MinIOClient
	Cleanup           func()
}

func SetupTestContainers(t *testing.T) *TestContainers {
	t.Helper()

	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("gzln_test"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("Failed to start postgres container: %v", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to get connection string: %v", err)
	}

	t.Setenv("DB_URL", connStr)

	db, err := database.NewDatabase(ctx)
	if err != nil {
		pgContainer.Terminate(ctx)
		t.Fatalf("Failed to initialize database: %v", err)
	}

	if err := runMigrations(); err != nil {
		db.Pool.Close()
		pgContainer.Terminate(ctx)
		t.Fatalf("Failed to run migrations: %v", err)
	}

	minioContainer, err := miniocontainer.Run(ctx,
		"minio/minio:latest",
		miniocontainer.WithUsername("minioadmin"),
		miniocontainer.WithPassword("minioadmin"),
	)
	if err != nil {
		db.Pool.Close()
		pgContainer.Terminate(ctx)
		t.Fatalf("Failed to start minio container: %v", err)
	}

	minioEndpoint, err := minioContainer.ConnectionString(ctx)
	if err != nil {
		db.Pool.Close()
		pgContainer.Terminate(ctx)
		minioContainer.Terminate(ctx)
		t.Fatalf("Failed to get minio endpoint: %v", err)
	}

	t.Setenv("MINIO_ENDPOINT", minioEndpoint)
	t.Setenv("MINIO_ACCESS_KEY", "minioadmin")
	t.Setenv("MINIO_SECRET_KEY", "minioadmin")
	t.Setenv("MINIO_BUCKET_NAME", "gzln-test")
	t.Setenv("MINIO_USE_SSL", "false")

	minioClient, err := storage.NewMinIOClient()
	if err != nil {
		db.Pool.Close()
		pgContainer.Terminate(ctx)
		minioContainer.Terminate(ctx)
		t.Fatalf("Failed to initialize MinIO client: %v", err)
	}

	cleanup := func() {
		db.Pool.Exec(ctx, "TRUNCATE TABLE files CASCADE")

		objectsCh := minioClient.Client.ListObjects(ctx, minioClient.BucketName, minio.ListObjectsOptions{
			Recursive: true,
		})
		for object := range objectsCh {
			if object.Err != nil {
				continue
			}
			minioClient.Client.RemoveObject(ctx, minioClient.BucketName, object.Key, minio.RemoveObjectOptions{})
		}

		db.Pool.Close()

		pgContainer.Terminate(ctx)
		minioContainer.Terminate(ctx)
	}

	return &TestContainers{
		PostgresContainer: pgContainer,
		MinioContainer:    minioContainer,
		Database:          db,
		MinioClient:       minioClient,
		Cleanup:           cleanup,
	}
}

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find project root (go.mod)")
		}
		dir = parent
	}
}

func runMigrations() error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}

	migrationDir := filepath.Join(projectRoot, "db/migration")
	dbURL := os.Getenv("DB_URL")

	cmd := exec.Command("goose", "-dir", migrationDir, "postgres", dbURL, "up")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run goose migrations: %w", err)
	}

	return nil
}

func CleanDatabase(ctx context.Context, db *database.Database) {
	db.Pool.Exec(ctx, "TRUNCATE TABLE files CASCADE")
}

func CleanMinIO(ctx context.Context, minioClient *storage.MinIOClient) {
	objectsCh := minioClient.Client.ListObjects(ctx, minioClient.BucketName, minio.ListObjectsOptions{
		Recursive: true,
	})
	for object := range objectsCh {
		if object.Err != nil {
			continue
		}
		minioClient.Client.RemoveObject(ctx, minioClient.BucketName, object.Key, minio.RemoveObjectOptions{})
	}
}
