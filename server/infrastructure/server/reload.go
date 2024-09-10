package server

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/radovskyb/watcher"
)

func LiveReload() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			request := c.Request()
			response := c.Response()

			if request.Method == http.MethodGet && request.URL.Path == "/live-reload" {
				w := watcher.New()
				// We stop the call after one event anyway.
				w.SetMaxEvents(1)
				w.FilterOps(watcher.Create, watcher.Move, watcher.Rename, watcher.Write)

				// Listen to the following files.
				// - Public styles
				// - Public scripts
				// - Templ text files
				r := regexp.MustCompile(`(?m)(public(\/|\\)css)|(public(\/|\\)js)|(views(\/|\\)(.*).txt)`)
				w.AddFilterHook(watcher.RegexFilterHook(r, true))

				if err := w.AddRecursive("./domain"); err != nil {
					return fmt.Errorf("failed to add domain path: %v", err)
				}

				if err := w.AddRecursive("./public"); err != nil {
					return fmt.Errorf("failed to add public path: %v", err)
				}

				go func() {
					if err := w.Start(100 * time.Millisecond); err != nil {
						return
					}
				}()

				response.Header().Set(echo.HeaderContentType, "text/event-stream")
				response.WriteHeader(http.StatusOK)

				for {
					io.WriteString(response, "event: \n")
					io.WriteString(response, "data: ping\n\n")
					response.Flush()

					select {
					case event := <-w.Event:
						io.WriteString(response, "event: \n")
						io.WriteString(response, fmt.Sprintf("data: file changed - %s\n\n", event.Name()))
						return nil
					case <-time.After(500 * time.Millisecond):
					}
				}
			}

			return next(c)
		}
	}
}
