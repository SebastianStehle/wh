package publish

import (
	"io"

	"go.uber.org/zap"
)

var (
	EventOrigin = 1001
)

type recorder struct {
	buckets        Buckets
	store          Store
	logger         *zap.Logger
	request        *TunneledRequest
	requestSize    int
	requestWriter  io.WriteCloser
	responseSize   int
	responseWriter io.WriteCloser
	response       *HttpResponseStart
	error          error
}

func NewRecorder(request *TunneledRequest, store Store, buckets Buckets, logger *zap.Logger) *recorder {
	if err := store.LogRequest(request.RequestId, request.Endpoint, request.Request); err != nil {
		logger.Error("Failed to record request",
			zap.Error(err),
		)
	}

	return &recorder{
		buckets: buckets,
		logger:  logger,
		request: request,
		store:   store,
	}
}

func (l *recorder) Listen(request *TunneledRequest) {
	request.OnRequestData(EventOrigin, l.OnRequestData)
	request.OnResponseStart(EventOrigin, l.OnResponseStart)
	request.OnResponseData(EventOrigin, l.OnResponseData)
	request.OnError(EventOrigin, l.OnError)
}

func (l *recorder) OnRequestData(msg HttpRequestData) {
	data := msg.Data
	if len(data) > 0 {
		if l.requestWriter == nil {
			writer, err := l.buckets.OpenRequestWriter(l.request.RequestId)
			if err != nil {
				l.logger.Error("Failed to open response writer",
					zap.Error(err),
				)
				return
			}
			l.requestWriter = writer
		}

		n, err := l.requestWriter.Write(data)
		if err != nil {
			l.requestSize = -1
			l.logger.Error("Failed to write to request writer",
				zap.Error(err),
			)
		}

		l.requestSize += n
	}

	if msg.Completed {
		l.closeRequestWriter()
	}
}

func (l *recorder) OnResponseStart(msg HttpResponseStart) {
	l.response = &msg
}

func (l *recorder) OnResponseData(msg HttpResponseData) {
	data := msg.Data
	if len(data) > 0 {
		if l.responseWriter == nil {
			writer, err := l.buckets.OpenResponseWriter(l.request.RequestId)
			if err != nil {
				l.logger.Error("Failed to open response writer",
					zap.Error(err),
				)
				return
			}
			l.responseWriter = writer
		}

		n, err := l.responseWriter.Write(data)
		if err != nil {
			l.responseSize = -1
			l.logger.Error("Failed to write to response writer",
				zap.Error(err),
			)
		}

		l.responseSize += n
	}

	if msg.Completed {
		l.complete(nil)
	}
}

func (l *recorder) OnError(msg HttpError) {
	l.complete(msg.Error)
}

func (l *recorder) complete(requestError error) {
	l.closeRequestWriter()
	l.closeResponseWriter()

	err := l.store.LogResponse(
		l.request.RequestId,
		l.requestSize,
		l.response,
		l.responseSize,
		requestError,
		l.request.Status)
	if err != nil {
		l.logger.Error("Failed to update request",
			zap.Error(err),
		)
	}
}

func (l *recorder) closeRequestWriter() {
	if l.requestWriter == nil {
		return
	}

	defer func() {
		l.requestWriter = nil
	}()

	if err := l.requestWriter.Close(); err != nil {
		l.logger.Error("Failed to close request writer",
			zap.Error(err),
		)
	}
}

func (l *recorder) closeResponseWriter() {
	if l.responseWriter == nil {
		return
	}

	defer func() {
		l.responseWriter = nil
	}()

	if err := l.responseWriter.Close(); err != nil {
		l.logger.Error("Failed to close response writer",
			zap.Error(err),
		)
	}
}
