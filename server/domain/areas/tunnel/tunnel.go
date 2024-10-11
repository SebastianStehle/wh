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
	tunnelDone := make(chan publish.HttpComplete)
	tunnelError := make(chan publish.HttpError)
	unsubscribed := make(chan bool)

	endpoint := ""
	defer func() {
		s.logger.Info("Tunnel closes by client.")

		// Ensure that the goroutine completes when we are done with the tunnel.
		// There is no guarantee that the channel stil has receivers, if it has already been completed.
		select {
		case unsubscribed <- true:
		default:
			return
		}

		s.publisher.Unsubscribe(endpoint)
	}()

	go func() {
		// There is no weak map yet, therefore ensure to clean it up.
		requests := make(map[string]*publish.TunneledRequest)
		defer func() {
			fmt.Printf("DONE")
		}()

		for {
			select {
			case <-unsubscribed:
				return
			case msg := <-tunnelDone:
				delete(requests, msg.Request.RequestId)
			case msg := <-requestStart:
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

				s.sendMessage(stream, msg.Request, m, true)

			case msg := <-tunnelError:
				errorText := ""
				if msg.Error != nil {
					errorText = msg.Error.Error()
				}

				m := &generated.ServerMessage{
					TestMessageType: &generated.ServerMessage_Error{
						Error: &generated.TransportError{
							RequestId: &msg.Request.RequestId,
							Error:     &errorText,
							Timeout:   &msg.Timeout,
						},
					},
				}

				s.sendMessage(stream, msg.Request, m, false)

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

			case msg := <-clientError:
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

			if err := s.publisher.Subscribe(endpoint, func(request *publish.TunneledRequest) {
				// There is no guarantee that the channel stil has receivers, if it has already been completed.
				select {
				case requestStart <- request:
				default:
					return
				}

				request.OnRequestData(EventOrigin, func(msg publish.HttpRequestData) {
					// There is no guarantee that the channel stil has receivers, if it has already been completed.
					select {
					case requestData <- msg:
					default:
						return
					}
				})

				request.OnError(EventOrigin, func(msg publish.HttpError) {
					// There is no guarantee that the channel stil has receivers, if it has already been completed.
					select {
					case tunnelError <- msg:
					default:
						return
					}
				})

				request.OnComplete(EventOrigin, func(msg publish.HttpComplete) {
					// There is no guarantee that the channel stil has receivers, if it has already been completed.
					select {
					case tunnelDone <- msg:
					default:
						return
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

		start := message.GetResponseStart()
		if start != nil {
			// There is no guarantee that the channel stil has receivers, if it has already been completed.
			select {
			case responseStart <- start:
			default:
				return nil
			}
		}

		data := message.GetResponseData()
		if data != nil {
			// There is no guarantee that the channel stil has receivers, if it has already been completed.
			select {
			case responseData <- data:
			default:
				return nil
			}
		}

		e := message.GetError()
		if e != nil {
			// There is no guarantee that the channel stil has receivers, if it has already been completed.
			select {
			case clientError <- e:
			default:
				return nil
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
