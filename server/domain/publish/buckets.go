package publish

import (
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Buckets interface {
	OpenRequestWriter(requestId string) (io.WriteCloser, error)

	OpenRequestReader(requestId string) (io.ReadCloser, error)

	OpenResponseWriter(requestId string) (io.WriteCloser, error)

	OpenResponseReader(requestId string) (io.ReadCloser, error)

	Delete(requestId string) error
}

type fileBucket struct {
	basePath string
}

func NewFileBucket(config *viper.Viper) Buckets {
	basePath := config.GetString("dataFolder")

	return &fileBucket{basePath: basePath}
}

func (f fileBucket) OpenRequestWriter(requestId string) (io.WriteCloser, error) {
	fullPath, err := f.getFilePath(requestId, "request.blob")
	if err != nil {
		return nil, err
	}

	return os.Create(fullPath)
}

func (f fileBucket) OpenRequestReader(requestId string) (io.ReadCloser, error) {
	fullPath, err := f.getFilePath(requestId, "request.blob")
	if err != nil {
		return nil, err
	}

	return os.Open(fullPath)
}

func (f fileBucket) OpenResponseWriter(requestId string) (io.WriteCloser, error) {
	fullPath, err := f.getFilePath(requestId, "response.blob")
	if err != nil {
		return nil, err
	}

	return os.Create(fullPath)
}

func (f fileBucket) OpenResponseReader(requestId string) (io.ReadCloser, error) {
	fullPath, err := f.getFilePath(requestId, "response.blob")
	if err != nil {
		return nil, err
	}

	return os.Open(fullPath)
}

func (f fileBucket) Delete(requestId string) error {
	folder := filepath.Join(f.basePath, "dumps", requestId)

	return os.RemoveAll(folder)
}

func (f fileBucket) getFilePath(requestId string, file string) (string, error) {
	folder := filepath.Join(f.basePath, "dumps", requestId)

	err := os.MkdirAll(folder, 0755)
	if err != nil {
		return "", err
	}

	fullPath := filepath.Join(folder, file)
	return fullPath, nil
}
