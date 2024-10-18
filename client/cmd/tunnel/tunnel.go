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

		clientError := make(chan HttpError)
		requestData := make(chan *tunnel.RequestData)
		requestStart := make(chan *tunnel.RequestStart)
		responseData := make(chan HttpResponseData)
		responseStart := make(chan HttpResponseStart)
		serverError := make(chan *tunnel.TransportError)
		unregister := make(chan *TunneledRequest)

		go func() {
			// This map is only used in this goroutine, therefore we don't have to send updates.
			requests := make(map[string]*TunneledRequest)

			for {
				select {
				case msg := <-requestStart:
					request := NewTunneledRequest(localBase,
						msg.GetRequestId(),
						msg.GetMethod(),
						msg.GetPath(),
						fromHeaders(msg.GetHeaders()))

					printStatus(request, "Started")

					request.OnResponseStart(func(msg HttpResponseStart) {
						responseStart <- msg
					})

					request.OnResponseData(func(msg HttpResponseData) {
						responseData <- msg
					})

					request.OnError(func(msg HttpError) {
						clientError <- msg
					})

					// Register the request immediately, because the actual consecutive request might arrive immediately.
					requests[request.RequestId] = request

					// Run the request in parallel to other requests.
					go func() {
						defer func() {
							unregister <- request
						}()

						request.Run(ctx, 1*time.Hour)
					}()

				case msg := <-unregister:
					// There are no weak refs in golang, therefore remove the completed request.
					delete(requests, msg.RequestId)

				case msg := <-requestData:
					t, ok := requests[msg.GetRequestId()]
					if !ok {
						break
					}

					t.WriteRequestData(msg.GetData(), msg.GetCompleted())

				case msg := <-responseStart:
					t, ok := requests[msg.Request.RequestId]
					if !ok {
						break
					}

					m := &tunnel.ClientMessage{
						TestMessageType: &tunnel.ClientMessage_ResponseStart{
							ResponseStart: &tunnel.ResponseStart{
								RequestId: &t.RequestId,
								Headers:   toHeaders(msg.Headers),
								Status:    &msg.Status,
							},
						},
					}

					if err := stream.Send(m); err != nil {
						printStatus(t, "Error: Failed to send request to server. %v", err)
					}

				case msg := <-responseData:
					t, ok := requests[msg.Request.RequestId]
					if !ok {
						break
					}

					m := &tunnel.ClientMessage{
						TestMessageType: &tunnel.ClientMessage_ResponseData{
							ResponseData: &tunnel.ResponseData{
								RequestId: &t.RequestId,
								Data:      msg.Data,
								Completed: &msg.Completed,
							},
						},
					}

					if err := stream.Send(m); err != nil {
						printStatus(t, "Error: Failed to send request to server. %v", err)
					}

				case msg := <-clientError:
					t, ok := requests[msg.Request.RequestId]
					if !ok {
						break
					}

					m := &tunnel.ClientMessage{
						TestMessageType: &tunnel.ClientMessage_Error{
							Error: &tunnel.TransportError{
								RequestId: &t.RequestId,
								Error:     toError(msg.Error),
								Timeout:   &msg.Timeout,
							},
						},
					}

					if err := stream.Send(m); err != nil {
						printStatus(t, "Error: Failed with client error. %v", msg.Error)
					}

				case msg := <-serverError:
					t, ok := requests[msg.GetRequestId()]
					if !ok {
						break
					}

					t.cancel()

					if msg.GetTimeout() {
						printStatus(t, "Error: Failed with server timeout")
					} else {
						printStatus(t, "Error: Failed with server error. %s", msg.GetError())
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

			if s := serverMessage.GetRequestStart(); s != nil {
				requestStart <- s
			}

			if d := serverMessage.GetRequestData(); d != nil {
				requestData <- d
			}

			if e := serverMessage.GetError(); e != nil {
				serverError <- e
			}
		}
	},
}

func printStatus(request *TunneledRequest, format string, a ...any) {
	requestPath := request.Path

	// Both paths are technically not the same, but it looks weird.
	if requestPath == "" {
		requestPath = "/"
	}

	prefix := fmt.Sprintf(" - %s %s ",
		formatCell(request.Method, 10),
		formatCell(requestPath, 30))

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
