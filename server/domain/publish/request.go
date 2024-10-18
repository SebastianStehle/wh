package publish

import (
	"net/http"
	"sync"

	"go.uber.org/zap"
)

type registration[T any] struct {
	action T
	// Use the origin to only send events to other listeners.
	origin int
}

type TunneledRequest struct {
	lock            sync.RWMutex
	logger          *zap.Logger
	onError         []registration[func(HttpError)]
	onRequestData   []registration[func(HttpRequestData)]
	onResponseData  []registration[func(HttpResponseData)]
	onResponseStart []registration[func(HttpResponseStart)]
	Endpoint        string
	Request         HttpRequestStart
	RequestId       string
	Status          Status
}

func NewTunneledRequest(endpoint string, requestId string, request HttpRequestStart, logger *zap.Logger) *TunneledRequest {
	return &TunneledRequest{
		Endpoint:        endpoint,
		onError:         make([]registration[func(HttpError)], 0, 2),
		onRequestData:   make([]registration[func(HttpRequestData)], 0, 2),
		onResponseData:  make([]registration[func(HttpResponseData)], 0, 2),
		onResponseStart: make([]registration[func(HttpResponseStart)], 0, 2),
		logger:          logger,
		Request:         request,
		RequestId:       requestId,
		Status:          StatusRequestStarted,
	}
}

func (t *TunneledRequest) OnError(origin int, action func(HttpError)) {
	r := registration[func(HttpError)]{origin: origin, action: action}
	t.onError = append(t.onError, r)
}

func (t *TunneledRequest) OnRequestData(origin int, action func(HttpRequestData)) {
	r := registration[func(HttpRequestData)]{origin: origin, action: action}
	t.onRequestData = append(t.onRequestData, r)
}

func (t *TunneledRequest) OnResponseData(origin int, action func(HttpResponseData)) {
	r := registration[func(HttpResponseData)]{origin: origin, action: action}
	t.onResponseData = append(t.onResponseData, r)
}

func (t *TunneledRequest) OnResponseStart(origin int, action func(HttpResponseStart)) {
	r := registration[func(HttpResponseStart)]{origin: origin, action: action}
	t.onResponseStart = append(t.onResponseStart, r)
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

	if len(t.onRequestData) > 0 {
		msg := HttpRequestData{Request: t, Data: data, Completed: completed}
		for _, r := range t.onRequestData {
			if r.origin != origin {
				r.action(msg)
			}
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

	if len(t.onResponseStart) > 0 {
		msg := HttpResponseStart{Headers: headers, Status: status}
		for _, r := range t.onResponseStart {
			if r.origin != origin {
				r.action(msg)
			}
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

	if len(t.onRequestData) > 0 {
		msg := HttpResponseData{Request: t, Data: data, Completed: completed}
		for _, r := range t.onResponseData {
			if r.origin != origin {
				r.action(msg)
			}
		}
	}
}

func (t *TunneledRequest) EmitError(origin int, err error, timeout bool) {
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

	if len(t.onError) > 0 {
		msg := HttpError{Request: t, Error: err, Timeout: timeout}
		for _, r := range t.onError {
			if r.origin != origin {
				r.action(msg)
			}
		}
	}
}

func (t *TunneledRequest) Cancel(origin int) {
	t.EmitError(origin, nil, true)
}
