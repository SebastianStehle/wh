package auth

import (
	"net/http"
	"time"

	"github.com/gorilla/securecookie"
	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
)

type authenticator struct {
	apiKey     string
	cookieName string
	secure     securecookie.SecureCookie
}

type Authenticator interface {
	Validate(apiKey string) bool

	GetApiKey(c echo.Context) (string, error)

	SetApiKey(c echo.Context, apiKey string) error
}

func NewAuthenticator(config *viper.Viper) Authenticator {
	secure := *securecookie.New(
		createKey("auth.hashKey", config),
		createKey("auth.blockKey", config))

	return &authenticator{
		apiKey:     config.GetString("auth.apiKey"),
		secure:     secure,
		cookieName: "API_KEY",
	}
}

func (a authenticator) Validate(apiKey string) bool {
	return a.apiKey == apiKey
}

func (a authenticator) GetApiKey(c echo.Context) (string, error) {
	cookie, err := c.Cookie(a.cookieName)

	if err != nil || cookie == nil {
		return "", err
	}

	decoded := make(map[string]string)
	if err := a.secure.Decode(a.cookieName, cookie.Value, &decoded); err != nil {
		return "", err
	}

	apiKey := decoded[a.cookieName]
	return apiKey, nil
}

func (a authenticator) SetApiKey(c echo.Context, apiKey string) error {
	if apiKey == "" {
		cookie := &http.Cookie{
			Name:    a.cookieName,
			Value:   "removed",
			Path:    "/",
			Expires: time.Unix(0, 0),
		}

		c.SetCookie(cookie)
		return nil
	}

	return setCookie(a, c, apiKey)
}

func setCookie(a authenticator, c echo.Context, apiKey string) error {
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
