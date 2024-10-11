package tunnel

import (
	"context"
	"fmt"
	"os"
	"time"
	"wh/cli/api"
	"wh/cli/api/tunnel"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
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

		serverError := make(chan *tunnel.TransportError)
		requestStart := make(chan *tunnel.RequestStart)
		requestData := make(chan *tunnel.RequestData)
		responseStart := make(chan HttpResponseStart)
		responseData := make(chan HttpResponseData)
		tunnelError := make(chan HttpError)

		ch := make(chan interface{})
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
						transportToHttp(msg.GetHeaders()))

					printStatus(request, "Started")

					// Run the request in another go-routine.
					go request.Run(ctx, 1*time.Hour)

					// Register the request, so we can send updates to it.
					requests[request.RequestId] = request

				case msg := <-requestData:
					req, ok := requests[msg.GetRequestId()]
					if !ok {
						break
					}

					req.AppendRequestData(msg.GetData(), msg.GetCompleted())

				case msg := <-responseStart:
					m := &tunnel.ClientMessage{
						TestMessageType: &tunnel.ClientMessage_ResponseStart{
							ResponseStart: &tunnel.ResponseStart{
								RequestId: &msg.Request.RequestId,
								Headers:   headersToGrpc(msg.Headers),
								Status:    &msg.Status,
							},
						},
					}

					sendResponse(msg.Request, m, stream, requests)

				case msg := <-responseData:
					_, ok := requests[msg.Request.RequestId]
					if !ok {
						break
					}

					m := &tunnel.ClientMessage{
						TestMessageType: &tunnel.ClientMessage_ResponseData{
							ResponseData: &tunnel.ResponseData{
								RequestId: &msg.Request.RequestId,
								Data:      msg.Data,
								Completed: &msg.Completed,
							},
						},
					}

					if msg.Completed {
						delete(requests, msg.Request.RequestId)
					}

					sendResponse(msg.Request, m, stream, requests)

				case msg := <-tunnelError:
					_, ok := requests[msg.Request.RequestId]
					if !ok {
						break
					}

					m := &tunnel.ClientMessage{
						TestMessageType: &tunnel.ClientMessage_Error{
							Error: &tunnel.TransportError{
								RequestId: &msg.Request.RequestId,
								Error:     errorToGrpc(msg.Error),
								Timeout:   &msg.Timeout,
							},
						},
					}

					sendResponse(msg.Request, m, stream, requests)
				}
			}
		}()

		for {
			serverMessage, e := stream.Recv()
			if e != nil {
				fmt.Printf("Error: Connection closed by server: %v.\n", e)
				return
			}

			start := serverMessage.GetRequestStart()
			if start != nil {
				select {
				case requestStart <- start:
				default:
				}
			}

			data := serverMessage.GetRequestData()
			if data != nil {
				select {
				case requestData <- data:
				default:
				}
			}

			e := serverMessage.GetError()
			if e != nil {
				select {
				case serverError <- e:
				default:
				}
			}
		}
	},
}

func sendResponse(request *TunneledRequest, m *tunnel.ClientMessage, stream grpc.BidiStreamingClient[tunnel.ClientMessage, tunnel.ServerMessage], requests map[string]*TunneledRequest) {
	err := stream.Send(m)
	if err != nil {
		printStatus(request, "Error: Failed to send response to server. %v\n", err)

		// Remove the request first, consecutive cancellations are just ignored.
		delete(requests, request.RequestId)
		request.Cancel()
	}
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
