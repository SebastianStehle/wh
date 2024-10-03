package views

import (
	"fmt"
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

		requestType, ok := publish.GetRequestType(&entry)
		if ok {
			source := fmt.Sprintf("/buckets/%s/request", entry.RequestId)
			entryVm.RequestEditor = getEditorInfo(requestType, source)
		}

		responseType, ok := publish.GetResponseType(&entry)
		if ok {
			source := fmt.Sprintf("/buckets/%s/response", entry.RequestId)
			entryVm.ResponseEditor = getEditorInfo(responseType, source)
		}

		vms = append(vms, entryVm)
	}

	return EventsVM{Entries: vms}
}

func getEditorInfo(contentType string, source string) *EditorInfo {
	parts := strings.Split(contentType, ";")

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
