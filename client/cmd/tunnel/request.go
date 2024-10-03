package tunnel

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
	"wh/cli/api/tunnel"
)

type responseMessage struct {
	completed bool
	request   *tunneledRequest
	response  *tunnel.ClientMessage
	status    string
}

type tunneledRequest struct {
	cancel      context.CancelFunc
	method      string
	path        string
	requestBody *requestReader
	request     *http.Request
	requestId   string
}

func newTunneledRequest(ctx context.Context, localBase string, start *tunnel.RequestStart, sender chan interface{}) (*tunneledRequest, error) {
	requestReader := newRequestReader()

	requestCtx, cancel := context.WithTimeout(ctx, 4*time.Hour)

	localUrl := combineUrl(localBase, start.GetPath())

	httpReq, err := http.NewRequestWithContext(requestCtx, start.GetMethod(), localUrl, requestReader)
	if err != nil {
		return nil, err
	}

	for header, v := range start.GetHeaders() {
		for _, value := range v.GetValues() {
			httpReq.Header.Add(header, value)
		}
	}

	request := &tunneledRequest{
		method:      start.GetMethod(),
		path:        start.GetPath(),
		requestBody: requestReader,
		requestId:   start.GetRequestId(),
		request:     httpReq,
		cancel:      cancel,
	}

	go func() {
		defer cancel()

		request.run(ctx, sender)
	}()

	return request, nil
}

func (r *tunneledRequest) appendRequestData(data []byte, completed bool) {
	r.requestBody.AppendData(data, completed)
}

func (r *tunneledRequest) run(ctx context.Context, sender chan interface{}) {
	response, err := http.DefaultClient.Do(r.request)
	if err != nil {
		// Just send errors back to the sender go-routine, because we can't handle them here.
		sender <- r.buildErrorResponse(err, fmt.Sprintf("Failed to send tunneledRequest %s", err))
		return
	}

	status := int32(response.StatusCode)

	h := headersToLocal(response.Header)
	sender <- responseMessage{
		request: r,
		response: &tunnel.ClientMessage{
			TestMessageType: &tunnel.ClientMessage_ResponseStart{
				ResponseStart: &tunnel.ResponseStart{
					RequestId: &r.requestId,
					Headers:   h,
					Status:    &status,
				},
			},
		},
	}

	body := response.Body
	for {
		select {
		case <-ctx.Done():
			// Stop reading when the context has been cancelled.
			return

		default:
			buffer := make([]byte, 4*1024)
			n, err := body.Read(buffer)
			if err != nil && err != io.EOF {
				// Just send errors back to the sender go-routine, because we can't handle them here.
				sender <- r.buildErrorResponse(err, "Failed to read from tunneledRequest")
				return
			}

			complete := err == io.EOF

			statusText := ""
			if complete {
				statusText = fmt.Sprintf("%d %s", status, http.StatusText(int(status)))
			}

			sender <- responseMessage{
				completed: complete,
				request:   r,
				response: &tunnel.ClientMessage{
					TestMessageType: &tunnel.ClientMessage_ResponseData{
						ResponseData: &tunnel.ResponseData{
							RequestId: &r.requestId,
							Data:      buffer[0:n],
							Completed: &complete,
						},
					},
				},
				status: statusText,
			}

			if complete {
				return
			}
		}
	}
}

func (r *tunneledRequest) buildErrorResponse(err error, statusText string) responseMessage {
	errMsg := err.Error()

	fmt.Println()
	fmt.Print(statusText)
	fmt.Println()

	return responseMessage{
		request: r,
		response: &tunnel.ClientMessage{
			TestMessageType: &tunnel.ClientMessage_Error{
				Error: &tunnel.TransportError{
					RequestId: &r.requestId,
					Error:     &errMsg,
				},
			},
		},
		completed: true,
		status:    statusText,
	}
}

func headersToLocal(headers http.Header) map[string]*tunnel.HttpHeaderValues {
	result := make(map[string]*tunnel.HttpHeaderValues, len(headers))

	for header, v := range headers {
		result[header] = &tunnel.HttpHeaderValues{Values: v}
	}

	return result
}
