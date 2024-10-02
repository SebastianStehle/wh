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

	ctx, cancel := context.WithTimeout(c.Request().Context(), 4*time.Hour)
	defer cancel()

	tunneled, err := a.publisher.ForwardRequest(endpoint, forwardedRequest, ctx)
	if errors.Is(err, publish.ErrNotRegistered) {
		response.WriteHeader(http.StatusServiceUnavailable)
		return nil
	} else if err != nil {
		return err
	}

	done := make(chan error)
	tunneled.OnComplete(func() {
		done <- nil
	})

	tunneled.OnError(func(msg publish.HttpError) {
		if msg.Timeout {
			response.WriteHeader(http.StatusGatewayTimeout)
		} else if msg.Error != nil {
			done <- msg.Error
		}
	})

	tunneled.OnResponseStart(func(msg publish.HttpResponseStart) {
		for k, v := range msg.Headers {
			for _, h := range v {
				response.Header().Add(k, h)
			}
		}

		response.WriteHeader(int(msg.Status))
	})

	tunneled.OnResponseData(func(msg publish.HttpResponseData) {
		if len(msg.Data) > 0 {
			_, err := response.Write(msg.Data)
			if err != nil {
				done <- err
			}
		}
	})

	body := request.Body
	for {
		buffer := make([]byte, 4096)
		n, err := body.Read(buffer)
		if err != nil && err != io.EOF {
			tunneled.EmitError(err, true)
			return err
		}

		completed := err == io.EOF

		tunneled.EmitRequestData(buffer[:n], completed)
		if completed {
			break
		}
	}

	select {
	case err := <-done:
		return err
	}
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
