package monitor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// CheckOnce performs a single HTTP check for a target.
// - Uses ctx for cancellation/timeout (workers should pass a per-job context.WithTimeout).
// - Measures total request latency.
// - Validates expected status and optional keyword match.
func CheckOnce(ctx context.Context, client *http.Client, t Target) CheckResult {
	start := time.Now()

	res := CheckResult{
		TargetName: t.Name,
		URL:        t.URL,
		At:         time.Now(),
		Attempt:    1,
	}

	req, err := http.NewRequestWithContext(ctx, t.Method, t.URL, nil)
	if err != nil {
		res.Up = false
		res.Error = fmt.Sprintf("build request: %v", err)
		res.Latency = time.Since(start)
		return res
	}

	// Optional: if we want to ensure UA even if client wrapper is removed later.
	// if req.Header.Get("User-Agent") == "" {
	// 	req.Header.Set("User-Agent", "CyprusStatusMonitor/0.1")
	// }

	resp, err := client.Do(req)
	if err != nil {
		res.Up = false
		res.Error = classifyHTTPError(err)
		res.Latency = time.Since(start)
		return res
	}
	defer resp.Body.Close()
	
	res.StatusCode = resp.StatusCode
	res.Latency = time.Since(start)

	// 1) Status code validation
	if t.ExpectedStatus != 0 && resp.StatusCode != t.ExpectedStatus {
		res.Up = false
		res.Validation = fmt.Sprintf("unexpected status: got %d want %d", resp.StatusCode, t.ExpectedStatus)
		return res
	}

	// If no expected status provided (or set to 0), consider 200-399 as UP.
	if t.ExpectedStatus == 0 {
		if resp.StatusCode < 200 || resp.StatusCode >= 400 {
			res.Up = false
			res.Validation = fmt.Sprintf("bad status: %d", resp.StatusCode)
			return res
		}
	}

	// 2) Keyword/content validation (GET only)
	contains := strings.TrimSpace(t.Contains)
	if contains != "" {
		if strings.ToUpper(t.Method) == "HEAD" {
			res.Up = false
			res.Validation = "contains check configured but method is HEAD (no body)"
			return res
		}

		maxBytes := t.MaxBodyBytes
		if maxBytes <= 0 {
			maxBytes = 64 * 1024 // 64KB default safety
		}

		limited := io.LimitReader(resp.Body, maxBytes)
		bodyBytes, readErr := io.ReadAll(limited)
		if readErr != nil {
			res.Up = false
			res.Error = fmt.Sprintf("read body: %v", readErr)
			return res
		}

		if !strings.Contains(string(bodyBytes), contains) {
			res.Up = false
			res.Validation = fmt.Sprintf("keyword missing: %q", contains)
			return res
		}
	}

	// Passed all validations
	res.Up = true
	res.Error = ""
	res.Validation = ""
	return res
}

// classifyHTTPError tries to produce a stable, human-readable reason.
// Keep it simple for MVP (don’t over-engineer).
func classifyHTTPError(err error) string {
	// ctx deadline exceeded shows up commonly
	if errorsIsContextDeadline(err) {
		return "timeout"
	}
	if errorsIsContextCanceled(err) {
		return "canceled"
	}
	// The raw error string is useful for debugging; you can refine later.
	return err.Error()
}

// Small helpers to avoid importing errors/context in multiple places later.
// (Feel free to inline these if you prefer.)
func errorsIsContextDeadline(err error) bool {
	return err == context.DeadlineExceeded || strings.Contains(err.Error(), "context deadline exceeded")
}

func errorsIsContextCanceled(err error) bool {
	return err == context.Canceled || strings.Contains(err.Error(), "context canceled")
}
