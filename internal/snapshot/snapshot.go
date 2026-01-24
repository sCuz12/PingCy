package snapshot

import "sync/atomic"

// Snapshot is the read-only view used by the API.
type Snapshot struct {
	All    []StateDTO
	ByName map[string]StateDTO
}

// StateDTO is what the API exposes per target.
type StateDTO struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Up          bool   `json:"up"`
	LastChecked string `json:"last_checked"`
	LatencyMs   int64  `json:"latency_ms"`
	StatusCode int    `json:"status_code"`
	LastError  string `json:"last_error"`

	ConsecutiveSuccess int `json:"consecutive_success"`
	ConsecutiveFail    int `json:"consecutive_fail"`
	TotalChecks        int `json:"total_checks"`
	TotalFails         int `json:"total_fails"`
}

var current atomic.Value // stores Snapshot

// Publish replaces the current snapshot.
func Publish(s Snapshot) {
	current.Store(s)
}

// Get returns the latest snapshot.
// If nothing was published yet, returns zero-value snapshot.
func Get() Snapshot {
	if v := current.Load(); v != nil {
		return v.(Snapshot)
	}
	return Snapshot{}
}
