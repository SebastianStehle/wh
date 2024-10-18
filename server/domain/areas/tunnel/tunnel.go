package tunnel

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	generated "wh/domain/areas/tunnel/api/tunnel"
	"wh/domain/publish"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var (
	EventOrigin = 13
)

type Stream = grpc.BidiStreamingServer[generated.ClientMessage, generated.ServerMessage]

type tunnelServer struct {
	logger    *zap.Logger
	publisher publish.Publisher
	generated.UnimplementedWebhookServiceServer
}

func NewTunnelServer(publisher publish.Publisher, logger *zap.Logger) generated.WebhookServiceServer {
	return &tunnelServer{logger: logger, publisher: publisher}
}

func (s *tunnelServer) Subscribe(stream Stream) error {
	s.logger.Info("Tunnel opened by client.")

	// Use one channel per type to have a type safe behavior.
	clientError := make(chan *generated.TransportError)
	requestData := make(chan publish.HttpRequestData)
	requestStart := make(chan *publish.TunneledRequest)
	responseData := make(chan *generated.ResponseData)
	responseStart := make(chan *generated.ResponseStart)
	serverError := make(chan publish.HttpError)
	unsubscribed := make(chan bool)

	// Have a separate closed channel that is closed by the sender to avoid deadlocks.
	closed := make(chan bool)

	endpoint := ""
	defer func() {
		s.logger.Info("Tunnel closes by client.")

		// Ensure that the goroutine completes when we are done with the tunnel.
		// There is no guarantee that the channel stil has receivers, if it has already been completed.
		unsubscribed <- true

		s.publisher.Unsubscribe(endpoint)
	}()

	go func() {
		// There is no weak map yet, therefore ensure to clean it up.
		requests := make(map[string]*publish.TunneledRequest)

		for {
			select {
			case <-unsubscribed:
				close(closed)
				return
			case msg := <-requestStart:
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

				// Only add the requests to the pending list when the request start has been sent successfully.
				requests[msg.RequestId] = msg

				s.logger.Info("Forwarding request to client.",
					zap.String("input.endpoint", endpoint),
					zap.String("input.method", request.Method),
					zap.String("input.path", request.Path),
				)

			case msg := <-requestData:
				m := &generated.ServerMessage{
					TestMessageType: &generated.ServerMessage_RequestData{
						RequestData: &generated.RequestData{
							RequestId: &msg.Request.RequestId,
							Data:      msg.Data,
							Completed: &msg.Completed,
						},
					},
				}

				if !s.sendMessage(stream, msg.Request, m, true) {
					// An error always terminates the request.
					delete(requests, msg.Request.RequestId)
				}

			case msg := <-responseStart:
				t, ok := requests[msg.GetRequestId()]
				if !ok {
					s.logUnknownRequest(msg.GetRequestId())
					break
				}

				t.EmitResponse(EventOrigin, fromHeaders(msg.GetHeaders()), msg.GetStatus())

			case msg := <-responseData:
				t, ok := requests[msg.GetRequestId()]
				if !ok {
					s.logUnknownRequest(msg.GetRequestId())
					break
				}

				t.EmitResponseData(EventOrigin, msg.GetData(), msg.GetCompleted())

				if msg.GetCompleted() {
					// Default completion.
					delete(requests, t.RequestId)
				}

			case msg := <-serverError:
				m := &generated.ServerMessage{
					TestMessageType: &generated.ServerMessage_Error{
						Error: &generated.TransportError{
							RequestId: &msg.Request.RequestId,
							Error:     toError(msg.Error),
							Timeout:   &msg.Timeout,
						},
					},
				}

				// An error always terminates the request.
				delete(requests, msg.Request.RequestId)

				s.sendMessage(stream, msg.Request, m, false)

			case msg := <-clientError:
				t, ok := requests[msg.GetRequestId()]
				if !ok {
					s.logUnknownRequest(msg.GetRequestId())
					break
				}

				// An error always terminates the request.
				delete(requests, t.RequestId)

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

			if err := s.publisher.Subscribe(endpoint, func(request *publish.TunneledRequest) {
				requestStart <- request

				request.OnRequestData(EventOrigin, func(msg publish.HttpRequestData) {
					select {
					case requestData <- msg:
					case <-closed:
					}
				})

				request.OnError(EventOrigin, func(msg publish.HttpError) {
					select {
					case serverError <- msg:
					case <-closed:
					}
				})
			}); err != nil {
				return err
			}
			continue
		}

		if len(endpoint) == 0 {
			return fmt.Errorf("not subscribed yet")
		}

		if s := message.GetResponseStart(); s != nil {
			select {
			case responseStart <- s:
			case <-closed:
			}
		}

		if d := message.GetResponseData(); d != nil {
			select {
			case responseData <- d:
			case <-closed:
			}
		}

		if e := message.GetError(); e != nil {
			select {
			case clientError <- e:
			case <-closed:
			}
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
		s.logger.Error("Could not send request to client.",
			zap.Error(err),
		)

		if emit {
			t.EmitError(EventOrigin, err, false)
		}

		return false
	}

	return true
}

func toHeaders(source http.Header) map[string]*generated.HttpHeaderValues {
	result := make(map[string]*generated.HttpHeaderValues, len(source))
	for k, v := range source {
		result[k] = &generated.HttpHeaderValues{Values: v}
	}

	return result
}

func fromHeaders(source map[string]*generated.HttpHeaderValues) http.Header {
	result := make(http.Header, len(source))
	for k, v := range source {
		result[k] = v.Values
	}

	return result
}

func toError(err error) *string {
	result := ""
	if err != nil {
		result = err.Error()
	}

	return &result
}
