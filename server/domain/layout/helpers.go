package layout

import "os"

var (
	isTest int
)

func IsTestEnvironment() bool {
	if isTest == 0 {
		isWatch := os.Getenv("WATCH_MODE") == "ENABLED"

		if isWatch {
			isTest = 1
		} else {
			isTest = 2
		}
	}

	return isTest == 1
}
