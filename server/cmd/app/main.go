package main

import (
	"fmt"
	"net"
	"net/http"
	"syscall"
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
	"google.golang.org/grpc"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/soheilhy/cmux"
	"github.com/spf13/viper"
	DEATH "github.com/vrecan/death/v3"
)

var (
	authenticator auth.Authenticator
	config        *viper.Viper
	handleApi     api.ApiHandler
	handleHome    home.HomeHandler
	logger        *zap.Logger
	publisher     publish.Publisher
)

func main() {
	var err error

	config, err = configuration.NewConfig()
	if err != nil {
		panic(fmt.Errorf("fatal error reading config file: %w", err))
	}

	logger, err = log.NewLogger(config)
	if err != nil {
		panic(fmt.Errorf("fatal error creating logger: %w", err))
	}

	defer func(log *zap.Logger) {
		_ = log.Sync()
	}(logger)

	domain.SetDefaultConfig(config)

	// The address also includes the port by default.
	address := config.GetString("http.address")

	listener, err := net.Listen("tcp", address)
	if err != nil {
		panic(fmt.Errorf("failed to listen to address %w", err))
	}

	publisher = publish.NewPublisher(config)
	handleHome = home.NewHomeHandler(publisher, authenticator, logger)
	handleApi = api.NewApiHandler(config, publisher, logger)
	authenticator = auth.NewAuthenticator(config, logger)

	m := cmux.New(listener)
	startGrpc(m)
	startHttp(m)

	go m.Serve()

	logger.Info("starting server",
		zap.String("address", address),
	)

	death := DEATH.NewDeath(syscall.SIGINT, syscall.SIGTERM)

	err = death.WaitForDeath()
	if err != nil {
		panic(fmt.Errorf("fatal error creating logger: %w", err))
	}

	m.Close()
}

func startHttp(mux cmux.CMux) {
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

	e.POST("/", handleHome.PostIndex, authenticator.MustNotBeAuthenticated)
	e.GET("/", handleHome.GetIndex, authenticator.MustNotBeAuthenticated)
	e.GET("/internal", handleHome.GetInternal, authenticator.MustBeAuthenticated)
	e.GET("/error", handleHome.GetError)
	e.GET("/events", handleHome.GetEvents, authenticator.MustBeAuthenticated)
	e.Any("/endpoints/*", handleApi.Index)

	server := &http.Server{
		Handler: e,
	}

	match := mux.Match(cmux.Any())
	go server.Serve(match)
}

func startGrpc(mux cmux.CMux) {
	server := grpc.NewServer()
	service := tunnel.NewCliServer(publisher)

	generated.RegisterWebhookServiceServer(server, service)

	match := mux.Match(cmux.HTTP2HeaderField("content-type", "application/grpc"))
	go server.Serve(match)
}
