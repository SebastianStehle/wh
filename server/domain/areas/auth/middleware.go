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

type authMiddleware struct {
	authenticator Authenticator
	cookieName    string
	secure        securecookie.SecureCookie
	logger        *zap.Logger
}

type AuthMiddleware interface {
	MustBeAuthenticated(next echo.HandlerFunc) echo.HandlerFunc

	MustNotBeAuthenticated(next echo.HandlerFunc) echo.HandlerFunc

	SetApiKey(c echo.Context, apiKey string) bool
}

func NewAuthMiddleware(authenticator Authenticator, config *viper.Viper, logger *zap.Logger) AuthMiddleware {
	secure := *securecookie.New(
		createKey("auth.hashKey", config),
		createKey("auth.blockKey", config))

	return &authMiddleware{
		secure:        secure,
		logger:        logger,
		authenticator: authenticator,
		cookieName:    "API_KEY",
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

func (a authMiddleware) MustBeAuthenticated(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		apiKey, err := a.getCookie(c)
		if err != nil {
			log.Warn("Failed to decode cookie.",
				zap.Error(err),
			)
		} else if apiKey == "" {
			log.Debug("Auth cookie not found.")
		} else {
			if a.authenticator.Validate(apiKey) {
				return next(c)
			}

			log.Warn("Auth cookie invalid.")
		}

		return redirectToLogin(c, a.cookieName)
	}
}

func (a authMiddleware) MustNotBeAuthenticated(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		cookie, _ := c.Cookie(a.cookieName)

		if cookie == nil || cookie.Value == "" {
			return next(c)
		}

		return c.Redirect(http.StatusFound, "/internal")
	}
}

func (a authMiddleware) SetApiKey(c echo.Context, apiKey string) bool {
	if !a.authenticator.Validate(apiKey) {
		log.Warn("Invalid API Key entered")
		return false
	}

	if err := a.SetCookie(c, apiKey); err != nil {
		log.Warn("Failed to encode cookie.", zap.Error(err))
		return false
	}

	return true
}

func (a authMiddleware) getCookie(c echo.Context) (string, error) {
	cookie, _ := c.Cookie(a.cookieName)

	if cookie == nil {
		return "", nil
	}

	decoded := make(map[string]string)
	if err := a.secure.Decode(a.cookieName, cookie.Value, &decoded); err != nil {
		return "", err
	}

	apiKey := decoded[a.cookieName]
	return apiKey, nil
}

func (a authMiddleware) SetCookie(c echo.Context, apiKey string) error {
	values := map[string]string{
		a.cookieName: apiKey,
	}

	encoded, err := a.secure.Encode(a.cookieName, values)
	if err != nil {
		return err
	}

	cookie := &http.Cookie{
		Name:    a.cookieName,
		Value:   encoded,
		Path:    "/",
		Expires: time.Now().Add(30 * 24 * time.Hour),
	}

	c.SetCookie(cookie)
	return nil
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
