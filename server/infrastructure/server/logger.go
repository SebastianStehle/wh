package server

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
)

func Logger(log *zap.Logger) echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus: true,
		LogURI:    true,
		LogError:  true,
		LogMethod: true,
		BeforeNextFunc: func(c echo.Context) {
			c.Set("customValueFromContext", 42)
		},
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			fields := []zap.Field{
				zap.Int("response.status", v.Status),
			}

			if v.Method != "" {
				fields = append(fields, zap.String("request.method", v.Method))
			}

			if v.URI != "" {
				fields = append(fields, zap.String("request.path", v.URI))
			}

			if v.RequestID != "" {
				fields = append(fields, zap.String("request.id", v.RequestID))
			}

			if v.Error != nil {
				fields = append(fields, zap.Error(v.Error))
			}

			log.Info("HTTP request", fields...)
			return nil
		},
	})
}
