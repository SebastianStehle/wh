package home

import (
	"net/http"
	"strconv"
	"strings"
	"wh/domain/areas/auth"
	"wh/domain/areas/home/views"
	"wh/domain/publish"
	"wh/infrastructure/server"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type HomeHandler interface {
	GetIndex(c echo.Context) error

	GetInternal(c echo.Context) error

	PostIndex(c echo.Context) error

	GetError(c echo.Context) error

	GetEvents(c echo.Context) error

	ErrorHandler(err error, c echo.Context)
}

type homeHandler struct {
	publisher     publish.Publisher
	authenticator auth.Authenticator
	logger        *zap.Logger
}

func NewHomeHandler(publisher publish.Publisher, authenticator auth.Authenticator, logger *zap.Logger) HomeHandler {
	return &homeHandler{
		publisher:     publisher,
		authenticator: authenticator,
		logger:        logger,
	}
}

// GET /
func (h homeHandler) GetIndex(c echo.Context) error {
	vm := views.IndexVM{}

	return server.Render(c, http.StatusOK, views.IndexView(vm))
}

// POST /
func (h homeHandler) PostIndex(c echo.Context) error {
	apiKey := c.FormValue("apiKey")

	if h.authenticator.SetApiKey(c, apiKey) {
		return c.Redirect(http.StatusFound, "/internal")
	}

	vm := views.IndexVM{
		InvalidApiKey: true,
	}

	return server.Render(c, http.StatusOK, views.IndexView(vm))
}

// GET /internal
func (h homeHandler) GetInternal(c echo.Context) error {
	vm := views.InternalVM{}

	return server.Render(c, http.StatusOK, views.InternalView(vm))
}

// GET /error
func (h homeHandler) GetError(c echo.Context) error {
	vm := views.ErrorVM{
		Type: c.QueryParam("type"),
	}

	return server.Render(c, http.StatusOK, views.ErrorView(vm))
}

// GET /events
func (h homeHandler) GetEvents(c echo.Context) error {
	changeQuery := c.QueryParam("changeSet")
	changeSet, _ := strconv.Atoi(changeQuery)

	events, tag := h.publisher.GetEntries(int64(changeSet))
	vm := views.BuildEventsVM(events)

	c.Response().Header().Add("X-ChangeSet", strconv.FormatInt(tag, 10))

	return server.Render(c, http.StatusOK, views.EventsView(vm))
}

var (
	defaultErrorHandler = echo.New().DefaultHTTPErrorHandler
)

func (h homeHandler) ErrorHandler(err error, c echo.Context) {
	h.logger.Error("Error in request", zap.Error(err))

	if strings.Contains(c.Request().URL.Path, "/endpoints") {
		defaultErrorHandler(err, c)
		return
	}

	code := http.StatusInternalServerError
	if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
	}

	errorType := "General"
	if code == http.StatusNotFound {
		errorType = "NotFound"
	}

	vm := views.ErrorVM{
		Type: errorType,
	}

	server.Render(c, code, views.ErrorView(vm))
}
