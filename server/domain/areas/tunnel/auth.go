package tunnel

import (
	"context"

	"wh/"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func Authorize(auth Authenticator, ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Errorf(codes.InvalidArgument, "Retrieving metadata is failed")
	}

	authHeader, ok := md["authorization"]
	if !ok {
		return status.Errorf(codes.Unauthenticated, "Authorization token is not supplied")
	}

	token := authHeader[0]
	// validateToken function validates the token
	err := validateToken(token)

	if err != nil {
		return status.Errorf(codes.Unauthenticated, err.Error())
	}

	return nil
}
