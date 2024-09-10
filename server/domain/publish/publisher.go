package publish

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/viper"
)

type pathHandlers struct {
	requests map[string]*responeHandler
	response func(string, HttpRequest)
}

type responeHandler struct {
	onResponse chan HttpResponse
	onError    chan error
}

type publisher struct {
	endpoints map[string]*pathHandlers
	log       Log
}

// There was no response within the defined duration.
var ErrTimeout = errors.New("Timeout")

// There is already a request handler.
var ErrAlreadyRegistered = errors.New("AlreadyRegistered")

// There is no listener.
var ErrNotRegistered = errors.New("NotRegistered")

type Publisher interface {
	Subscribe(endpoint string, handler func(string, HttpRequest)) error

	Unsubscribe(endpoint string)

	OnResponse(endpoint string, requestId string, response HttpResponse) error

	OnError(endpoint string, requestId string, err error) error

	ForwardRequest(endpoint string, timeout time.Duration, request HttpRequest) (*HttpResponse, error)

	GetEntries(etag int64) ([]LogEntry, int64)
}

func NewPublisher(config *viper.Viper) Publisher {
	maxSize := config.GetInt("log.maxSize")
	maxEntries := config.GetInt("log.maxEntries")

	log := NewLog(maxSize, maxEntries)

	return &publisher{
		endpoints: make(map[string]*pathHandlers),
		log:       log,
	}
}

func (p publisher) GetEntries(etag int64) ([]LogEntry, int64) {
	return p.log.GetEntries(etag)
}

func (p publisher) Unsubscribe(endpoint string) {
	delete(p.endpoints, endpoint)
}

func (p publisher) Subscribe(endpoint string, handler func(string, HttpRequest)) error {
	registration := p.endpoints[endpoint]
	if registration != nil {
		return ErrAlreadyRegistered
	}

	registration = &pathHandlers{
		requests: map[string]*responeHandler{},
		response: handler,
	}

	p.endpoints[endpoint] = registration
	return nil
}

func (p publisher) OnResponse(endpoint string, requestId string, response HttpResponse) error {
	handler := p.getRequestHandler(endpoint, requestId)
	if handler == nil {
		return ErrNotRegistered
	}

	handler.onResponse <- response
	return nil
}

func (p publisher) OnError(endpoint string, requestId string, err error) error {
	handler := p.getRequestHandler(endpoint, requestId)
	if handler == nil {
		return ErrNotRegistered
	}

	handler.onError <- err
	return nil
}

func (p publisher) ForwardRequest(endpoint string, timeout time.Duration, request HttpRequest) (*HttpResponse, error) {
	requestId := uuid.New().String()

	// Event if nobody is listening, we would like to log the event.
	p.log.LogRequest(requestId, endpoint, request)

	byEndpoint := p.endpoints[endpoint]
	if byEndpoint == nil {
		p.log.LogTimeout(requestId)
		return nil, ErrNotRegistered
	}

	handler := responeHandler{
		onResponse: make(chan HttpResponse),
		onError:    make(chan error),
	}

	byEndpoint.requests[requestId] = &handler
	byEndpoint.response(requestId, request)

	timer := time.After(timeout)

	defer delete(byEndpoint.requests, requestId)

	select {
	case response := <-handler.onResponse:
		p.log.LogResponse(requestId, response)
		return &response, nil
	case err := <-handler.onError:
		p.log.LogError(requestId, err)
		return nil, err
	case <-timer:
		p.log.LogTimeout(requestId)
		return nil, ErrTimeout
	}
}

func (p publisher) getRequestHandler(endpoint string, requestId string) *responeHandler {
	registration := p.endpoints[endpoint]
	if registration == nil {
		return nil
	}

	return registration.requests[requestId]
}
