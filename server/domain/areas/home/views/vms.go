package views

import (
	"fmt"
	"net/http"
	"strings"
	"wh/domain/publish"
)

type ErrorVM struct {
	Type string
}

type IndexVM struct {
	InvalidApiKey bool
}

type InternalVM struct {
}

type EventsVM struct {
	Entries []LogEntryVM
}

type LogEntryVM struct {
	Entry          publish.StoreEntry
	RequestEditor  *EditorInfo
	ResponseEditor *EditorInfo
}

type EditorInfo struct {
	Mode   string
	Source string
}

func BuildEventsVM(entries []publish.StoreEntry) EventsVM {
	vms := make([]LogEntryVM, 0)

	for _, entry := range entries {
		entryVm := LogEntryVM{
			Entry: entry,
		}

		if publish.HasRequestBody(&entry) {
			source := fmt.Sprintf("/buckets/%s/request", entry.RequestId)
			entryVm.RequestEditor = getEditorInfo(entry.Request.Headers, source)
		}

		if publish.HasResponseBody(&entry) {
			source := fmt.Sprintf("/buckets/%s/response", entry.RequestId)
			entryVm.ResponseEditor = getEditorInfo(entry.Response.Headers, source)
		}

		vms = append(vms, entryVm)
	}

	return EventsVM{Entries: vms}
}

func getEditorInfo(header http.Header, source string) *EditorInfo {
	t := header.Get("Content-Type")
	if t == "" {
		return nil
	}

	parts := strings.Split(t, ";")

	mode := ""
	switch parts[0] {
	case "text/json":
		mode = "ace/mode/javascript"
	case "application/json":
		mode = "ace/mode/javascript"
	case "text/html":
		mode = "ace/mode/html"
	case "application/xhtml+xml":
		mode = "ace/mode/html"
	}

	if mode == "" {
		return nil
	}

	return &EditorInfo{Mode: mode, Source: source}
}
