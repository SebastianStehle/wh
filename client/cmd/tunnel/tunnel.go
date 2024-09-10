package tunnel

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"wh/cli/api"
	"wh/cli/api/tunnel"

	"github.com/spf13/cobra"
)

var TunnelCmd = &cobra.Command{
	Use:   "tunnel",
	Short: "Creates a tunnel with and endpoint",
	Long: `Pass in the endpoint and the local server:

Simple tunnel
	tunnel <endpoint> <local_server>

for example:
	tunnel users http://localhost:8080/users`,
	Args: cobra.MatchAll(cobra.ExactArgs(2), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		endpoint := args[0]

		client, connection, err := api.GetClient()
		if err != nil {
			fmt.Printf("Error: Failed to retrieve configuration. %v\n", err)
			os.Exit(1)
			return
		}

		defer connection.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Hour)
		defer cancel()

		stream, err := client.Subscribe(ctx)
		if err != nil {
			fmt.Printf("Error: Failed to subscribe to stream. %v\n", err)
			os.Exit(1)
			return
		}

		localUrl := strings.TrimSuffix(args[0], "/")
		for {
			subscribeMessage := &tunnel.WebhookMessage{
				TestMessageType: &tunnel.WebhookMessage_Subscribe{
					Subscribe: &tunnel.SubscribeRequest{
						Endpoint: &endpoint,
					},
				},
			}

			err := stream.Send(subscribeMessage)
			if err != nil {
				fmt.Printf("Error: Failed to subscribe to server. Endpoint is probably already used. %v\n", err)
				return
			}

			requestMessage, err := stream.Recv()
			if err == io.EOF {
				fmt.Printf("Error: Connection closed by server. %v\n", err)
				return
			}

			url := localUrl
			url += "/"
			url += strings.TrimPrefix(requestMessage.GetPath(), "/")
			requestBody := bytes.NewReader(requestMessage.GetBody())

			req, err := http.NewRequest(*requestMessage.Method, url, requestBody)
			if err != nil {
				fmt.Printf("could not create request: %v.", err)
			}

			for header, v := range requestMessage.GetHeaders() {
				for _, value := range v.GetValues() {
					req.Header.Add(header, value)
				}
			}

			res, err := http.DefaultClient.Do(req)
			if err != nil {
				fmt.Printf("could not create request: %v.", err)
			}

			responseBody, err := io.ReadAll(res.Body)
			if err != nil {
				fmt.Printf("client: could not read response body: %s\n", err)
				os.Exit(1)
			}

			responseHeaders := make(map[string]*tunnel.HttpHeaderValues, 0)
			for header, v := range res.Header {
				responseHeaders[header] = &tunnel.HttpHeaderValues{Values: v}
			}

			status := int32(res.StatusCode)
			responseMessage := &tunnel.WebhookMessage{
				TestMessageType: &tunnel.WebhookMessage_Response{
					Response: &tunnel.HttpResponse{
						Body:      responseBody,
						Status:    &status,
						Headers:   headersToLocal(res),
						RequestId: requestMessage.RequestId,
					},
				},
			}

			err = stream.Send(responseMessage)
			if err != nil {
				fmt.Printf("Error: Failed to subscribe to server. Endpoint is probably already used. %v\n", err)
				return
			}
		}
	},
}

func headersToLocal(res *http.Response) map[string]*tunnel.HttpHeaderValues {
	result := make(map[string]*tunnel.HttpHeaderValues, 0)

	for header, v := range res.Header {
		result[header] = &tunnel.HttpHeaderValues{Values: v}
	}

	return result
}
