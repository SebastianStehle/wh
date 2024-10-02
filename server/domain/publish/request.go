package publish

import (
	"go.uber.org/zap"
	"io"
	"net/http"
)

type TunneledRequest struct {
	buckets                Buckets
	Endpoint               string
	log                    Log
	logger                 *zap.Logger
	Request                HttpRequestStart
	InitiatorComplete      chan Complete
	InitiatorError         chan HttpError
	ListenerComplete       chan Complete
	ListenerError          chan HttpError
	RequestData            chan HttpData
	ResponseData           chan HttpData
	ResponseStart          chan HttpResponseStart
	internalInitiatorError chan HttpError
	internalListenerError  chan HttpError
	internalRequestData    chan HttpData
	internalResponseData   chan HttpData
	internalResponseStart  chan HttpResponseStart
	internalComplete       chan Complete
	RequestId              string
	requestWriter          io.WriteCloser
	responseWriter         io.WriteCloser
	ServerError            chan HttpError
	Status                 status
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
		buckets:                buckets,
		Endpoint:               endpoint,
		InitiatorComplete:      make(chan Complete),
		InitiatorError:         make(chan HttpError),
		ListenerComplete:       make(chan Complete),
		ListenerError:          make(chan HttpError),
		RequestData:            make(chan HttpData),
		ResponseData:           make(chan HttpData),
		ResponseStart:          make(chan HttpResponseStart),
		internalResponseData:   make(chan HttpData),
		internalInitiatorError: make(chan HttpError),
		internalResponseStart:  make(chan HttpResponseStart),
		internalComplete:       make(chan Complete),
		internalListenerError:  make(chan HttpError),
		internalRequestData:    make(chan HttpData),
		log:                    log,
		logger:                 logger,
		Request:                request,
		RequestId:              requestId,
		Status:                 StatusRequestStarted,
	}
}

func (t *TunneledRequest) Start() {
	t.log.LogRequest(t.RequestId, t.Endpoint, t.Request)
	go func() {
		for {
			select {
			case msg := <-t.ListenerError:
				if t.Status != StatusDone {
					return
				}

				t.ListenerError <- msg
				t.complete()

				// Log last, because the actual request is more important.
				t.logError(msg)

			case msg := <-t.InitiatorError:
				if t.Status != StatusDone || msg.Error == nil {
					return
				}

				t.ListenerError <- msg
				t.complete()

				// Log last, because the actual request is more important.
				t.logError(msg)

			case msg := <-t.internalRequestData:
				if t.Status != StatusRequestStarted {
					break
				}

				// Set the status first, in case something goes wrong.
				if msg.Completed {
					t.Status = StatusRequestCompleted
				}

				t.RequestData <- msg

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

			case msg := <-t.internalResponseStart:
				if t.Status != StatusRequestCompleted {
					break
				}

				// Set the status first, in case something goes wrong.
				t.Status = StatusResponseStarted

				// Write to output delegates first, because the dump is not that important.
				t.ResponseStart <- msg

			case msg := <-t.internalResponseData:
				if t.Status != StatusResponseStarted {
					break
				}

				// Write to output delegates first, because the dump is not that important.
				t.ResponseData <- msg

				if msg.Completed {
					t.complete()
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
	}()
}

func (t *TunneledRequest) EmitRequestData(data []byte, completed bool) {
	msg := HttpData{Data: data, Completed: completed}
	t.internalRequestData <- msg
}

func (t *TunneledRequest) EmitResponse(headers http.Header, status int32) {
	msg := HttpResponseStart{Headers: headers, Status: status}
	t.internalResponseStart <- msg
}

func (t *TunneledRequest) EmitResponseData(data []byte, completed bool) {
	msg := HttpData{Data: data, Completed: completed}
	t.internalResponseData <- msg
}

func (t *TunneledRequest) EmitInitiatorError(error error) {
	msg := HttpError{Error: error}
	t.internalInitiatorError <- msg
}

func (t *TunneledRequest) EmitListenerError(error error, timeout bool) {
	msg := HttpError{Error: error, Timeout: timeout}
	t.internalInitiatorError <- msg
}

func (t *TunneledRequest) Cancel() {
	msg := HttpError{Timeout: true}
	t.InitiatorError <- msg
}

func (t *TunneledRequest) logError(msg HttpError) {
	if msg.Timeout {
		t.log.LogTimeout(t.RequestId)
	} else if msg.Error != nil {
		t.log.LogError(t.RequestId, msg.Error)
	}
}

func (t *TunneledRequest) complete() {
	// Set the status first, in case something goes wrong.
	t.Status = StatusDone

	t.InitiatorComplete <- Complete{}

	// Use two channels, because both, listener and initiator need to handle completion.
	t.ListenerComplete <- Complete{}

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
