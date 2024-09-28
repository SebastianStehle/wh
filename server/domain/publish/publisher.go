package publish

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/viper"
)

type publisher struct {
	endpoints map[string]func(TunneledRequest)
	buckets   Buckets
	log       Log
}

type ResponseEvent struct {
	error   error
	started *HttpResponseStart
	chunk   *HttpRequestChunk
}

// ErrTimeout There was no response within the defined duration.
var ErrTimeout = errors.New("RequestTimeout")

// ErrAlreadyRegistered There is already a request handler.
var ErrAlreadyRegistered = errors.New("AlreadyRegistered")

// ErrNotRegistered There is no listener.
var ErrNotRegistered = errors.New("NotRegistered")

type Publisher interface {
	Subscribe(endpoint string, handler func(TunneledRequest)) error

	Unsubscribe(endpoint string)

	ForwardRequest(endpoint string, timeout time.Duration, request HttpRequestStart) (TunneledRequest, error)

	GetEntries(etag int64) ([]LogEntry, int64)
}

func NewPublisher(config *viper.Viper) Publisher {
	maxSize := config.GetInt("log.maxSize")
	maxEntries := config.GetInt("log.maxEntries")

	log := NewLog(maxSize, maxEntries)

	return &publisher{
		endpoints: make(map[string]func(TunneledRequest)),
		log:       log
		buckets: nil,
	}
}

func (p publisher) GetEntries(etag int64) ([]LogEntry, int64) {
	return p.log.GetEntries(etag)
}

func (p publisher) Unsubscribe(endpoint string) {
	delete(p.endpoints, endpoint)
}

func (p publisher) Subscribe(endpoint string, handler func(request TunneledRequest)) error {
	registration := p.endpoints[endpoint]
	if registration != nil {
		return ErrAlreadyRegistered
	}

	p.endpoints[endpoint] = handler
	return nil
}

func (p publisher) ForwardRequest(endpoint string, timeout time.Duration, request HttpRequestStart) (TunneledRequest, error) {
	requestId := uuid.New().String()

	// Event if nobody is listening, we would like to log the event.
	p.log.LogRequest(requestId, endpoint, request)

	byEndpoint := p.endpoints[endpoint]
	if p.endpoints[endpoint] == nil {
		return nil, ErrNotRegistered
	}

	tunneledRequest := NewTunneledRequest(p.buckets, p.log, endpoint, timeout, requestId, request)
	tunneledRequest.Start()

	byEndpoint(tunneledRequest)

	return tunneledRequest, nil
}
