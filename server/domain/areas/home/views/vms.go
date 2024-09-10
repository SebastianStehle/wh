package views

import (
	"net/http"
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
	Entry          publish.LogEntry
	RequestEditor  *EditorInfo
	ResponseEditor *EditorInfo
}

type EditorInfo struct {
	Mode string
}

func BuildEventsVM(entries []publish.LogEntry) EventsVM {
	vms := make([]LogEntryVM, 0)

	for _, entry := range entries {
		entryVm := LogEntryVM{
			Entry:         entry,
			RequestEditor: getEditorInfo(entry.Request.Headers),
		}

		if entry.Response != nil {
			entryVm.ResponseEditor = getEditorInfo(entry.Response.Headers)
		}

		vms = append(vms, entryVm)
	}

	return EventsVM{Entries: vms}
}

func getEditorInfo(headers http.Header) *EditorInfo {
	contentType := headers["Content-Type"]
	if len(contentType) == 0 {
		return nil
	}

	var (
		mode string
	)

	if contentType[0] == "text/json" || contentType[0] == "application/json" {
		mode = "ace/mode/javascript"
	}

	if mode != "" {
		return &EditorInfo{Mode: mode}
	}

	return nil
}
