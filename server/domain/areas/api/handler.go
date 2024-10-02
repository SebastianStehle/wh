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

	tunneled, err := a.publisher.ForwardRequest(endpoint, a.timeout, forwardedRequest)
	if errors.Is(err, publish.ErrNotRegistered) {
		response.WriteHeader(http.StatusServiceUnavailable)
		return nil
	} else if err != nil {
		return err
	}

	defer func() {
		tunneled.Close()
	}()

	body := request.Body
	for {
		buffer := make([]byte, 4096)
		n, err := body.Read(buffer)
		if err != nil && err != io.EOF {
			tunneled.Close()
			return err
		}

		completed := err == io.EOF

		tunneled.EmitRequestData(buffer[:n], completed)
		if completed {
			break
		}
	}

	ch := tunneled.Events()
	for e := range ch {
		switch m := e.(type) {
		case publish.Timeout:
			response.WriteHeader(http.StatusGatewayTimeout)
			return nil

		case publish.ClientError:
			return m.Error

		case publish.HttpResponseStart:
			for k, v := range m.Headers {
				for _, h := range v {
					response.Header().Add(k, h)
				}
			}

			response.WriteHeader(int(m.Status))

		case publish.HttpResponseData:
			if len(m.Data) > 0 {
				_, err := response.Write(m.Data)
				if err != nil {
					return err
				}
			}

			if m.Completed {
				return nil
			}
		}
	}

	return nil
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
