package views

import (
	"fmt"
	"net/http"
	"sort"
	"time"
	"wh/infrastructure/utils"
)

func getStatusClass(status int32) string {
	if status < 200 {
		return "badge badge-lg"
	}

	if status >= 200 && status < 300 {
		return "badge badge-lg badge-success text-white"
	}

	if status >= 300 && status < 400 {
		return "badge badge-lg badge-warning"
	}

	return "badge badge-lg badge-error text-white"
}

func getLogId(vm LogEntryVM) string {
	return fmt.Sprintf("log_%s", vm.Entry.RequestId)
}

func getStartTime(vm LogEntryVM) string {
	return vm.Entry.Started.Format(time.RFC822)
}

func getCompleteTime(vm LogEntryVM) string {
	return vm.Entry.Completed.Format(time.RFC822)
}

func getDuration(vm LogEntryVM) string {
	return vm.Entry.Completed.Sub(vm.Entry.Started).String()
}

func getSortedHeaders(headers http.Header) []string {
	keys := make([]string, 0, len(headers))

	for k := range headers {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool {
		return utils.LessLower(keys[i], keys[j])
	})

	return keys
}
