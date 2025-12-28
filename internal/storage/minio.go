package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOClient struct {
	Client     *minio.Client
	BucketName string
}

func NewMinIOClient() (*MinIOClient, error) {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	accessKey := os.Getenv("MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MINIO_SECRET_KEY")
	useSSL := os.Getenv("MINIO_USE_SSL") == "true"
	bucketName := os.Getenv("MINIO_BUCKET_NAME")

	client, error := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})

	if error != nil {
		return nil, fmt.Errorf("failed to craete minio client: %w", error)
	}

	ctx := context.Background()
	exists, err := client.BucketExists(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket: %w", err)
	}

	if !exists {
		err = client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
		slog.Info("minio bucket created successfully",
			slog.String("bucket_name", bucketName),
		)
	}

	return &MinIOClient{
		Client:     client,
		BucketName: bucketName,
	}, nil
}

func (m MinIOClient) UploadFile(ctx context.Context, file io.Reader, fileID string, chunkIndex string, fileSize int64) (minio.UploadInfo, error) {
	uniqueFileName := fmt.Sprintf("%s::%s", fileID, chunkIndex)

	uploadInfo, err := m.Client.PutObject(ctx, m.BucketName, uniqueFileName, file, fileSize, minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		return minio.UploadInfo{}, fmt.Errorf("failed to upload chunk to MinIO: %w", err)
	}

	return uploadInfo, nil
}
