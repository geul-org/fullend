package file

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// FileModel provides file/object storage operations.
type FileModel interface {
	Upload(ctx context.Context, key string, body io.Reader) error
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

// --- S3 implementation ---

type s3File struct {
	client *s3.Client
	bucket string
}

// NewS3File creates a FileModel backed by AWS S3.
func NewS3File(client *s3.Client, bucket string) FileModel {
	return &s3File{client: client, bucket: bucket}
}

func (f *s3File) Upload(ctx context.Context, key string, body io.Reader) error {
	_, err := f.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(f.bucket),
		Key:    aws.String(key),
		Body:   body,
	})
	return err
}

func (f *s3File) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := f.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(f.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

func (f *s3File) Delete(ctx context.Context, key string) error {
	_, err := f.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(f.bucket),
		Key:    aws.String(key),
	})
	return err
}

// --- LocalFile implementation ---

type localFile struct {
	root string
}

// NewLocalFile creates a FileModel backed by the local filesystem.
func NewLocalFile(root string) FileModel {
	return &localFile{root: root}
}

func (f *localFile) Upload(_ context.Context, key string, body io.Reader) error {
	path := filepath.Join(f.root, key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, body)
	return err
}

func (f *localFile) Download(_ context.Context, key string) (io.ReadCloser, error) {
	path := filepath.Join(f.root, key)
	return os.Open(path)
}

func (f *localFile) Delete(_ context.Context, key string) error {
	path := filepath.Join(f.root, key)
	return os.Remove(path)
}
