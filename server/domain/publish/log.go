package publish

import (
	"net/http"
	"slices"
	"time"
)

type LogEntry struct {
	Completed    time.Time
	Endpoint     string
	Error        error
	Request      HttpRequestStart
	RequestSize  int
	RequestId    string
	Response     *HttpResponseStart
	ResponseSize int
	Size         int
	Started      time.Time
	Timeout      bool
	Timestamp    int64
}

type log struct {
	entries    []LogEntry
	maxSize    int
	maxEntries int
}

type Log interface {
	LogRequest(requestId string, endpoint string, request HttpRequestStart)

	LogResponse(requestId string, response HttpResponseStart, requestSize int, responseSize int)

	LogTimeout(requestId string)

	LogError(requestId string, err error)

	GetEntries(etag int64) ([]LogEntry, int64)
}

func NewLog(maxSize int, maxEntries int) Log {
	return &log{
		entries:    make([]LogEntry, 0),
		maxEntries: maxEntries,
		maxSize:    maxSize,
	}
}

func (l log) LogRequest(requestId string, endpoint string, request HttpRequestStart) {
	if l.maxEntries <= 0 || l.maxSize <= 0 {
		return
	}

	entry := LogEntry{
		Endpoint:  endpoint,
		RequestId: requestId,
		Request:   request,
		Started:   time.Now(),
		Timeout:   false,
		Timestamp: timestamp(),
	}

	entry.estimateSize()

	l.entries = append(l.entries, entry)
	l.ensureSize()
}

func (l log) LogRequestBody(requestId string, size int) {
	entry := l.findEntry(requestId)
	if entry == nil {
		return
	}

	entry.RequestSize = size
}

func (l log) LogTimeout(requestId string) {
	entry := l.findEntry(requestId)
	if entry == nil || entry.Response != nil || entry.Timeout || entry.Error != nil {
		return
	}

	entry.Completed = time.Now()
	entry.Timeout = true
	entry.Timestamp = timestamp()
}

func (l log) LogResponse(requestId string, response HttpResponseStart, requestSize int, responseSize int) {
	entry := l.findEntry(requestId)
	if entry == nil || entry.Response != nil || entry.Timeout || entry.Error != nil {
		return
	}

	entry.Completed = time.Now()
	entry.RequestSize = requestSize
	entry.Response = &response
	entry.ResponseSize = responseSize
	entry.Timeout = true
	entry.Timestamp = timestamp()
	entry.estimateSize()

	l.ensureSize()
}

func (l log) LogResponseBody(requestId string, size int) {
	entry := l.findEntry(requestId)
	if entry == nil {
		return
	}

	entry.ResponseSize = size
}

func (l log) LogError(requestId string, err error) {
	entry := l.findEntry(requestId)
	if entry == nil || entry.Response != nil || entry.Timeout || entry.Error != nil {
		return
	}

	entry.Completed = time.Now()
	entry.Error = err
	entry.Timeout = true
	entry.Timestamp = timestamp()
	entry.estimateSize()

	l.ensureSize()
}

func (l log) GetEntries(etag int64) ([]LogEntry, int64) {
	result := make([]LogEntry, 0)

	t := etag
	for _, e := range l.entries {
		if e.Timestamp > etag {
			result = append(result, e)
			if e.Timestamp > t {
				t = e.Timestamp
			}
		}
	}

	return result, t
}

func (l log) findEntry(requestId string) *LogEntry {
	index := slices.IndexFunc(l.entries, func(e LogEntry) bool {
		return e.RequestId == requestId
	})

	if index < 0 {
		return nil
	}

	return &l.entries[index]
}

func (l log) ensureSize() {
	size := 0
	for _, e := range l.entries {
		size += e.Size
	}

	for size > l.maxSize && len(l.entries) > l.maxEntries {
		size -= l.entries[0].Size

		l.entries = l.entries[1:]
	}
}

func (e *LogEntry) estimateSize() {
	e.Size = e.Request.estimateRequestSize()

	if e.Response != nil {
		e.Size += e.Response.estimateResponseSize()
	}
}

func (r HttpRequestStart) estimateRequestSize() int {
	size := 0
	size += len(r.Path)
	size += len(r.Method)
	size += estimateHeaderSize(r.Headers)
	return size
}

func (r HttpResponseStart) estimateResponseSize() int {
	size := 0
	size += estimateHeaderSize(r.Headers)
	return size
}

func estimateHeaderSize(headers http.Header) int {
	size := 0
	for k, v := range headers {
		size += len(k)
		for _, h := range v {
			size += len(h)
		}
	}

	return size
}

func timestamp() int64 {
	return time.Now().Unix()
}
