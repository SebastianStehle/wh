package publish

import (
	"errors"
	"sync"

	"github.com/google/uuid"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type publisher struct {
	endpoints map[string]func(*TunneledRequest)
	buckets   Buckets
	lockObj   sync.RWMutex
	log       Log
	logger    *zap.Logger
}

// ErrAlreadyRegistered There is already a request handler.
var ErrAlreadyRegistered = errors.New("AlreadyRegistered")

// ErrNotRegistered There is no listener.
var ErrNotRegistered = errors.New("NotRegistered")

type Publisher interface {
	Subscribe(endpoint string, handler func(*TunneledRequest)) error

	Unsubscribe(endpoint string)

	ForwardRequest(endpoint string, request HttpRequestStart) (*TunneledRequest, error)

	GetEntries(etag int64) ([]LogEntry, int64)
}

func NewPublisher(config *viper.Viper, logger *zap.Logger, buckets Buckets) Publisher {
	maxSize := config.GetInt("logger.maxSize")
	maxEntries := config.GetInt("logger.maxEntries")

	log := NewLog(maxSize, maxEntries)

	return &publisher{
		endpoints: make(map[string]func(*TunneledRequest)),
		buckets:   buckets,
		lockObj:   sync.RWMutex{},
		log:       log,
		logger:    logger,
	}
}

func (p *publisher) GetEntries(etag int64) ([]LogEntry, int64) {
	return p.log.GetEntries(etag)
}

func (p *publisher) Unsubscribe(endpoint string) {
	// Ensure that only a single thread can access the thread
	p.lockObj.Lock()
	defer p.lockObj.Unlock()

	delete(p.endpoints, endpoint)
}

func (p *publisher) Subscribe(endpoint string, handler func(request *TunneledRequest)) error {
	registration, ok := p.endpoints[endpoint]
	if ok || registration != nil {
		return ErrAlreadyRegistered
	}

	p.endpoints[endpoint] = handler
	return nil
}

func (p *publisher) ForwardRequest(endpoint string, request HttpRequestStart) (*TunneledRequest, error) {
	requestId := uuid.New().String()

	handler, err := p.getHandler(endpoint)
	if err != nil {
		return nil, err
	}

	req := NewTunneledRequest(endpoint, requestId, request, p.logger)
	req.Listen(1001, NewLogListener(req, p.buckets, p.log, p.logger))
	// Publish the request first, so that we can receive events.
	handler(req)

	return req, nil
}

func (p *publisher) getHandler(endpoint string) (func(*TunneledRequest), error) {
	// Ensure that only a single thread can access the thread
	p.lockObj.Lock()
	defer p.lockObj.Unlock()

	byEndpoint, ok := p.endpoints[endpoint]
	if !ok || byEndpoint == nil {
		return nil, ErrNotRegistered
	}

	return byEndpoint, nil
}
