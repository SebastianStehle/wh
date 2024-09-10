package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func RunServer(e *echo.Echo, log *zap.Logger, config *viper.Viper) {
	port := config.GetInt("http.port")
	tlsAuto := config.GetBool("http.tlsAuto")
	tlsEnabled := config.GetBool("http.tls")
	tlsCertFile := config.GetString("http.tlsCertFile")
	tlsKeyFile := config.GetString("http.tlsKeyFile")

	go func() {
		address := fmt.Sprintf(":%d", port)

		var start error
		if tlsAuto {
			log.Info("Server starting using auto TLS.",
				zap.String("address", address),
			)

			start = e.StartAutoTLS(address)
		} else if tlsEnabled && tlsCertFile != "" && tlsKeyFile != "" {
			log.Info("Server starting using TLS.",
				zap.String("address", address),
			)

			start = e.StartTLS(address, tlsCertFile, tlsKeyFile)
		} else {
			log.Info("Server starting using HTTP.",
				zap.String("address", address),
			)

			start = e.Start(address)
		}

		if start != http.ErrServerClosed {
			log.Fatal("Shutting down the server.",
				zap.Error(start),
			)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with a timeout of 10 seconds.
	// Use a buffered channel to avoid missing signals as recommended for signal.Notify
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}
}
