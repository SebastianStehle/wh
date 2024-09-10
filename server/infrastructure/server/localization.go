package server

import (
	"context"

	"github.com/BurntSushi/toml"
	"github.com/labstack/echo/v4"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var (
	bundle *i18n.Bundle
)

func GetLocalizer(c context.Context) *i18n.Localizer {
	value := c.Value("localizer")
	if value == nil {
		return nil
	}

	return value.(*i18n.Localizer)
}

func Localize() echo.MiddlewareFunc {
	bundle = i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			accept := c.Request().Header.Get("Accept-language")

			localizer := i18n.NewLocalizer(bundle, accept)

			c.Set("localizer", localizer)
			return next(c)
		}
	}
}
