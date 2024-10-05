package tunnel

import (
	"context"
	"fmt"
	"os"
	"time"
	"wh/cli/api"
	"wh/cli/api/tunnel"

	"github.com/spf13/cobra"
)

var TunnelCmd = &cobra.Command{
	Use:   "tunnel <ENDPOINT> <LOCAL_URL>",
	Short: "Creates a tunnel with and endpoint",
	Long: `Pass in the endpoint and the local server:

Simple tunnel
	tunnel <endpoint> <local_server>

for example:
	tunnel users http://localhost:8080/users`,
	Args: cobra.MatchAll(cobra.ExactArgs(2), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		endpoint := args[0]

		client, ctx, err := api.GetClient()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
			return
		}

		defer func() {
			// There is very little we can do right now.
			_ = client.Connection.Close()
		}()

		ctx, cancel := context.WithTimeout(ctx, 4*time.Hour)
		defer cancel()

		stream, err := client.Service.Subscribe(ctx)
		if err != nil {
			fmt.Printf("Error: Failed to subscribe to stream. %v\n", err)
			os.Exit(1)
			return
		}

		subscribeMessage := &tunnel.ClientMessage{
			TestMessageType: &tunnel.ClientMessage_Subscribe{
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

		fmt.Println()
		fmt.Printf("WEBHOOK TUNNEL")
		fmt.Println()
		fmt.Println()
		fmt.Printf("Forwarding from:  %s\n", combineUrl(client.Config.Endpoint, "endpoints", endpoint))
		fmt.Printf("Forwarding to:    %s\n", localBase)
		fmt.Println()
		fmt.Println("HTTP Requests")
		fmt.Println("-------------")
		fmt.Println()

		ch := make(chan interface{})
		go func() {
			// This map is only used in this goroutine, therefore we don't have to send updates.
			requests := make(map[string]*tunneledRequest)

			for e := range ch {
				switch m := e.(type) {
				case tunnel.RequestStart:
					req, _ := newTunneledRequest(ctx, localBase, &m, ch)
					if err != nil {
						printStatus(m.GetMethod(), m.GetPath(), "Failed with error: %s", err.Error())
						break
					}

					printStatus(req.method, req.path, "Started")

					// Register the request, so we can send updates to it.
					requests[req.requestId] = req

				case tunnel.TransportError:
					req, ok := requests[m.GetRequestId()]
					if !ok {
						break
					}

					req.cancel()

				case tunnel.RequestData:
					req, ok := requests[m.GetRequestId()]
					if !ok {
						break
					}

					req.appendRequestData(m.GetData(), m.GetCompleted())

				case responseMessage:
					if m.completed {
						delete(requests, m.request.requestId)
					}

					err = stream.Send(m.response)
					if err != nil {
						fmt.Printf("Error: Failed to send response to server. %v\n", err)
					} else if m.status != "" {
						printStatus(m.request.method, m.request.path, m.status)
					}
				}
			}
		}()

		for {
			serverMessage, err := stream.Recv()
			if err != nil {
				fmt.Printf("Error: Connection closed by server: %v.\n", err)
				return
			}

			requestStart := serverMessage.GetRequestStart()
			if requestStart != nil {
				ch <- *requestStart
			}

			requestData := serverMessage.GetRequestData()
			if requestData != nil {
				ch <- *requestData
			}

			requestError := serverMessage.GetError()
			if requestError != nil {
				ch <- *requestError
			}
		}
	},
}

func printStatus(method string, path string, format string, a ...any) {
	prefix := fmt.Sprintf(" - %s %s ",
		formatCell(method, 10),
		formatCell(path, 30))

	fmt.Println(prefix + fmt.Sprintf(format, a...))
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
