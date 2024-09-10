package publish

import "net/http"

type HttpRequest struct {
	// The request URL.
	Path string

	// The request method.
	Method string

	// The request headers.
	Headers http.Header

	// The request body.
	Body []byte
}

type HttpResponse struct {
	// The response headers.
	Headers http.Header

	// The status code.
	Status int32

	// The response body.
	Body []byte
}
