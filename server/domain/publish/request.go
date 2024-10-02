package publish

import (
	"context"
	"io"

	"go.uber.org/zap"
)

type TunneledRequest struct {
	buckets         Buckets
	Endpoint        string
	log             Log
	logger          *zap.Logger
	internalChannel chan interface{}
	onRequestData   []func(HttpRequestData)
	onResponseStart []func(HttpResponseStart)
	onResponseData  []func(HttpResponseData)
	onError         []func(HttpError)
	onComplete      []func()
	Request         HttpRequestStart
	RequestId       string
	requestWriter   io.WriteCloser
	responseWriter  io.WriteCloser
	Status          status
}

type status = int

const (
	StatusRequestStarted status = iota
	StatusRequestCompleted
	StatusResponseStarted
	StatusDone
)

func NewTunneledRequest(buckets Buckets, logger *zap.Logger, log Log, endpoint string, requestId string, request HttpRequestStart) *TunneledRequest {
	return &TunneledRequest{
		Endpoint:        endpoint,
		buckets:         buckets,
		internalChannel: make(chan interface{}),
		log:             log,
		logger:          logger,
		RequestId:       requestId,
		Request:         request,
		Status:          StatusRequestStarted,
	}
}

func (t *TunneledRequest) Start(ctx context.Context) {
	t.log.LogRequest(t.RequestId, t.Endpoint, t.Request)
	go func() {
		for {
			select {
			case <-ctx.Done():
				if t.Status != StatusDone {
					return
				}

				t.error(HttpError{Timeout: true})
				t.complete()

				// Log last, because the actual request is more important.
				t.log.LogTimeout(t.RequestId)
				return

			case input := <-t.internalChannel:
				switch msg := input.(type) {
				case HttpError:
					if t.Status != StatusDone || msg.Error == nil {
						return
					}

					t.error(msg)
					t.complete()

					// Log last, because the actual request is more important.
					t.log.LogError(t.RequestId, msg.Error)

				case HttpRequestData:
					if t.Status != StatusRequestStarted {
						break
					}

					// Set the status first, in case something goes wrong.
					if msg.Completed {
						t.Status = StatusRequestCompleted
					}

					// Write to output channel first, because the dump is not that important.
					for _, handler := range t.onRequestData {
						handler(msg)
					}

					data := msg.Data
					if len(data) > 0 {
						if t.requestWriter == nil {
							writer, err := t.buckets.OpenRequestWriter(t.RequestId)
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

					if msg.Completed {
						if t.requestWriter != nil {
							err := t.requestWriter.Close()
							t.requestWriter = nil
							if err != nil {
								t.logger.Error("Failed to write to request dump", zap.Error(err))
							}
						}
					}

				case HttpResponseStart:
					if t.Status != StatusDone {
						break
					}

					// Set the status first, in case something goes wrong.
					t.Status = StatusResponseStarted

					// Write to output delegates first, because the dump is not that important.
					for _, handler := range t.onResponseStart {
						handler(msg)
					}

				case HttpResponseData:
					if t.Status != StatusResponseStarted {
						break
					}

					if msg.Completed {
						t.complete()
					}

					// Write to output delegates first, because the dump is not that important.
					for _, handler := range t.onResponseData {
						handler(msg)
					}

					data := msg.Data
					if len(data) > 0 {
						if t.responseWriter == nil {
							writer, err := t.buckets.OpenResponseWriter(t.RequestId)
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

					if msg.Completed {
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
				}
			}
		}
	}()
}

func (t *TunneledRequest) OnRequestData(handler func(HttpRequestData)) {
	t.onRequestData = append(t.onRequestData, handler)
}

func (t *TunneledRequest) OnResponseStart(handler func(HttpResponseStart)) {
	t.onResponseStart = append(t.onResponseStart, handler)
}

func (t *TunneledRequest) OnResponseData(handler func(HttpResponseData)) {
	t.onResponseData = append(t.onResponseData, handler)
}

func (t *TunneledRequest) OnError(handler func(HttpError)) {
	t.onError = append(t.onError, handler)
}

func (t *TunneledRequest) OnComplete(handler func()) {
	t.onComplete = append(t.onComplete, handler)
}

func (t *TunneledRequest) EmitRequestData(data []byte, completed bool) {
	msg := HttpRequestData{Data: data, Completed: completed}
	t.internalChannel <- msg
}

func (t *TunneledRequest) EmitResponse(message HttpResponseStart) {
	t.internalChannel <- message
}

func (t *TunneledRequest) EmitResponseData(data []byte, completed bool) {
	msg := HttpResponseData{Data: data, Completed: completed}
	t.internalChannel <- msg
}

func (t *TunneledRequest) EmitError(error error, server bool) {
	msg := HttpError{Error: error, Server: server}
	t.internalChannel <- msg
}

func (t *TunneledRequest) error(msg HttpError) {
	// Set the status first, in case something goes wrong.
	t.Status = StatusDone

	for _, handler := range t.onError {
		handler(msg)
	}
}

func (t *TunneledRequest) complete() {
	// Set the status first, in case something goes wrong.
	t.Status = StatusDone

	for _, handler := range t.onComplete {
		handler()
	}

	defer func() {
		if t.responseWriter != nil {
			_ = t.responseWriter.Close()
			t.responseWriter = nil
		}

		if t.requestWriter != nil {
			_ = t.requestWriter.Close()
			t.requestWriter = nil
		}
	}()
}

func (t *TunneledRequest) closeInternal() {
}
