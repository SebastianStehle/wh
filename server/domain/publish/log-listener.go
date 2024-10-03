package publish

import (
	"go.uber.org/zap"
	"io"
)

type logListener struct {
	buckets        Buckets
	log            Log
	logger         *zap.Logger
	request        *TunneledRequest
	requestSize    int
	requestWriter  io.WriteCloser
	responseSize   int
	responseWriter io.WriteCloser
	status         int
	response       *HttpResponseStart
	error          error
}

func NewLogListener(request *TunneledRequest, buckets Buckets, log Log, logger *zap.Logger) RequestListener {
	log.LogRequest(request.RequestId, request.Endpoint, request.Request)

	return &logListener{
		buckets: buckets,
		log:     log,
		logger:  logger,
		request: request,
	}
}

func (l logListener) OnRequestData(msg HttpData) {
	data := msg.Data
	if len(data) <= 0 {
		return
	}

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
	if msg.Completed {
		l.closeRequestWriter()
	}
}

func (l logListener) OnResponseStart(HttpResponseStart) {
}

func (l logListener) OnResponseData(msg HttpData) {
	data := msg.Data
	if len(data) <= 0 {
		return
	}

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
	if msg.Completed {
		l.closeResponseWriter()
	}
}

func (l logListener) OnError(msg HttpError) {
	l.error = msg.Error
}

func (l logListener) OnComplete() {
	l.closeRequestWriter()
	l.closeResponseWriter()

	requestId := l.request.RequestId
	if l.error != nil && l.error.Error != nil {
		l.log.LogError(requestId, l.error)
	} else if l.response != nil {
		l.log.LogResponse(requestId, *l.response, l.requestSize, l.responseSize)
	} else {
		l.log.LogTimeout(requestId)
	}
}

func (l logListener) closeRequestWriter() {
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

func (l logListener) closeResponseWriter() {
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
