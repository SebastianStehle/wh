package server

import (
	"context"

	"github.com/labstack/echo/v4"
)

type contextValue struct {
	echo.Context
}

func (ctx contextValue) Get(key string) interface{} {
	val := ctx.Context.Get(key)
	if val != nil {
		return val
	}

	return ctx.Request().Context().Value(key)
}

func (ctx contextValue) Set(key string, val interface{}) {
	ctx.SetRequest(ctx.Request().WithContext(context.WithValue(ctx.Request().Context(), key, val)))
}

func PassThroughContext() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			return next(contextValue{c})
		}
	}
}
