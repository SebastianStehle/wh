package tunnel

import (
	"net/http"
	"strings"
	"wh/cli/api/tunnel"
)

func combineUrl(baseUrl string, paths ...string) string {
	url := strings.TrimSuffix(baseUrl, "/")

	for _, path := range paths {
		url += "/"
		url += strings.TrimPrefix(path, "/")
	}

	return url
}

func transportToHttp(headers map[string]*tunnel.HttpHeaderValues) http.Header {
	result := make(http.Header, len(headers))
	for header, v := range headers {
		result[header] = v.GetValues()
	}

	return result
}

func headersToGrpc(headers http.Header) map[string]*tunnel.HttpHeaderValues {
	result := make(map[string]*tunnel.HttpHeaderValues, len(headers))
	for header, v := range headers {
		result[header] = &tunnel.HttpHeaderValues{Values: v}
	}

	return result
}

func errorToGrpc(err error) *string {
	result := ""
	if err != nil {
		result = err.Error()
	}

	return &result
}
