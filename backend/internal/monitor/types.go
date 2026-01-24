package monitor

import "time"

// Target describes what to check and how.
type Target struct {
	Name     string
	URL      string
	Method   string        // "GET" or "HEAD"
	Interval time.Duration // how often to schedule checks
	Timeout  time.Duration // per-request timeout

	ExpectedStatus int    // default 200
	Contains       string // optional keyword check (GET only)
	MaxBodyBytes   int64  // limit response read when doing Contains

	Enabled bool
	Tags    []string
}

// CheckJob is a single scheduled check request.
type CheckJob struct {
	Target      Target
	ScheduledAt time.Time
	Attempt     int // for retries later (MVP uses 1)
}

// CheckResult is the outcome of executing a CheckJob.
type CheckResult struct {
	TargetName string
	URL        string

	At      time.Time
	Latency time.Duration

	Up         bool
	StatusCode int // 0 if no response
	Error      string
	Validation string // e.g. "keyword missing", "unexpected status"

	Attempt int
}

// State is the latest (rolling) view per target.
type State struct {
	Name string
	URL  string

	LastUp         bool
	LastChecked    time.Time
	LastLatency    time.Duration
	LastStatusCode int
	LastError      string

	ConsecutiveSuccess int
	ConsecutiveFail    int

	TotalChecks int
	TotalFails  int

	// Optional: keep last N results for history (MVP can skip filling this)
	History []CheckResult
}

// Event is emitted on transitions (UP->DOWN or DOWN->UP).
type Event struct {
	TargetName string
	URL        string

	From bool
	To   bool

	At     time.Time
	Reason string // error/validation/status explanation
}

