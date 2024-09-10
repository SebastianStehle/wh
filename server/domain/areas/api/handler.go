package api

import (
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

type ApiHandler interface {
	Index(c echo.Context) error
}

type apiHandler struct {
	publisher      publish.Publisher
	maxRequestSize int
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

	if request.ContentLength > int64(a.maxRequestSize) {
		response.WriteHeader(http.StatusRequestEntityTooLarge)
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

	body, err := readAll(request.Body, a.maxRequestSize)
	if err == ErrMaxReached {
		response.WriteHeader(http.StatusRequestEntityTooLarge)
		return nil
	} else if err != nil {
		return err
	}

	forwaredRequest := publish.HttpRequest{
		Path:    path,
		Method:  request.Method,
		Headers: request.Header,
		Body:    body,
	}

	forwaredResponse, err := a.publisher.ForwardRequest(endpoint, a.maxRequestTime, forwaredRequest)
	if err == publish.ErrTimeout {
		response.WriteHeader(http.StatusGatewayTimeout)
		return nil
	} else if err == publish.ErrNotRegistered {
		response.WriteHeader(http.StatusServiceUnavailable)
		return nil
	} else if err != nil {
		return err
	}

	for k, v := range forwaredResponse.Headers {
		for _, h := range v {
			response.Header().Add(k, h)
		}
	}

	response.WriteHeader(int(forwaredResponse.Status))

	_, err = response.Write(forwaredResponse.Body)
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

func readAll(r io.Reader, capacity int) ([]byte, error) {
	b := make([]byte, 0, 512)
	for {
		n, err := r.Read(b[len(b):cap(b)])
		b = b[:len(b)+n]
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return b, err
		}

		if len(b) > capacity {
			return nil, ErrMaxReached
		}

		if len(b) == cap(b) {
			// Add more capacity (let append pick how much).
			b = append(b, 0)[:len(b)]
		}
	}
}

var ErrMaxReached = errors.New("MaxReached")
