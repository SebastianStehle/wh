package api

import (
	"context"
	"fmt"
	"wh/cli/api/tunnel"
	"wh/cli/config"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type Client struct {
	Service    tunnel.WebhookServiceClient
	Config     *config.Server
	Connection *grpc.ClientConn
}

func GetClient() (*Client, context.Context, error) {
	server, err := config.GetServer()
	if server == nil {
		return nil, nil, err
	}

	connection, err := grpc.NewClient("localhost:5000", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect %v", err)
	}

	client := &Client{
		Service:    tunnel.NewWebhookServiceClient(connection),
		Config:     server,
		Connection: connection,
	}

	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", server.ApiKey)

	return client, ctx, nil
}
