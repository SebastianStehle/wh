package api

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"wh/domain/publish"

	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type ApiHandler interface {
	Index(c echo.Context) error
}

type apiHandler struct {
	publisher      publish.Publisher
	maxRequestTime time.Duration
	logger         *zap.Logger
}

func NewApiHandler(config *viper.Viper, publisher publish.Publisher, logger *zap.Logger) ApiHandler {
	return &apiHandler{
		publisher:      publisher,
		maxRequestSize: config.GetInt("request.maxSize"),
		maxRequestTime: config.GetDuration("request.timeout"),
		logger:         logger,
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

	chDone := make(chan bool)
	chError := make(chan error)
	tunneled.OnResponseStart(func(message publish.HttpResponseStart) {
		for k, v := range message.Headers {
			for _, h := range v {
				response.Header().Add(k, h)
			}
		}

		response.WriteHeader(int(message.Status))
	})

	tunneled.OnResponseChunk(func(message publish.HttpResponseChunk) {
		if len(message.Chunk) > 0 {
			_, err := response.Write(message.Chunk)
			if err != nil {
				chError <- err
			}
		}

		if message.Completed {
			chDone <- true
		}
	})

	tunneled.OnClientError(func(error error) {
		chError <- error
	})

	timer := time.After(a.maxRequestTime)
	select {
	case <-chDone:
		return nil
	case err := <-chError:
		return err
	case <-timer:
		response.WriteHeader(int(message.Status))
	}
	return err
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
