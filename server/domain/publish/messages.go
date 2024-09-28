package publish

import "net/http"

type HttpRequestStart struct {
	// The request URL.
	Path string

	// The request method.
	Method string

	// The request headers.
	Headers http.Header
}

type HttpRequestChunk struct {
	// The chunk
	Chunk []byte

	// Indicated if the request is complete
	Completed bool
}

type HttpResponseStart struct {
	// The response headers.
	Headers http.Header

	// The status code.
	Status int32
}

type HttpResponseChunk struct {
	// The chunk
	Chunk []byte

	// Indicated if the response is complete
	Completed bool
}
