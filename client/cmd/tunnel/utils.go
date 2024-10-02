package tunnel

import "strings"

func combineUrl(baseUrl string, paths ...string) string {
	url := strings.TrimSuffix(baseUrl, "/")

	for _, path := range paths {
		url += "/"
		url += strings.TrimPrefix(path, "/")
	}

	return url
}
