package publish

import (
	"go.uber.org/zap"
	"io"
)

type bucketListener struct {
	buckets        Buckets
	log            Log
	logger         *zap.Logger
	request        *TunneledRequest
	requestSize    int
	requestWriter  io.WriteCloser
	responseSize   int
	responseWriter io.WriteCloser
}

func NewBucketListener(request *TunneledRequest, buckets Buckets, log Log, logger *zap.Logger) RequestListener {
	return &bucketListener{
		buckets: buckets,
		log:     log,
		logger:  logger,
		request: request,
	}
}

func (l bucketListener) OnRequestData(msg HttpData) {
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
		l.logger.Error("Failed to write to request writer",
			zap.Error(err),
		)
	}

	l.requestSize += n

	if msg.Completed {
		l.closeRequestWriter()
	}
}

func (l bucketListener) OnResponseStart(msg HttpResponseStart) {
}

func (l bucketListener) OnResponseData(msg HttpData) {
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
		l.logger.Error("Failed to write to response writer",
			zap.Error(err),
		)
	}

	l.responseSize += n

	if msg.Completed {
		l.closeResponseWriter()
	}
}

func (l bucketListener) OnError(msg HttpError) {
}

func (l bucketListener) OnComplete() {
	l.closeRequestWriter()
	l.closeResponseWriter()
}

func (l bucketListener) closeRequestWriter() {
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

func (l bucketListener) closeResponseWriter() {
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
