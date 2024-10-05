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
	logger    *zap.Logger
	publisher publish.Publisher
	timeout   time.Duration
}

type ApiHandler interface {
	Index(c echo.Context) error
}

func NewApiHandler(publisher publish.Publisher, config *viper.Viper, logger *zap.Logger) ApiHandler {
	timeout := config.GetDuration("request.timeout")

	return &apiHandler{
		logger:    logger,
		publisher: publisher,
		timeout:   timeout,
	}
}

// ANY /endpoints/*
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

	responseData := make(chan publish.HttpResponseData)
	responseStart := make(chan publish.HttpResponseStart)
	tunnelDone := make(chan publish.HttpComplete)
	tunnelError := make(chan publish.HttpError)

	tunneled.OnResponseStart(EventOrigin, func(msg publish.HttpResponseStart) {
		responseStart <- msg
	})

	tunneled.OnResponseData(EventOrigin, func(msg publish.HttpResponseData) {
		responseData <- msg
	})

	tunneled.OnError(EventOrigin, func(msg publish.HttpError) {
		tunnelError <- msg
	})

	tunneled.OnComplete(EventOrigin, func(msg publish.HttpComplete) {
		tunnelDone <- msg
	})

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
		case <-tunnelDone:
			return nil
		case msg := <-tunnelError:
			if msg.Timeout {
				response.WriteHeader(http.StatusGatewayTimeout)
				return nil
			} else {
				return msg.Error
			}
		case msg := <-responseStart:
			for k, v := range msg.Headers {
				for _, h := range v {
					response.Header().Add(k, h)
				}
			}

			response.WriteHeader(int(msg.Status))

		case msg := <-responseData:
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
