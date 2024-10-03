package tunnel

import (
	"errors"
	"fmt"
	"io"
	generated "wh/domain/areas/tunnel/api/tunnel"
	"wh/domain/publish"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var (
	EventOrigin = 13
)

type Stream = grpc.BidiStreamingServer[generated.ClientMessage, generated.ServerMessage]

type sender struct {
	clientError   chan *generated.TransportError
	complete      chan tunnelMessage[complete]
	done          chan bool
	requestData   chan tunnelMessage[publish.HttpData]
	requestStart  chan *publish.TunneledRequest
	responseData  chan *generated.ResponseData
	responseStart chan *generated.ResponseStart
	serverError   chan tunnelMessage[publish.HttpError]
}

type complete struct{}

type tunnelMessage[T any] struct {
	request *publish.TunneledRequest
	payload T
}

type listener struct {
	sender  sender
	request *publish.TunneledRequest
}

func (l listener) OnResponseStart(msg publish.HttpResponseStart) {
}

func (l listener) OnResponseData(msg publish.HttpData) {
}

func (l listener) OnRequestData(msg publish.HttpData) {
	l.sender.requestData <- tunnelMessage[publish.HttpData]{request: l.request, payload: msg}
}

func (l listener) OnError(msg publish.HttpError) {
	l.sender.serverError <- tunnelMessage[publish.HttpError]{request: l.request, payload: msg}
}

func (l listener) OnComplete() {
	l.sender.complete <- tunnelMessage[complete]{request: l.request}
}

type tunnelServer struct {
	logger    *zap.Logger
	publisher publish.Publisher
	requests  map[string]publish.TunneledRequest
	generated.UnimplementedWebhookServiceServer
}

func NewTunnelServer(publisher publish.Publisher, logger *zap.Logger) generated.WebhookServiceServer {
	return &tunnelServer{logger: logger, publisher: publisher}
}

func (s *tunnelServer) Subscribe(stream Stream) error {
	s.logger.Info("Tunnel opened by client.")

	// Use one channel per type to have a type safe behavior.
	sender := sender{
		clientError:   make(chan *generated.TransportError),
		complete:      make(chan tunnelMessage[complete]),
		requestData:   make(chan tunnelMessage[publish.HttpData]),
		requestStart:  make(chan *publish.TunneledRequest),
		responseData:  make(chan *generated.ResponseData),
		responseStart: make(chan *generated.ResponseStart),
		serverError:   make(chan tunnelMessage[publish.HttpError]),
	}

	endpoint := ""
	defer func() {
		s.logger.Info("Tunnel closes by client.")

		// Ensure that the goroutine completes when we are done with the tunnel.
		sender.done <- true
		s.publisher.Unsubscribe(endpoint)
	}()

	go func() {
		// There is no weak map yet, therefore ensure to clean it up.
		requests := make(map[string]*publish.TunneledRequest)

		for {
			select {
			case <-sender.done:
				return
			case msg := <-sender.requestStart:
				requests[msg.RequestId] = msg
				request := msg.Request

				m := &generated.ServerMessage{
					TestMessageType: &generated.ServerMessage_RequestStart{
						RequestStart: &generated.RequestStart{
							RequestId: &msg.RequestId,
							Endpoint:  &endpoint,
							Path:      &request.Path,
							Method:    &request.Method,
							Headers:   toHeaders(request.Headers),
						},
					},
				}

				if !s.sendMessage(stream, msg, m, true) {
					break
				}

				s.logger.Info("Forwarding request to client.",
					zap.String("input.endpoint", endpoint),
					zap.String("input.method", request.Method),
					zap.String("input.path", request.Path),
				)

			case msg := <-sender.requestData:
				m := &generated.ServerMessage{
					TestMessageType: &generated.ServerMessage_RequestData{
						RequestData: &generated.RequestData{
							RequestId: &msg.request.RequestId,
							Data:      msg.payload.Data,
							Completed: &msg.payload.Completed,
						},
					},
				}

				s.sendMessage(stream, msg.request, m, true)

			case msg := <-sender.serverError:
				errorText := msg.payload.Error.Error()

				m := &generated.ServerMessage{
					TestMessageType: &generated.ServerMessage_Error{
						Error: &generated.TransportError{
							RequestId: &msg.request.RequestId,
							Error:     &errorText,
							Timeout:   &msg.payload.Timeout,
						},
					},
				}

				s.sendMessage(stream, msg.request, m, false)

			case msg := <-sender.responseStart:
				t, ok := requests[msg.GetRequestId()]
				if !ok {
					s.logUnknownRequest(msg.GetRequestId())
					break
				}

				t.EmitResponse(EventOrigin, fromHeaders(msg.GetHeaders()), msg.GetStatus())

			case msg := <-sender.responseData:
				t, ok := requests[msg.GetRequestId()]
				if !ok {
					s.logUnknownRequest(msg.GetRequestId())
					break
				}

				t.EmitResponseData(EventOrigin, msg.GetData(), msg.GetCompleted())

			case msg := <-sender.clientError:
				t, ok := requests[msg.GetRequestId()]
				if !ok {
					s.logUnknownRequest(msg.GetRequestId())
					break
				}

				t.EmitError(EventOrigin, errors.New(msg.GetError()), true)
			}
		}
	}()

	for {
		message, err := stream.Recv()
		if err == io.EOF {
			s.logger.Info("Tunnel stream closed by client.")
			return nil
		}

		if err != nil {
			s.logger.Error("Tunnel stream interrupted with error.",
				zap.Error(err),
			)
			return err
		}

		subscribeMessage := message.GetSubscribe()
		if subscribeMessage != nil {
			if len(endpoint) > 0 {
				return fmt.Errorf("you can only subscribe once. Current endpoint %s", endpoint)
			}

			endpoint = subscribeMessage.GetEndpoint()

			if err := s.publisher.Subscribe(endpoint, func(tunneled *publish.TunneledRequest) {
				sender.requestStart <- tunneled

				listener := listener{sender: sender, request: tunneled}
				tunneled.Listen(EventOrigin, listener)
			}); err != nil {
				return err
			}
			continue
		}

		if len(endpoint) == 0 {
			return fmt.Errorf("not subscribed yet")
		}

		start := message.GetResponseStart()
		if start != nil {
			sender.responseStart <- start
		}

		data := message.GetResponseData()
		if data != nil {
			sender.responseData <- data
		}

		e := message.GetError()
		if e != nil {
			sender.clientError <- e
		}
	}
}

func (s *tunnelServer) logUnknownRequest(requestId string) {
	s.logger.Error("Cannot find request.",
		zap.String("requestId", requestId),
	)
}

func (s *tunnelServer) sendMessage(stream Stream, t *publish.TunneledRequest, msg *generated.ServerMessage, emit bool) bool {
	if err := stream.Send(msg); err != nil {
		s.logger.Error("Could not send request start to client.",
			zap.Error(err),
		)

		if emit {
			t.EmitError(EventOrigin, err, false)
		}

		return false
	}

	return true
}

func toHeaders(source map[string][]string) map[string]*generated.HttpHeaderValues {
	headers := make(map[string]*generated.HttpHeaderValues)

	for k, v := range source {
		headers[k] = &generated.HttpHeaderValues{Values: v}
	}

	return headers
}

func fromHeaders(source map[string]*generated.HttpHeaderValues) map[string][]string {
	headers := make(map[string][]string)

	for k, v := range source {
		headers[k] = v.Values
	}

	return headers
}
