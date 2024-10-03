package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
	"wh/domain/publish"

	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var (
	EventOrigin = 12
)

type apiHandler struct {
	publisher publish.Publisher
	timeout   time.Duration
	logger    *zap.Logger
}

type ApiHandler interface {
	Index(c echo.Context) error
}

func NewApiHandler(config *viper.Viper, publisher publish.Publisher, logger *zap.Logger) ApiHandler {
	return &apiHandler{
		publisher: publisher,
		timeout:   config.GetDuration("request.timeout"),
		logger:    logger,
	}
}

func (a apiHandler) Index(c echo.Context) error {
	request := c.Request()
	response := c.Response()

	var endpoint, path, ok = splitEndpointAndPath(request.URL.Path)
	if !ok {
		response.WriteHeader(http.StatusBadRequest)
		return nil
	}

	// Fragments are not sent to the server, therefore we just have to handle query strings.
	if request.URL.RawQuery != "" {
		path += "?"
		path += request.URL.RawQuery
	}

	a.logger.Info("Received webhook call",
		zap.String("input.endpoint", endpoint),
		zap.String("input.path", path),
	)

	forwardedRequest := publish.HttpRequestStart{
		Path:    path,
		Method:  request.Method,
		Headers: request.Header,
	}

	tunneled, err := a.publisher.ForwardRequest(endpoint, forwardedRequest)
	if errors.Is(err, publish.ErrNotRegistered) {
		response.WriteHeader(http.StatusServiceUnavailable)
		return nil
	} else if err != nil {
		return err
	}

	listener := listener{
		responseData:  make(chan publish.HttpData),
		responseStart: make(chan publish.HttpResponseStart),
		error:         make(chan publish.HttpError),
		done:          make(chan bool),
	}

	tunneled.Listen(EventOrigin, listener)
	defer tunneled.Cancel(EventOrigin)

	ctx, cancel := context.WithTimeout(c.Request().Context(), 4*time.Hour)
	defer cancel()

	body := request.Body
	for {
		buffer := make([]byte, 4096)
		n, err := body.Read(buffer)
		if err != nil && err != io.EOF {
			tunneled.EmitError(EventOrigin, err, false)
			return err
		}

		completed := err == io.EOF

		tunneled.EmitRequestData(EventOrigin, buffer[:n], completed)
		if completed {
			break
		}
	}

	for {
		select {
		case <-ctx.Done():
			response.WriteHeader(http.StatusGatewayTimeout)
			return nil
		case <-listener.done:
			return nil
		case msg := <-listener.error:
			if msg.Timeout {
				response.WriteHeader(http.StatusGatewayTimeout)
				return nil
			} else {
				return msg.Error
			}
		case msg := <-listener.responseStart:
			for k, v := range msg.Headers {
				for _, h := range v {
					response.Header().Add(k, h)
				}
			}

			response.WriteHeader(int(msg.Status))

		case msg := <-listener.responseData:
			if len(msg.Data) > 0 {
				_, err := response.Write(msg.Data)
				if err != nil {
					tunneled.EmitError(EventOrigin, err, false)
					return err
				}
			}
		}
	}
}

type listener struct {
	responseStart chan publish.HttpResponseStart
	responseData  chan publish.HttpData
	error         chan publish.HttpError
	done          chan bool
}

func (l listener) OnRequestData(publish.HttpData) {
}

func (l listener) OnComplete() {
	l.done <- true
}

func (l listener) OnResponseStart(msg publish.HttpResponseStart) {
	l.responseStart <- msg
}

func (l listener) OnResponseData(msg publish.HttpData) {
	l.responseData <- msg
}

func (l listener) OnError(msg publish.HttpError) {
	l.error <- msg
}

func splitEndpointAndPath(rawPath string) (string, string, bool) {
	parts := make([]string, 0)
	for _, v := range strings.Split(rawPath, "/") {
		if v != "" {
			parts = append(parts, v)
		}
	}

	if len(parts) < 2 {
		return "", "", false
	}

	var path = strings.Join(parts[2:], "/")

	if path != "" {
		path = "/" + path
	}

	if strings.HasSuffix(rawPath, "/") {
		path += "/"
	}

	return parts[1], path, true
}
