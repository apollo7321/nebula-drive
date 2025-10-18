package main

type StorageClient interface {
	Buckets() ([]string, error)
	List(bucket string, key string) ([]string, error)
	Upload(bucket string, key string, fileName string) error
}
