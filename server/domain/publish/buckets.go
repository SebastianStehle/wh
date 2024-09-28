package publish

import "io"

type Buckets interface {
	OpenRequestWriter(requestId string) (io.WriteCloser, error)

	OpenResponseWriter(requestId string) (io.WriteCloser, error)
}
