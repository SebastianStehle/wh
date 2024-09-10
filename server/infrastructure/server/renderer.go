package server

import (
	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"
)

func Render(c echo.Context, statusCode int, t templ.Component) error {
	response := c.Response()

	response.Writer.WriteHeader(statusCode)
	response.Header().Set(echo.HeaderContentType, echo.MIMETextHTML)

	return t.Render(c.Request().Context(), response.Writer)
}
