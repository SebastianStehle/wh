package publish

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"path"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/viper"
)

type StoreEntry struct {
	RequestId    string
	Started      time.Time
	Endpoint     string
	Request      HttpRequestStart
	RequestSize  int
	Response     *HttpResponseStart
	ResponseSize int
	Error        error
	Completed    time.Time
	Status       Status
}

const (
	tableDefinition string = `
		CREATE TABLE IF NOT EXISTS requests (
			requestId		STRING NOT NULL PRIMARY KEY,
			started			DATETIME NOT NULL,
			endpoint		STRING NOT NULL,
			requestMethod	STRING NOT NULL,
			requestPath		STRING NOT NULL,
			requestHeaders	STRING NOT NULL,
			requestSize		INT,
			responseStatus 	INT,
			responseHeaders STRING,
			responseSize 	INT,
			error			STRING,
			completed		DATETIME,
			status			INT NOT NULL,
			etag 			INT NOT NULL
		)`
)

func GetRequestType(r *StoreEntry) (string, bool) {
	hasContent := r != nil && r.RequestSize > 0 && r.Status == StatusCompleted
	if !hasContent {
		return "", false
	}

	header := r.Request.Headers.Get("Content-Type")
	return header, header != ""
}

func GetResponseType(r *StoreEntry) (string, bool) {
	hasContent := r != nil && r.ResponseSize > 0 && r.Status == StatusCompleted && r.Response != nil
	if !hasContent {
		return "", false
	}

	header := r.Response.Headers.Get("Content-Type")
	return header, header != ""
}

type record struct {
	requestId       string
	started         time.Time
	endpoint        string
	requestMethod   string
	requestPath     string
	requestHeaders  string
	requestSize     int
	responseStatus  int32
	responseHeaders *string
	responseSize    int
	error           string
	completed       time.Time
	status          Status
	etag            int64
}

type store struct {
	db *sql.DB
}

type Store interface {
	LogRequest(requestId string, endpoint string, request HttpRequestStart) error

	LogResponse(requestId string, requestSize int, response *HttpResponseStart, responseSize int, error error, status Status) error

	GetEntry(requestId string) (*StoreEntry, error)

	GetEntries(etag int64) ([]StoreEntry, int64, error)
}

func NewStore(config *viper.Viper) (Store, error) {
	file := path.Join(config.GetString("dataFolder"), "data.db")

	db, err := sql.Open("sqlite3", file)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(tableDefinition); err != nil {
		return nil, err
	}

	return &store{db: db}, nil
}

func (l store) LogRequest(requestId string, endpoint string, request HttpRequestStart) error {
	const insert string = `
		INSERT INTO requests(
			requestId,
			started,
			endpoint,
			requestMethod,
			requestPath,
			requestHeaders,
			status,
			etag
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	encoded, err := json.Marshal(request.Headers)
	if err != nil {
		return err
	}

	requestHeaders := string(encoded)

	_, err = l.db.Exec(insert,
		requestId,
		time.Now(),
		endpoint,
		request.Method,
		request.Path,
		requestHeaders,
		StatusRequestStarted,
		createEtag())

	return err
}

func (l store) LogResponse(requestId string, requestSize int, response *HttpResponseStart, responseSize int, error error, status Status) error {
	const update string = `
		UPDATE requests 
		SET
			requestSize = ?,
		    responseStatus = ?,
		    responseHeaders = ?,
		    responseSize = ?,
			error = ?,
			completed = ?,
			status = ?,
			etag = ?
		WHERE requestId = ?
	`

	responseStatus := 0
	responseHeaders := ""

	if response != nil {
		encoded, err := json.Marshal(response.Headers)
		if err != nil {
			return err
		}

		responseStatus = int(response.Status)
		responseHeaders = string(encoded)
	}

	errorText := ""
	if error != nil {
		errorText = error.Error()
	}

	_, err := l.db.Exec(update,
		requestSize,
		responseStatus,
		responseHeaders,
		responseSize,
		errorText,
		time.Now(),
		status,
		createEtag(),
		requestId)

	return err
}

func (l store) GetEntry(requestId string) (*StoreEntry, error) {
	const query string = `
		SELECT 
		    requestId,
			started,
			endpoint,
			requestMethod,
			requestPath,
			requestHeaders,
			requestSize,
			responseStatus,
			responseHeaders,
			responseSize,
			error,
			completed,
			status,
			etag		
		FROM requests WHERE requestId = ?
 	`

	rows, err := l.db.Query(query, requestId)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		r, _, err := mapRecord(rows)
		if err != nil {
			return nil, nil
		}

		return r, nil
	}

	return nil, nil
}

func (l store) GetEntries(etag int64) ([]StoreEntry, int64, error) {
	result := make([]StoreEntry, 0)

	const query string = `
		SELECT 
		    requestId,
			started,
			endpoint,
			requestMethod,
			requestPath,
			requestHeaders,
			requestSize,
			responseStatus,
			responseHeaders,
			responseSize,
			error,
			completed,
			status,
			etag		
		FROM requests WHERE etag > ? ORDER BY started DESC LIMIT 100
 	`

	rows, err := l.db.Query(query, etag)
	if err != nil {
		return result, 0, err
	}

	newEtag := etag
	for rows.Next() {
		r, etag, err := mapRecord(rows)
		if err != nil {
			return result, 0, err
		}

		if etag > newEtag {
			newEtag = etag
		}

		result = append(result, *r)
	}

	return result, newEtag, nil
}

func mapRecord(rows *sql.Rows) (*StoreEntry, int64, error) {
	r := &record{}
	err := rows.Scan(
		&r.requestId,
		&r.started,
		&r.endpoint,
		&r.requestMethod,
		&r.requestPath,
		&r.requestHeaders,
		&r.requestSize,
		&r.responseStatus,
		&r.responseHeaders,
		&r.responseSize,
		&r.error,
		&r.completed,
		&r.status,
		&r.etag)

	if err != nil {
		return nil, 0, err
	}

	requestHeaders := make(http.Header)
	err = json.Unmarshal([]byte(r.requestHeaders), &requestHeaders)
	if err != nil {
		return nil, 0, err
	}

	var response *HttpResponseStart = nil
	if r.responseStatus > 0 && r.responseHeaders != nil {
		responseHeaders := make(http.Header)
		err = json.Unmarshal([]byte(*r.responseHeaders), &responseHeaders)
		if err != nil {
			return nil, 0, err
		}

		response = &HttpResponseStart{Status: r.responseStatus, Headers: responseHeaders}
	}

	entry := StoreEntry{
		RequestId:    r.requestId,
		Started:      r.started,
		Endpoint:     r.endpoint,
		Request:      HttpRequestStart{Method: r.requestMethod, Path: r.requestPath, Headers: requestHeaders},
		RequestSize:  r.requestSize,
		Response:     response,
		ResponseSize: r.responseSize,
		Completed:    r.completed,
		Status:       r.status,
	}

	return &entry, r.etag, nil
}

func createEtag() int64 {
	return time.Now().Unix()
}
