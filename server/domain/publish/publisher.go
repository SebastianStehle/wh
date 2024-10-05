package publish

import (
	"errors"
	"sync"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type publisher struct {
	endpoints map[string]func(*TunneledRequest)
	buckets   Buckets
	lock      sync.RWMutex
	logger    *zap.Logger
	store     Store
}

// ErrAlreadyRegistered There is already a request handler.
var ErrAlreadyRegistered = errors.New("AlreadyRegistered")

// ErrNotRegistered There is no listener.
var ErrNotRegistered = errors.New("NotRegistered")

type Publisher interface {
	Subscribe(endpoint string, handler func(*TunneledRequest)) error

	Unsubscribe(endpoint string)

	ForwardRequest(endpoint string, request HttpRequestStart) (*TunneledRequest, error)
}

func NewPublisher(store Store, buckets Buckets, logger *zap.Logger) Publisher {
	return &publisher{
		endpoints: make(map[string]func(*TunneledRequest)),
		buckets:   buckets,
		lock:      sync.RWMutex{},
		logger:    logger,
		store:     store,
	}
}

func (p *publisher) Unsubscribe(endpoint string) {
	// Ensure that only a single thread can access the map
	p.lock.Lock()
	defer p.lock.Unlock()

	delete(p.endpoints, endpoint)
}

func (p *publisher) Subscribe(endpoint string, handler func(request *TunneledRequest)) error {
	// Ensure that only a single thread can access the map
	p.lock.Lock()
	defer p.lock.Unlock()

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

	// Record the request details and store them in a file and database.
	rec := NewRecorder(req, p.store, p.buckets, p.logger)
	rec.Listen(req)

	// Publish the request first, so that we can receive events.
	handler(req)

	return req, nil
}

func (p *publisher) getHandler(endpoint string) (func(*TunneledRequest), error) {
	// Ensure that only a single thread can access the map
	p.lock.Lock()
	defer p.lock.Unlock()

	byEndpoint, ok := p.endpoints[endpoint]
	if !ok || byEndpoint == nil {
		return nil, ErrNotRegistered
	}

	return byEndpoint, nil
}
