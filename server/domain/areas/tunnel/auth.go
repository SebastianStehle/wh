package tunnel

import (
	"context"
	"wh/domain/areas/auth"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func Authorize(authenticator auth.Authenticator, ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Errorf(codes.InvalidArgument, "Retrieving metadata is failed")
	}

	authHeader, ok := md["authorization"]
	if !ok {
		return status.Errorf(codes.Unauthenticated, "Authorization token is not supplied")
	}

	token := authHeader[0]
	if !authenticator.Validate(token) {
		return status.Errorf(codes.Unauthenticated, "Invalid API Key")
	}

	return nil
}
