package publish

import (
	"fmt"
	"io"
	"time"

	"go.uber.org/zap"
)

type tunneledRequest struct {
	buckets         Buckets
	endpoint        string
	log             Log
	logger          *zap.Logger
	internalChannel chan interface{}
	request         HttpRequestStart
	requestId       string
	requestWriter   io.WriteCloser
	responseWriter  io.WriteCloser
	state           state
	timeout         time.Duration
	publicChannel   chan interface{}
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

type TunneledRequest interface {
	Request() HttpRequestStart

	RequestId() string

	Events() chan interface{}

	EmitRequestData(data []byte, isComplete bool)

	EmitResponse(message HttpResponseStart)

	EmitResponseData(message HttpResponseData)

	EmitClientError(error error)

	Start()

	Close()
}

func NewTunneledRequest(buckets Buckets, logger *zap.Logger, log Log, endpoint string, timeout time.Duration, requestId string, request HttpRequestStart) TunneledRequest {
	return &tunneledRequest{
		endpoint:        endpoint,
		buckets:         buckets,
		internalChannel: make(chan interface{}),
		log:             log,
		logger:          logger,
		publicChannel:   make(chan interface{}),
		requestId:       requestId,
		request:         request,
		state:           StateRequestStarted,
		timeout:         timeout,
	}
}

func (t tunneledRequest) Request() HttpRequestStart {
	return t.request
}

func (t tunneledRequest) RequestId() string {
	return t.requestId
}

func (t tunneledRequest) Events() chan interface{} {
	return t.publicChannel
}

func (t tunneledRequest) Start() {
	t.log.LogRequest(t.requestId, t.endpoint, t.request)

	timer := time.NewTimer(t.timeout)

	fmt.Printf("FOOBAR")
	go func() {
		for {
			select {
			case <-timer.C:
				if t.state == StateFailed || t.state == StateClosed {
					return
				}
				t.log.LogTimeout(t.requestId)
				t.state = StateFailed

				t.publicChannel <- &Timeout{}
				t.Close()
				return

			case m := <-t.internalChannel:
				switch r := m.(type) {
				case HttpRequestData:
					if t.state != StateRequestStarted {
						break
					}

					// Write to output channel first, because the dump is not that important.
					t.publicChannel <- r

					data := r.Data
					if len(data) > 0 {
						if t.requestWriter == nil {
							writer, err := t.buckets.OpenRequestWriter(t.requestId)
							if err != nil {
								t.logger.Error("Failed to write to request dump", zap.Error(err))
								break
							}

							t.requestWriter = writer
						}

						_, err := t.requestWriter.Write(data)
						if err != nil {
							t.logger.Error("Failed to write to request dump", zap.Error(err))
							break
						}
					}

					if r.Completed {
						t.state = StateRequestCompleted

						if t.requestWriter != nil {
							err := t.requestWriter.Close()
							t.requestWriter = nil
							if err != nil {
								t.logger.Error("Failed to write to request dump", zap.Error(err))
							}
						}
					}

				case HttpResponseStart:
					if t.state != StateRequestCompleted {
						break
					}

					t.state = StateResponseStarted
					t.publicChannel <- r

				case HttpResponseData:
					if t.state != StateResponseStarted {
						break
					}

					// Write to output channel first, because the dump is not that important.
					t.publicChannel <- r

					data := r.Data

					if len(data) > 0 {
						if t.responseWriter == nil {
							writer, err := t.buckets.OpenResponseWriter(t.requestId)
							if err != nil {
								t.logger.Error("Failed to write to response dump", zap.Error(err))
								break
							}

							t.responseWriter = writer
						}

						_, err := t.responseWriter.Write(data)
						if err != nil {
							t.logger.Error("Failed to write to response dump", zap.Error(err))
							break
						}
					}

					if r.Completed {
						t.state = StateResponseCompleted

						if t.responseWriter != nil {
							err := t.responseWriter.Close()
							t.responseWriter = nil
							if err != nil {
								t.logger.Error("Failed to close to response dump", zap.Error(err))
								break
							}

						}
						break
					}
				default:
					fmt.Printf("INVALID %T\n", m)
				}
			}
		}
	}()
}

func (t tunneledRequest) EmitRequestData(data []byte, completed bool) {
	msg := HttpRequestData{Data: data, Completed: completed}
	t.internalChannel <- msg
}

func (t tunneledRequest) EmitResponse(message HttpResponseStart) {
	t.internalChannel <- message
}

func (t tunneledRequest) EmitResponseData(message HttpResponseData) {
	t.internalChannel <- message
}

func (t tunneledRequest) EmitClientError(error error) {
	t.internalChannel <- ClientError{Error: error}
}

func (t tunneledRequest) Close() {
	close(t.publicChannel)
	close(t.internalChannel)

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
