package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

const (
	testTimeout   = 10 * time.Second
	clientTimeout = 5 * time.Second
	bucketName    = "mybucket"
	kb            = 1024
)

func TestBuckets(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	client, err := NewS3Client(ctx, clientTimeout)
	if err != nil {
		t.Errorf("Failed to get new S3 client: %v", err)
	}

	buckets, err := client.Buckets()
	if err != nil {
		t.Errorf("Failed to get buckets: %v", err)
	}

	if !slices.Contains(buckets, bucketName) {
		t.Errorf("Bucket '%v' is missing", bucketName)
	}
}

func TestList(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	client, err := NewS3Client(ctx, clientTimeout)
	if err != nil {
		t.Errorf("Failed to get new S3 client: %v", err)
	}

	tests := []struct {
		name string
		path string
	}{
		{"root", ""},
		{"folder", "foo/bar/bar/bar/baz/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.List(bucketName, tt.path)
			if err != nil {
				t.Errorf("Failed to get objects in bucket '%v': %v", bucketName, err)
			}
		})
	}
}

func TestUpload(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()
	client, err := NewS3Client(ctx, clientTimeout)
	if err != nil {
		t.Errorf("Failed to get new S3 client: %v", err)
	}

	fileName := createTempFile(t, 10*kb)
	key := filepath.Base(fileName)
	if err := client.Upload(bucketName, key, fileName); err != nil {
		t.Errorf("Failed to upload file: %v", err)
	}
}

func createTempFile(t *testing.T, size int64) string {
	t.Helper()
	fileName := fmt.Sprintf("%d_byte.txt", size)
	filePath := filepath.Join(t.TempDir(), fileName)
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("Failed to create temp file for test. Reason: %v", err)
	}
	defer file.Close()

	buffer := make([]byte, size)
	for i := range buffer {
		buffer[i] = '_'
	}

	_, err = file.Write(buffer)
	if err != nil {
		t.Fatalf("Failed to write to temp file '%v'. Reason: %v", filePath, err)
	}

	return filePath
}
