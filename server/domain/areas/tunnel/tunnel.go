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
	request *publish.TunneledRequest
	msg     T
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
	done := make(chan bool)
	clientError := make(chan *generated.TransportError)
	complete := make(chan tunnelMessage[publish.Complete])
	requestData := make(chan tunnelMessage[publish.HttpData])
	requestStart := make(chan *publish.TunneledRequest)
	responseData := make(chan *generated.ResponseData)
	responseStart := make(chan *generated.ResponseStart)
	serverError := make(chan tunnelMessage[publish.HttpError])

	endpoint := ""
	defer func() {
		s.logger.Info("Tunnel closes by client.")

		// Ensure that the goroutine completes when we are done with the tunnel.
		done <- true

		s.publisher.Unsubscribe(endpoint)
	}()

	go func() {
		// There is no weak map yet, therefore ensure to clean it up.
		requests := make(map[string]*publish.TunneledRequest)

		for {
			select {
			case <-done:
				return
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

				if err := stream.Send(m); err != nil {
					s.logger.Error("Could not send request start to client.",
						zap.Error(err),
					)

					msg.EmitListenerError(err, false)
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
							RequestId: &msg.request.RequestId,
							Data:      msg.msg.Data,
							Completed: &msg.msg.Completed,
						},
					},
				}

				s.sendMessage(stream, msg.request, m)

			case msg := <-responseStart:
				t, ok := requests[msg.GetRequestId()]
				if !ok {
					s.logUnknownRequest(msg.GetRequestId())
					break
				}

				t.EmitResponse(fromHeaders(msg.GetHeaders()), msg.GetStatus())

			case msg := <-responseData:
				t, ok := requests[msg.GetRequestId()]
				if !ok {
					s.logUnknownRequest(msg.GetRequestId())
					break
				}

				t.EmitResponseData(msg.GetData(), msg.GetCompleted())

			case msg := <-clientError:
				t, ok := requests[msg.GetRequestId()]
				if !ok {
					s.logUnknownRequest(msg.GetRequestId())
					break
				}

				t.EmitInitiatorError(errors.New(msg.GetError()))
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
				requestStart <- tunneled
				go func() {
					for {
						select {
						case <-tunneled.ListenerComplete:
							complete <- tunnelMessage[publish.Complete]{request: tunneled}

						case msg := <-tunneled.RequestData:
							requestData <- tunnelMessage[publish.HttpData]{request: tunneled, msg: msg}

						case msg := <-tunneled.InitiatorError:
							serverError <- tunnelMessage[publish.HttpError]{request: tunneled, msg: msg}
						}
					}
				}()
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
			responseStart <- start
		}

		data := message.GetResponseData()
		if data != nil {
			responseData <- data
		}

		e := message.GetError()
		if e != nil {
			clientError <- e
		}
	}
}

func (s *tunnelServer) logUnknownRequest(requestId string) {
	s.logger.Error("Cannot find request.",
		zap.String("requestId", requestId),
	)
}

func (s *tunnelServer) sendMessage(stream Stream, t *publish.TunneledRequest, msg *generated.ServerMessage) {
	if err := stream.Send(msg); err != nil {
		s.logger.Error("Could not send request start to client.",
			zap.Error(err),
		)

		t.EmitListenerError(err, false)
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
