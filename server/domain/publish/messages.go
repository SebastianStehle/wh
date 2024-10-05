package publish

import "net/http"

type HttpRequestStart struct {
	// The actual request
	Request *TunneledRequest

	// The request URL.
	Path string

	// The request method.
	Method string

	// The request headers.
	Headers http.Header
}

type HttpRequestData struct {
	// The actual request
	Request *TunneledRequest

	// The chunk
	Data []byte

	// Indicated if the request is complete
	Completed bool
}

type HttpResponseStart struct {
	// The actual request
	Request *TunneledRequest

	// The response headers.
	Headers http.Header

	// The status code.
	Status int32
}

type HttpResponseData struct {
	// The actual request
	Request *TunneledRequest

	// The chunk
	Data []byte

	// Indicated if the request is complete
	Completed bool
}

type HttpError struct {
	// The actual request
	Request *TunneledRequest

	// The client error
	Error error

	// Indicate if the error is a timeout
	Timeout bool
}

type HttpComplete struct {
	// The actual request
	Request *TunneledRequest
}
