package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigint
		cancel()
	}()

	if err := manualTest(rootCtx); err != nil {
		log.Fatalf("failed: %v", err)
	}
}

func manualTest(rootCtx context.Context) error {
	client, err := NewS3Client(rootCtx, 10*time.Second)

	if err != nil {
		return fmt.Errorf("Failed to get client: %v", err)
	}

	var c StorageClient = client
	buckets, err := c.Buckets()
	if err != nil {
		return fmt.Errorf("Failed to get buckets: %v", err)
	}

	log.Println("Buckets:")
	for _, bucket := range buckets {
		log.Println(" *", bucket)
	}

	bucket := buckets[2]
	log.Println("Objects in", bucket)
	objects, err := c.List(bucket, "")
	if err != nil {
		return fmt.Errorf("Failed to get objects: %v", err)
	}
	for _, o := range objects {
		log.Println(" *", o)
	}

	path := "foo/bar/bar/bar/baz/"
	log.Println("Objects in " + bucket + "/" + path)
	objects, err = c.List(bucket, path)
	if err != nil {
		return fmt.Errorf("Failed to get objects: %v", err)
	}
	for _, o := range objects {
		f := strings.TrimPrefix(o, path)
		log.Println(" *", f)
	}

	return nil
}
