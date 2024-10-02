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

type Stream = grpc.BidiStreamingServer[generated.ClientMessage, generated.ServerMessage]

type tunnelMessage[T any] struct {
	request publish.TunneledRequest
	message T
}

type complete struct {
}

type done struct {
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

	// We can only write to the stream from one go routing, therefore aggregate all messages in to one channel
	ch := make(chan interface{})

	endpoint := ""
	defer func() {
		s.logger.Info("Tunnel closes by client.")

		// Ensure that the goroutine completes when we are done with the tunnel.
		ch <- done{}

		s.publisher.Unsubscribe(endpoint)
	}()

	go func() {
		// There is no weak map yet, therefore ensure to clean it up.
		requests := make(map[string]publish.TunneledRequest)

		fmt.Printf("ABC")
		for e := range ch {
			switch m := e.(type) {
			case done:
				fmt.Printf("DONE")
				return
			case tunnelMessage[complete]:
				fmt.Printf("DONE")
				delete(requests, m.request.RequestId())

			case publish.TunneledRequest:
				fmt.Printf("S")
				requests[m.RequestId()] = m

				request := m.Request()
				requestId := m.RequestId()

				msg := &generated.ServerMessage{
					TestMessageType: &generated.ServerMessage_RequestStart{
						RequestStart: &generated.RequestStart{
							RequestId: &requestId,
							Endpoint:  &endpoint,
							Path:      &request.Path,
							Method:    &request.Method,
							Headers:   toHeaders(request.Headers),
						},
					},
				}

				if err := stream.Send(msg); err != nil {
					s.logger.Error("Could not send request start to client.",
						zap.Error(err),
					)

					m.EmitClientError(err)
					break
				}

				s.logger.Info("Forwarding request to client.",
					zap.String("input.endpoint", endpoint),
					zap.String("input.method", request.Method),
					zap.String("input.path", request.Path),
				)

			case tunnelMessage[publish.HttpRequestData]:
				fmt.Printf("\nFOO <%s>\n", m.request.RequestId())
				requestId := m.request.RequestId()
				msg := &generated.ServerMessage{
					TestMessageType: &generated.ServerMessage_RequestData{
						RequestData: &generated.RequestData{
							RequestId: &requestId,
							Data:      m.message.Data,
							Completed: &m.message.Completed,
						},
					},
				}

				s.sendMessage(stream, m.request, msg)

			case generated.ResponseStart:
				t, ok := requests[m.GetRequestId()]
				if !ok {
					s.logUnknownRequest(m.GetRequestId())
					break
				}

				msg := publish.HttpResponseStart{
					Headers: fromHeaders(m.GetHeaders()),
					Status:  m.GetStatus(),
				}

				t.EmitResponse(msg)

			case generated.ResponseData:
				t, ok := requests[m.GetRequestId()]
				if !ok {
					s.logUnknownRequest(m.GetRequestId())
					break
				}

				msg := publish.HttpResponseData{
					Data:      m.GetData(),
					Completed: m.GetCompleted(),
				}

				t.EmitResponseData(msg)

			case generated.TransportError:
				t, ok := requests[m.GetRequestId()]
				if !ok {
					s.logUnknownRequest(m.GetRequestId())
					break
				}

				t.EmitClientError(errors.New(m.GetError()))
			default:
				fmt.Printf("INVALID %T\n", m)
			}

		}
		fmt.Printf("DEF")
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

			if err := s.publisher.Subscribe(endpoint, func(t publish.TunneledRequest) {
				ch <- t
				fmt.Printf("REQUEST STARTED\n")
				go func() {
					defer func() {
						ch <- tunnelMessage[complete]{request: t}
					}()

					for e := range t.Events() {
						switch m := e.(type) {
						case publish.HttpRequestData:
							fmt.Printf("INVALID2 %T\n", m)
							ch <- tunnelMessage[publish.HttpRequestData]{request: t, message: m}
						default:
							fmt.Printf("INVALID %T\n", m)
						}
					}
					fmt.Printf("REQUEST CLOSED\n")
				}()
			}); err != nil {
				return err
			}
			continue
		}

		if len(endpoint) == 0 {
			return fmt.Errorf("not subscribed yet")
		}

		responseStart := message.GetResponseStart()
		if responseStart != nil {
			ch <- *responseStart
		}

		responseChunk := message.GetResponseData()
		if responseChunk != nil {
			ch <- *responseChunk
		}

		clientError := message.GetError()
		if clientError != nil {
			ch <- *clientError
		}
	}
}

func (s *tunnelServer) logUnknownRequest(requestId string) {
	s.logger.Error("Cannot find request.",
		zap.String("requestId", requestId),
	)
}

func (s *tunnelServer) sendMessage(stream Stream, t publish.TunneledRequest, msg *generated.ServerMessage) {
	if err := stream.Send(msg); err != nil {
		s.logger.Error("Could not send request start to client.",
			zap.Error(err),
		)

		t.EmitClientError(err)
	}
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
