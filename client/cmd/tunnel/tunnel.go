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

var (
	TABLE_COLUMN_WIDTH = 30
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

		client, err := api.GetClient()
		if err != nil {
			fmt.Printf("Error: Failed to retrieve configuration. %v\n", err)
			os.Exit(1)
			return
		}

		fmt.Printf("1")

		defer client.Connection.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Hour)
		defer cancel()

		stream, err := client.Service.Subscribe(ctx)
		if err != nil {
			fmt.Printf("Error: Failed to subscribe to stream. %v\n", err)
			os.Exit(1)
			return
		}

		subscribeMessage := &tunnel.WebhookMessage{
			TestMessageType: &tunnel.WebhookMessage_Subscribe{
				Subscribe: &tunnel.SubscribeRequest{
					Endpoint: &endpoint,
				},
			},
		}

		err = stream.Send(subscribeMessage)
		if err != nil {
			fmt.Printf("Error: Failed to subscribe to server. Endpoint is probably already used. %v\n", err)
			return
		}

		localBase := args[1]

		fmt.Printf("WEBHOOK TUNNEL")
		fmt.Println()
		fmt.Printf("Forwarding from: 		%s\n", formatUrl(client.Config.Endpoint, endpoint))
		fmt.Printf("Forwarding to: 		  %s\n", localBase)
		fmt.Println()
		fmt.Println("HTTP Requests")
		fmt.Println("-------------")
		fmt.Println()

		for {
			requestMessage, err := stream.Recv()
			if err != nil {
				fmt.Printf("Error: Connection closed by server: %v.\n", err)
				return
			}

			path := requestMessage.GetPath()

			fmt.Printf(" - %s %s",
				formatCell(requestMessage.GetMethod(), 10),
				formatCell(path, 30))

			var responseMessage *tunnel.WebhookMessage

			response, body, err := makeRequest(formatUrl(localBase, path), requestMessage)
			if err != nil {
				fmt.Printf("failed: could not create request: %v\n", err)

				message := err.Error()
				responseMessage = &tunnel.WebhookMessage{
					TestMessageType: &tunnel.WebhookMessage_Error{
						Error: &tunnel.HttpError{
							RequestId: requestMessage.RequestId,
							Error:     &message,
						},
					},
				}
			} else {
				fmt.Printf("%d %s\n", response.StatusCode, http.StatusText(response.StatusCode))

				responseHeaders := make(map[string]*tunnel.HttpHeaderValues, 0)
				for header, v := range response.Header {
					responseHeaders[header] = &tunnel.HttpHeaderValues{Values: v}
				}

				status := int32(response.StatusCode)
				responseMessage = &tunnel.WebhookMessage{
					TestMessageType: &tunnel.WebhookMessage_Response{
						Response: &tunnel.HttpResponse{
							Body:      body,
							Status:    &status,
							Headers:   headersToLocal(response),
							RequestId: requestMessage.RequestId,
						},
					},
				}
			}

			err = stream.Send(responseMessage)
			if err != nil {
				fmt.Printf("Error: Failed to send response to server. %v\n", err)
				return
			}
		}
	},
}

func makeRequest(localUrl string, requestMessage *tunnel.HttpRequest) (*http.Response, []byte, error) {
	requestBody := bytes.NewReader(requestMessage.GetBody())

	request, err := http.NewRequest(*requestMessage.Method, localUrl, requestBody)
	if err != nil {
		return nil, nil, err
	}

	for header, v := range requestMessage.GetHeaders() {
		for _, value := range v.GetValues() {
			request.Header.Add(header, value)
		}
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, nil, err
	}

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, nil, err
	}

	return response, responseBody, nil
}

func headersToLocal(response *http.Response) map[string]*tunnel.HttpHeaderValues {
	result := make(map[string]*tunnel.HttpHeaderValues, 0)

	for header, v := range response.Header {
		result[header] = &tunnel.HttpHeaderValues{Values: v}
	}

	return result
}

func formatCell(source string, max int) string {
	length := len(source)
	if length > max {
		return source[0:(max-3)] + "..."
	} else {
		for i := length; i < max; i++ {
			source += " "
		}

		return source
	}
}

func formatUrl(baseUrl string, paths ...string) string {
	url := strings.TrimSuffix(baseUrl, "/")

	for _, path := range paths {
		url += "/"
		url += strings.TrimPrefix(path, "/")
	}

	return url
}
