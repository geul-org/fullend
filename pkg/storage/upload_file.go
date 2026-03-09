package storage

import (
	"bytes"
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// @func uploadFile
// @description S3 호환 스토리지에 파일을 업로드한다

type UploadFileRequest struct {
	Bucket      string
	Key         string
	Data        []byte
	ContentType string
	Endpoint    string // MinIO 등 커스텀 엔드포인트 (빈 문자열이면 AWS 기본)
	Region      string
}

type UploadFileResponse struct {
	URL string
}

func UploadFile(req UploadFileRequest) (UploadFileResponse, error) {
	client, err := newS3Client(req.Endpoint, req.Region)
	if err != nil {
		return UploadFileResponse{}, err
	}
	_, err = client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(req.Bucket),
		Key:         aws.String(req.Key),
		Body:        bytes.NewReader(req.Data),
		ContentType: aws.String(req.ContentType),
	})
	if err != nil {
		return UploadFileResponse{}, err
	}
	url := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", req.Bucket, req.Region, req.Key)
	if req.Endpoint != "" {
		url = fmt.Sprintf("%s/%s/%s", req.Endpoint, req.Bucket, req.Key)
	}
	return UploadFileResponse{URL: url}, nil
}
