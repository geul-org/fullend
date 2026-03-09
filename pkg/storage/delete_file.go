package storage

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// @func deleteFile
// @description S3 호환 스토리지에서 파일을 삭제한다

type DeleteFileRequest struct {
	Bucket   string
	Key      string
	Endpoint string
	Region   string
}

type DeleteFileResponse struct{}

func DeleteFile(req DeleteFileRequest) (DeleteFileResponse, error) {
	client, err := newS3Client(req.Endpoint, req.Region)
	if err != nil {
		return DeleteFileResponse{}, err
	}
	_, err = client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(req.Bucket),
		Key:    aws.String(req.Key),
	})
	return DeleteFileResponse{}, err
}
