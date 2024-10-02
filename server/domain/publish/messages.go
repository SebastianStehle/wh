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

type HttpRequestData struct {
	// The chunk
	Data []byte

	// Indicated if the request is complete
	Completed bool
}

type HttpResponseStart struct {
	// The response headers.
	Headers http.Header

	// The status code.
	Status int32
}

type HttpResponseData struct {
	// The chunk
	Data []byte

	// Indicated if the response is complete
	Completed bool
}

type ClientError struct {
	// The client error
	Error error
}

type Timeout struct {
}
