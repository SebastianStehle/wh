package auth

import (
	"net/http"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type authenticator struct {
	apiKey     string
	cookieName string
	secure     securecookie.SecureCookie
	logger     *zap.Logger
}

type Authenticator interface {
	MustBeAuthenticated(next echo.HandlerFunc) echo.HandlerFunc

	MustNotBeAuthenticated(next echo.HandlerFunc) echo.HandlerFunc

	SetApiKey(c echo.Context, apiKey string) bool
}

func NewAuthenticator(config *viper.Viper, logger *zap.Logger) Authenticator {
	apiKey := config.GetString("auth.apiKey")

	secure := *securecookie.New(
		createKey("auth.hashKey", config),
		createKey("auth.blockKey", config))

	return &authenticator{
		secure:     secure,
		logger:     logger,
		apiKey:     apiKey,
		cookieName: "API_KEY",
	}
}

func createKey(name string, config *viper.Viper) []byte {
	fromConfig := config.GetString(name)

	if len(fromConfig) < 32 {
		return securecookie.GenerateRandomKey(32)
	} else {
		return []byte(fromConfig)
	}
}

func (a authenticator) MustBeAuthenticated(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		cookie, _ := c.Cookie(a.cookieName)

		if cookie == nil {
			log.Debug("Auth cookie not found.")
			return redirectToLogin(c, a.cookieName)
		}

		decoded := make(map[string]string)
		if err := a.secure.Decode(a.cookieName, cookie.Value, &decoded); err != nil {
			log.Warn("Failed to decode cookie.", zap.Error(err))
			return redirectToLogin(c, a.cookieName)
		}

		apiKey := decoded[a.cookieName]

		if apiKey != a.apiKey {
			log.Warn("Auth cookie invalid.")
			return redirectToLogin(c, a.cookieName)
		}

		return next(c)
	}
}

func (a authenticator) MustNotBeAuthenticated(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		cookie, _ := c.Cookie(a.cookieName)

		if cookie != nil && cookie.Value != "" {
			return c.Redirect(http.StatusFound, "/internal")
		}

		return next(c)
	}
}

func (a authenticator) SetApiKey(c echo.Context, apiKey string) bool {
	if apiKey != a.apiKey {
		log.Warn("Invalid API Key entered")
		return false
	}

	values := map[string]string{
		a.cookieName: apiKey,
	}

	encoded, err := a.secure.Encode(a.cookieName, values)
	if err != nil {
		log.Warn("Failed to encode cookie.", zap.Error(err))
		return false
	}

	cookie := &http.Cookie{
		Name:    a.cookieName,
		Value:   encoded,
		Path:    "/",
		Expires: time.Now().Add(30 * 24 * time.Hour),
	}

	c.SetCookie(cookie)
	return true
}

func redirectToLogin(c echo.Context, name string) error {
	cookie := &http.Cookie{
		Name:    name,
		Value:   "removed",
		Path:    "/",
		Expires: time.Unix(0, 0),
	}

	c.SetCookie(cookie)
	return c.Redirect(http.StatusFound, "/")
}
