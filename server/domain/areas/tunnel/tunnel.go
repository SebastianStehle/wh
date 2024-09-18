package tunnel

import (
	"fmt"
	"io"
	generated "wh/domain/areas/tunnel/api/tunnel"
	"wh/domain/publish"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type tunnelServer struct {
	logger    *zap.Logger
	publisher publish.Publisher
	generated.UnimplementedWebhookServiceServer
}

func NewTunnelServer(publisher publish.Publisher, logger *zap.Logger) generated.WebhookServiceServer {
	return &tunnelServer{logger: logger, publisher: publisher}
}

func (s *tunnelServer) Subscribe(stream grpc.BidiStreamingServer[generated.WebhookMessage, generated.HttpRequest]) error {
	s.logger.Info("Tunnel opened by client.")
	endpoint := ""

	defer func() {
		if endpoint != "" {
			s.publisher.Unsubscribe(endpoint)
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
			var newEndpoint = subscribeMessage.GetEndpoint()
			if newEndpoint == endpoint {
				continue
			}

			if endpoint != "" {
				s.publisher.Unsubscribe(endpoint)
			}

			endpoint = newEndpoint

			if endpoint != "" {
				s.publisher.Subscribe(endpoint, func(requestId string, request publish.HttpRequest) {
					s.logger.Info("Forwarding request to client.",
						zap.String("input.endpoint", endpoint),
						zap.String("input.method", request.Method),
						zap.String("input.path", request.Path),
					)

					requestMessage := &generated.HttpRequest{
						RequestId: &requestId,
						Endpoint:  &endpoint,
						Path:      &request.Path,
						Method:    &request.Method,
						Headers:   toHeaders(request.Headers),
						Body:      request.Body,
					}

					if err := stream.Send(requestMessage); err != nil {
						s.logger.Error("Could not send request to client.",
							zap.Error(err),
						)
					}
				})
			}
		}

		responseMessage := message.GetResponse()
		if responseMessage != nil && endpoint != "" {
			response := publish.HttpResponse{
				Headers: fromHeaders(responseMessage.GetHeaders()),
				Status:  responseMessage.GetStatus(),
				Body:    responseMessage.GetBody(),
			}

			s.publisher.OnResponse(endpoint, *responseMessage.RequestId, response)
		}

		errorMessage := message.GetError()
		if errorMessage != nil && endpoint != "" {
			err := fmt.Errorf("error from CLI: %s", *errorMessage.Error)

			s.publisher.OnError(endpoint, *errorMessage.RequestId, err)
		}
	}
}

func toHeaders(source map[string]([]string)) map[string]*generated.HttpHeaderValues {
	headers := make(map[string]*generated.HttpHeaderValues)

	for k, v := range source {
		headers[k] = &generated.HttpHeaderValues{Values: v}
	}

	return headers
}

func fromHeaders(source map[string]*generated.HttpHeaderValues) map[string]([]string) {
	headers := make(map[string]([]string))

	for k, v := range source {
		headers[k] = v.Values
	}

	return headers
}
