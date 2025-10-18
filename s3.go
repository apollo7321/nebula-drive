package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type S3Client struct {
	ctx     context.Context
	client  *s3.Client
	timeout time.Duration
}

func NewS3Client(rootCtx context.Context, timeout time.Duration) (*S3Client, error) {
	ctx, cancel := context.WithTimeout(rootCtx, timeout)
	defer cancel()

	// Load the Shared AWS Configuration (~/.aws/config)
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	return &S3Client{rootCtx, s3.NewFromConfig(cfg), timeout}, nil
}

func (s *S3Client) Buckets() ([]string, error) {
	ctx, cancel := context.WithTimeout(s.ctx, s.timeout)
	defer cancel()
	output, err := s.client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}

	buckets := make([]string, len(output.Buckets))
	for i, bucket := range output.Buckets {
		buckets[i] = *bucket.Name
	}
	return buckets, nil
}

func (s *S3Client) List(bucket string, key string) ([]string, error) {
	ctx, cancel := context.WithTimeout(s.ctx, s.timeout)
	defer cancel()
	cfg := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Delimiter: aws.String("/"),
		Prefix:    aws.String(key),
	}
	output, err := s.client.ListObjectsV2(ctx, cfg)
	if err != nil {
		return nil, err
	}

	folders := make([]string, len(output.CommonPrefixes))
	for i, prefix := range output.CommonPrefixes {
		folders[i] = strings.TrimPrefix(*prefix.Prefix, key)
	}

	objects := make([]string, len(output.Contents))
	for i, object := range output.Contents {
		objects[i] = *object.Key
	}
	return append(folders, objects...), nil
}

func (s *S3Client) Upload(bucket string, key string, fileName string) error {
	ctx, cancel := context.WithTimeout(s.ctx, s.timeout)
	defer cancel()

	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	uploader := manager.NewUploader(s.client)
	_, err = uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "EntityTooLarge" {
			return fmt.Errorf("The object is too large. %v", err)
		} else {
			return err
		}
	}

	return nil
}
