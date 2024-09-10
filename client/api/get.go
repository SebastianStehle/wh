package api

import (
	"fmt"
	"wh/cli/api/tunnel"
	"wh/cli/config"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func GetClient() (tunnel.WebhookServiceClient, *grpc.ClientConn, error) {
	server, err := config.GetServer()
	if server == nil {
		return nil, nil, err
	}

	connection, err := grpc.NewClient(server.Endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect %v", err)
	}

	client := tunnel.NewWebhookServiceClient(connection)
	return client, connection, nil
}
