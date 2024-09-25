package auth

import (
	"net/http"

	"github.com/gorilla/securecookie"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type authMiddleware struct {
	authenticator Authenticator
	logger        *zap.Logger
}

type AuthMiddleware interface {
	MustBeAuthenticated(next echo.HandlerFunc) echo.HandlerFunc

	MustNotBeAuthenticated(next echo.HandlerFunc) echo.HandlerFunc
}

func NewAuthMiddleware(authenticator Authenticator, logger *zap.Logger) AuthMiddleware {
	return &authMiddleware{logger: logger, authenticator: authenticator}
}

func createKey(name string, config *viper.Viper) []byte {
	fromConfig := config.GetString(name)

	if len(fromConfig) < 32 {
		return securecookie.GenerateRandomKey(32)
	} else {
		return []byte(fromConfig)
	}
}

func (a authMiddleware) MustBeAuthenticated(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		apiKey, err := a.authenticator.GetApiKey(c)

		if err != nil {
			log.Warn("Failed to decode cookie.",
				zap.Error(err),
			)

			return redirectToLogin(a.authenticator, c)
		}

		if apiKey == "" {
			log.Debug("Auth cookie not found.")
			return redirectToLogin(a.authenticator, c)
		}

		if !a.authenticator.Validate(apiKey) {
			log.Warn("Auth cookie invalid.")
			return redirectToLogin(a.authenticator, c)
		}

		return next(c)
	}
}

func (a authMiddleware) MustNotBeAuthenticated(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		apiKey, err := a.authenticator.GetApiKey(c)

		if err != nil {
			log.Info("FOO")
			log.Warn("Failed to decode cookie.",
				zap.Error(err),
			)

			return next(c)
		}

		if apiKey == "" {
			return next(c)
		}

		return c.Redirect(http.StatusFound, "/internal")
	}
}

func redirectToLogin(a Authenticator, c echo.Context) error {
	a.SetApiKey(c, "")
	return c.Redirect(http.StatusFound, "/")
}
