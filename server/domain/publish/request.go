package publish

import (
	"net/http"
	"sync"

	"go.uber.org/zap"
)

type RequestListener interface {
	OnComplete()

	OnError(msg HttpError)

	OnRequestData(msg HttpData)

	OnResponseStart(msg HttpResponseStart)

	OnResponseData(msg HttpData)
}

type registration struct {
	origin   int
	listener RequestListener
}

type TunneledRequest struct {
	listeners []registration
	lock      sync.RWMutex
	logger    *zap.Logger
	Endpoint  string
	Request   HttpRequestStart
	RequestId string
	Status    Status
}

func NewTunneledRequest(endpoint string, requestId string, request HttpRequestStart, logger *zap.Logger) *TunneledRequest {
	return &TunneledRequest{
		Endpoint:  endpoint,
		listeners: make([]registration, 0),
		logger:    logger,
		Request:   request,
		RequestId: requestId,
		Status:    StatusRequestStarted,
	}
}

func (t *TunneledRequest) Listen(origin int, listener RequestListener) {
	registration := registration{origin: origin, listener: listener}

	t.listeners = append(t.listeners, registration)
}

func (t *TunneledRequest) EmitRequestData(origin int, data []byte, completed bool) {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.Status != StatusRequestStarted {
		return
	}

	if completed {
		t.Status = StatusRequestCompleted
	}

	msg := HttpData{Data: data, Completed: completed}
	for _, r := range t.listeners {
		if r.origin != origin {
			r.listener.OnRequestData(msg)
		}
	}
}

func (t *TunneledRequest) EmitResponse(origin int, headers http.Header, status int32) {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.Status != StatusRequestCompleted {
		return
	}

	t.Status = StatusResponseStarted

	msg := HttpResponseStart{Headers: headers, Status: status}
	for _, r := range t.listeners {
		if r.origin != origin {
			r.listener.OnResponseStart(msg)
		}
	}
}

func (t *TunneledRequest) EmitResponseData(origin int, data []byte, completed bool) {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.Status != StatusResponseStarted {
		return
	}

	if completed {
		t.Status = StatusCompleted
	}

	msg := HttpData{Data: data, Completed: completed}
	for _, r := range t.listeners {
		if r.origin != origin {
			r.listener.OnResponseData(msg)

			if completed {
				r.listener.OnComplete()
			}
		}
	}
}

func (t *TunneledRequest) EmitError(origin int, error error, timeout bool) {
	t.lock.Lock()
	defer t.lock.Unlock()

	if IsTerminated(t.Status) {
		return
	}

	if timeout {
		t.Status = StatusTimeout
	} else {
		t.Status = StatusFailed
	}

	msg := HttpError{Error: error, Timeout: timeout}
	for _, r := range t.listeners {
		if r.origin != origin {
			r.listener.OnError(msg)
			r.listener.OnComplete()
		}
	}
}

func (t *TunneledRequest) Cancel(origin int) {
	t.lock.Lock()
	defer t.lock.Unlock()

	if IsTerminated(t.Status) {
		return
	}

	t.Status = StatusTimeout

	msg := HttpError{Timeout: true}
	for _, r := range t.listeners {
		if r.origin != origin {
			r.listener.OnError(msg)
			r.listener.OnComplete()
		}
	}
}
