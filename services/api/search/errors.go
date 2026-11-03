package search

import "fmt"

type UpstreamError struct {
	StatusCode int
	Body       string
}

func (e *UpstreamError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("orchestrator returned: %d", e.StatusCode)
	}

	return fmt.Sprintf("orchestrator returned %d: %s", e.StatusCode, e.Body)
}
