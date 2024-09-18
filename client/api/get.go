package api

import (
	"fmt"
	"wh/cli/api/tunnel"
	"wh/cli/config"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	Service    tunnel.WebhookServiceClient
	Config     *config.Server
	Connection *grpc.ClientConn
}

func GetClient() (*Client, error) {
	server, err := config.GetServer()
	if server == nil {
		return nil, err
	}

	connection, err := grpc.NewClient("localhost:5000", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect %v", err)
	}

	client := &Client{
		Service:    tunnel.NewWebhookServiceClient(connection),
		Config:     server,
		Connection: connection,
	}

	return client, nil
}
