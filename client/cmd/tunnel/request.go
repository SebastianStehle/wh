package tunnel

import (
	"context"
	"io"
	"net/http"
	"time"
)

type TunneledRequest struct {
	cancel          context.CancelFunc
	completed       bool
	Headers         http.Header
	Method          string
	onError         []func(msg HttpError)
	onResponseData  []func(msg HttpResponseData)
	onResponseStart []func(msg HttpResponseStart)
	Path            string
	requestBody     *requestReader
	RequestId       string
	Url             string
}

func NewTunneledRequest(localBase string, requestId string, method string, path string, headers http.Header) *TunneledRequest {
	request := &TunneledRequest{
		Headers:         headers,
		Method:          method,
		onError:         make([]func(msg HttpError), 1),
		onResponseData:  make([]func(msg HttpResponseData), 1),
		onResponseStart: make([]func(msg HttpResponseStart), 1),
		Path:            path,
		requestBody:     newRequestReader(),
		RequestId:       requestId,
		Url:             combineUrl(localBase, path),
	}

	return request
}

func (r *TunneledRequest) AppendRequestData(data []byte, completed bool) {
	r.requestBody.AppendData(data, completed)
}

func (r *TunneledRequest) Cancel() {
	if r.cancel == nil {
		return
	}

	r.cancel()
}

func (r *TunneledRequest) Run(ctx context.Context, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	r.cancel = cancel

	request, err := http.NewRequestWithContext(ctx, r.Method, r.Url, r.requestBody)
	if err != nil {
		r.emitError(err, false)
		return
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		r.emitError(err, false)
		return
	}

	if len(r.onResponseStart) > 0 {
		msg := HttpResponseStart{Request: r, Headers: response.Header, Status: int32(response.StatusCode)}
		for _, r := range r.onResponseStart {
			r(msg)
		}
	}

	body := response.Body
	for {
		select {
		case <-ctx.Done():
			r.emitError(nil, true)
			return

		default:
			buffer := make([]byte, 4*1024)
			n, err := body.Read(buffer)
			if err != nil && err != io.EOF {
				r.emitError(err, false)
				return
			}

			complete := err == io.EOF

			msg := HttpResponseData{Request: r, Data: buffer[0:n], Completed: complete}
			for _, r := range r.onResponseData {
				r(msg)
			}

			if complete {
				r.completed = true
				return
			}
		}
	}
}

func (r *TunneledRequest) emitError(err error, timeout bool) {
	if !r.completed {
		return
	}

	r.completed = true

	if len(r.onError) > 0 {
		msg := HttpError{Request: r, Error: err, Timeout: timeout}
		for _, r := range r.onError {
			r(msg)
		}
	}
}
