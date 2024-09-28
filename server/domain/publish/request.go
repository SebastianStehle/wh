package publish

import (
	"fmt"
	"io"
	"time"
)

type tunneledRequest struct {
	buckets         Buckets
	endpoint        string
	log             Log
	onClientError   ClientErrorHandler
	onRequestChunk  RequestChunkHandler
	onResponseChunk ResponseChunkHandler
	onResponseStart ResponseStartHandler
	onTimeout       TimeoutHandler
	request         HttpRequestStart
	requestId       string
	requestWriter   io.WriteCloser
	responseWriter  io.WriteCloser
	state           state
	timeout         time.Duration
}

type state = int

const (
	StateRequestStarted state = iota
	StateRequestCompleted
	StateResponseStarted
	StateResponseCompleted
	StateFailed
	StateClosed
)

type RequestChunkHandler = func(chunk HttpRequestChunk)
type ResponseStartHandler = func(response HttpResponseStart)
type ResponseChunkHandler = func(chunk HttpResponseChunk)
type ClientErrorHandler = func(error error)
type TimeoutHandler = func()

type TunneledRequest interface {
	OnRequestChunk(handler RequestChunkHandler)

	OnResponseStart(handler ResponseStartHandler)

	OnResponseChunk(handler ResponseChunkHandler)

	OnClientError(handler ClientErrorHandler)

	OnTimeout(handler TimeoutHandler)

	Request() HttpRequestStart

	EmitRequestData(message HttpRequestChunk) error

	EmitResponse(message HttpResponseStart) error

	EmitResponseData(message HttpResponseChunk) error

	EmitClientError(error error) error

	Start()

	Close()
}

func NewTunneledRequest(buckets Buckets, log Log, endpoint string, timeout time.Duration, requestId string, request HttpRequestStart) TunneledRequest {
	return &tunneledRequest{
		endpoint:  endpoint,
		buckets:   buckets,
		log:       log,
		requestId: requestId,
		request:   request,
		timeout:   timeout,
	}
}

func (t tunneledRequest) Request() HttpRequestStart {
	return t.request
}

func (t tunneledRequest) OnRequestChunk(handler RequestChunkHandler) {
	t.onRequestChunk = handler
}

func (t tunneledRequest) OnResponseStart(handler ResponseStartHandler) {
	t.onResponseStart = handler
}

func (t tunneledRequest) OnResponseChunk(handler ResponseChunkHandler) {
	t.onResponseChunk = handler
}

func (t tunneledRequest) OnClientError(handler ClientErrorHandler) {
	t.onClientError = handler
}

func (t tunneledRequest) OnTimeout(handler TimeoutHandler) {
	t.onTimeout = handler
}

func (t tunneledRequest) Start() {
	t.log.LogRequest(t.requestId, t.endpoint, t.request)

	timer := time.NewTimer(t.timeout)

	go func() {
		<-timer.C

		if t.state == StateFailed || t.state == StateClosed {
			return
		}

		handler := t.onTimeout
		if handler != nil {
			handler()
		}

		t.log.LogTimeout(t.requestId)
		t.state = StateFailed
	}()
}

func (t tunneledRequest) EmitRequestData(message HttpRequestChunk) error {
	if t.state != StateRequestStarted {
		return fmt.Errorf("request not in RequestStarted state")
	}

	handler := t.onRequestChunk
	if handler == nil {
		return fmt.Errorf("no request chunk handler registered")
	}

	chunk := message.Chunk
	if len(chunk) > 0 {
		writer, err := t.buckets.OpenRequestWriter(t.requestId)
		if err != nil {
			return err
		}

		t.requestWriter = writer
		_, err = t.requestWriter.Write(chunk)
		if err != nil {
			return err
		}
	}

	if message.Completed {
		t.state = StateRequestCompleted

		if t.requestWriter != nil {
			err := t.requestWriter.Close()
			if err != nil {
				return err
			}

			t.requestWriter = nil
		}
	}

	handler(message)
	return nil
}

func (t tunneledRequest) EmitResponse(message HttpResponseStart) error {
	if t.state != StateRequestCompleted {
		return fmt.Errorf("request not in RequestCompleted state")
	}

	handler := t.onResponseStart
	if handler == nil {
		return fmt.Errorf("no response handler registered")
	}

	t.state = StateResponseStarted

	handler(message)
	return nil
}

func (t tunneledRequest) EmitResponseData(message HttpResponseChunk) error {
	if t.state != StateRequestStarted {
		return fmt.Errorf("request not in ResponseStarted state")
	}

	handler := t.onResponseChunk
	if handler == nil {
		return fmt.Errorf("no response chunk handler registered")
	}

	chunk := message.Chunk
	if len(chunk) > 0 {
		writer, err := t.buckets.OpenResponseWriter(t.requestId)
		if err != nil {
			return err
		}

		t.responseWriter = writer
		_, err = t.responseWriter.Write(chunk)
		if err != nil {
			return err
		}
	}

	if message.Completed {
		t.state = StateResponseCompleted

		if t.requestWriter != nil {
			err := t.requestWriter.Close()
			if err != nil {
				return err
			}

			t.requestWriter = nil
		}
	}

	handler(message)
	return nil
}

func (t tunneledRequest) EmitClientError(error error) error {
	handler := t.onClientError
	if handler == nil {
		return fmt.Errorf("no response handler registered")
	}

	t.state = StateFailed

	handler(error)
	return nil
}

func (t tunneledRequest) Close() {
	if t.responseWriter != nil {
		_ = t.responseWriter.Close()
		t.responseWriter = nil
	}

	if t.requestWriter != nil {
		_ = t.requestWriter.Close()
		t.requestWriter = nil
	}

	t.state = StateClosed
}
