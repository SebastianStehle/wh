package publish

import "net/http"

type HttpRequestStart struct {
	RequestId string

	// The request URL.
	Path string

	// The request method.
	Method string

	// The request headers.
	Headers http.Header
}

type HttpResponseStart struct {
	// The response headers.
	Headers http.Header

	// The status code.
	Status int32
}

type HttpData struct {
	// The chunk
	Data []byte

	// Indicated if the request is complete
	Completed bool
}

type HttpError struct {
	// The client error
	Error error

	// Indicate if the error is a timeout
	Timeout bool
}

type Complete struct{}
