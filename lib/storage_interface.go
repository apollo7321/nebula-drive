package lib

type StorageClient interface {
	Buckets() ([]string, error)
	List(bucket string, key string) ([]string, error)
	Upload(bucket string, key string, fileName string) error
	Download(bucket string, key string, fileName string) error
	//Delete(bucket string, key string) error
}
