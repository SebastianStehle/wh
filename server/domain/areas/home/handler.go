package home

import (
	"io"
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

	RequestBlob(c echo.Context) error

	ResponseBlob(c echo.Context) error

	ErrorHandler(err error, c echo.Context)
}

type homeHandler struct {
	authenticator auth.Authenticator
	buckets       publish.Buckets
	logger        *zap.Logger
	store         publish.Store
}

func NewHomeHandler(store publish.Store, buckets publish.Buckets, authenticator auth.Authenticator, logger *zap.Logger) HomeHandler {
	return &homeHandler{
		authenticator: authenticator,
		buckets:       buckets,
		logger:        logger,
		store:         store,
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

	if h.authenticator.Validate(apiKey) {
		if err := h.authenticator.SetApiKey(c, apiKey); err == nil {
			return c.Redirect(http.StatusFound, "/internal")
		}
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

// GET /buckets/:id/request
func (h homeHandler) RequestBlob(c echo.Context) error {
	id := c.Param("id")
	record, err := h.store.GetEntry(id)
	if err != nil {
		return err
	}

	mimeType, ok := publish.GetRequestType(record)
	if !ok {
		return c.NoContent(http.StatusNotFound)
	}

	reader, err := h.buckets.OpenRequestReader(id)
	if err != nil {
		return err
	}

	c.Response().Header().Add("Content-Type", mimeType)

	_, err = io.Copy(c.Response().Writer, reader)
	return err
}

// GET /buckets/:id/response
func (h homeHandler) ResponseBlob(c echo.Context) error {
	id := c.Param("id")
	record, err := h.store.GetEntry(id)
	if err != nil {
		return err
	}

	mimeType, ok := publish.GetResponseType(record)
	if !ok {
		return c.NoContent(http.StatusNotFound)
	}

	reader, err := h.buckets.OpenResponseReader(id)
	if err != nil {
		return err
	}

	c.Response().Header().Add("Content-Type", mimeType)

	_, err = io.Copy(c.Response().Writer, reader)
	return err
}

// GET /events
func (h homeHandler) GetEvents(c echo.Context) error {
	changeQuery := c.QueryParam("changeSet")
	changeSet, _ := strconv.Atoi(changeQuery)

	events, tag, err := h.store.GetEntries(int64(changeSet))
	if err != nil {
		return err
	}

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
