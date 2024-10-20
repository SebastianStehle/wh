package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"syscall"
	"time"
	"wh/domain"
	"wh/domain/areas/api"
	"wh/domain/areas/auth"
	"wh/domain/areas/home"
	"wh/domain/areas/tunnel"
	generated "wh/domain/areas/tunnel/api/tunnel"
	"wh/domain/publish"
	"wh/infrastructure/configuration"
	"wh/infrastructure/log"
	"wh/infrastructure/server"

	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/spf13/viper"
	DEATH "github.com/vrecan/death/v3"
)

var (
	authenticator  auth.Authenticator
	authMiddleware auth.AuthMiddleware
	buckets        publish.Buckets
	config         *viper.Viper
	handleApi      api.ApiHandler
	handleHome     home.HomeHandler
	logger         *zap.Logger
	publisher      publish.Publisher
	store          publish.Store
)

func main() {
	var err error

	config, err = configuration.NewConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error reading config file: %w", err))
	}

	// Set default config for consecutive calls.
	domain.SetDefaultConfig(config)

	logger, err = log.NewLogger(config)
	if err != nil {
		panic(fmt.Errorf("fatal error creating logger: %w", err))
	}

	store, err = publish.NewStore(config)
	if err != nil {
		panic(fmt.Errorf("fatal error creating store: %w", err))
	}

	defer func(log *zap.Logger) {
		_ = log.Sync()
	}(logger)

	buckets = publish.NewFileBucket(config)
	publisher = publish.NewPublisher(store, buckets, logger)
	authenticator = auth.NewAuthenticator(config)
	authMiddleware = auth.NewAuthMiddleware(authenticator, logger)
	handleHome = home.NewHomeHandler(store, buckets, authenticator, logger)
	handleApi = api.NewApiHandler(publisher, config, logger)

	// Create a grpc server, but do not start it yet, because it is handled by the mux.
	grpcServer := initGrpc()

	// Create the echo http handler.
	httpServer := startHttp()

	mixedHandler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.ProtoMajor == 2 && strings.Contains(request.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(writer, request)
			return
		}

		httpServer.ServeHTTP(writer, request)
	})

	httpAddress := config.GetString("http.address")
	http2Server := &http2.Server{}
	http1Server := &http.Server{Handler: h2c.NewHandler(mixedHandler, http2Server), Addr: httpAddress}

	go func() {
		if err := http1Server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("Shutting down the server.",
				zap.Error(err),
			)
		}
	}()

	logger.Info("Started listening to incoming http calls",
		zap.String("address", httpAddress),
	)

	death := DEATH.NewDeath(syscall.SIGINT, syscall.SIGTERM)

	err = death.WaitForDeath()
	if err != nil {
		panic(fmt.Errorf("fatal error waiting for application stop:  %w", err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = http1Server.Shutdown(ctx)
}

func startHttp() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(middleware.Recover())
	e.Use(server.PassThroughContext())
	e.Use(server.Localize())
	e.Use(server.Logger(logger))
	e.Use(server.LiveReload())
	e.Static("/public", "./public")
	e.HTTPErrorHandler = handleHome.ErrorHandler

	e.POST("/", handleHome.PostIndex, authMiddleware.MustNotBeAuthenticated)
	e.GET("/", handleHome.GetIndex, authMiddleware.MustNotBeAuthenticated)
	e.GET("/buckets/:id/request", handleHome.RequestBlob, authMiddleware.MustBeAuthenticated)
	e.GET("/buckets/:id/response", handleHome.ResponseBlob, authMiddleware.MustBeAuthenticated)
	e.GET("/internal", handleHome.GetInternal, authMiddleware.MustBeAuthenticated)
	e.GET("/error", handleHome.GetError)
	e.GET("/events", handleHome.GetEvents, authMiddleware.MustBeAuthenticated)
	e.Any("/endpoints/*", handleApi.Index)

	return e
}

func initGrpc() *grpc.Server {
	serverG := grpc.NewServer()
	service := tunnel.NewTunnelServer(publisher, logger)

	generated.RegisterWebhookServiceServer(serverG, service)

	return serverG
}
