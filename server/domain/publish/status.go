package publish

type Status = int

const (
	StatusRequestStarted Status = iota
	StatusRequestCompleted
	StatusResponseStarted
	StatusFailed
	StatusTimeout
	StatusCompleted
)

func IsTerminated(status Status) bool {
	return status == StatusFailed || status == StatusTimeout || status == StatusCompleted
}
