package tunnel

import (
	"io"
	generated "wh/domain/areas/tunnel/api/tunnel"
	"wh/domain/publish"

	"google.golang.org/grpc"
)

type cliServer struct {
	publisher publish.Publisher
	generated.UnimplementedWebhookServiceServer
}

func NewCliServer(publisher publish.Publisher) generated.WebhookServiceServer {
	return &cliServer{publisher: publisher}
}

func (s *cliServer) Subscribe(stream grpc.BidiStreamingServer[generated.WebhookMessage, generated.HttpRequest]) error {
	endpoint := ""
	for {
		message, err := stream.Recv()
		if err == io.EOF {
			return nil
		}

		if err != nil {
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

			if newEndpoint != "" {
				s.publisher.Subscribe(newEndpoint, func(requestId string, request publish.HttpRequest) {
					requestMessage := &generated.HttpRequest{
						RequestId: &requestId,
						Path:      &request.Path,
						Method:    &request.Method,
						Headers:   toHeaders(request.Headers),
						Body:      request.Body,
					}

					stream.Send(requestMessage)
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
