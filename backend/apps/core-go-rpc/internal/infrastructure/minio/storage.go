package minio

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	minioSDK "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Storage struct {
	client *minioSDK.Client
	bucket string
}

func New(ctx context.Context, endpoint, accessKey, secretKey string, useSSL bool, bucket string) (*Storage, error) {
	endpoint = strings.TrimSpace(endpoint)
	bucket = strings.TrimSpace(bucket)
	if endpoint == "" {
		return nil, errors.New("minio endpoint is required")
	}
	if accessKey == "" || secretKey == "" {
		return nil, errors.New("minio credentials are required")
	}
	if bucket == "" {
		return nil, errors.New("minio bucket is required")
	}

	client, err := minioSDK.New(endpoint, &minioSDK.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	storage := &Storage{client: client, bucket: bucket}
	if err := storage.ensureBucket(ctx); err != nil {
		return nil, err
	}
	return storage, nil
}

func (s *Storage) ensureBucket(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := s.client.MakeBucket(ctx, s.bucket, minioSDK.MakeBucketOptions{})
	if err == nil {
		return nil
	}

	exists, existsErr := s.client.BucketExists(ctx, s.bucket)
	if existsErr != nil {
		return fmt.Errorf("check minio bucket: %w", existsErr)
	}
	if !exists {
		return fmt.Errorf("create minio bucket failed: %w", err)
	}
	return nil
}

func (s *Storage) PutObject(ctx context.Context, key string, data []byte, contentType string) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("object key is required")
	}
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	_, err := s.client.PutObject(
		ctx,
		s.bucket,
		key,
		bytes.NewReader(data),
		int64(len(data)),
		minioSDK.PutObjectOptions{ContentType: contentType},
	)
	if err != nil {
		return fmt.Errorf("put object: %w", err)
	}
	return nil
}

func (s *Storage) GetObject(ctx context.Context, key string) ([]byte, string, error) {
	if strings.TrimSpace(key) == "" {
		return nil, "", errors.New("object key is required")
	}

	obj, err := s.client.GetObject(ctx, s.bucket, key, minioSDK.GetObjectOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("get object: %w", err)
	}
	defer obj.Close()

	stat, err := obj.Stat()
	if err != nil {
		return nil, "", fmt.Errorf("stat object: %w", err)
	}

	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, "", fmt.Errorf("read object: %w", err)
	}
	return data, stat.ContentType, nil
}

func (s *Storage) DeleteObject(ctx context.Context, key string) error {
	if strings.TrimSpace(key) == "" {
		return nil
	}
	if err := s.client.RemoveObject(ctx, s.bucket, key, minioSDK.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}
