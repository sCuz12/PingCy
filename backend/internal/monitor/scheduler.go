package monitor

import (
	"context"
	"log"
	"math/rand"
	"time"
)

// jitterFraction controls how much randomness we add to the schedule. 0.2 = ±20%.
const jitterFraction = 0.2

func init() {
	rand.Seed(time.Now().UnixNano())
}

// StartSchedulers starts one goroutine per enabled target.
// Each scheduler ticks on target.Interval and tries to enqueue a CheckJob.
//
// MVP backpressure policy: if jobsCh is full, drop the job (do not block).
func StartSchedulers(
	ctx context.Context,
	targets []Target,
	jobsCh chan<- CheckJob,
) {
	for _, t := range targets {
		tt := t // capture loop var

		if !tt.Enabled {
			continue
		}

		go func(target Target) {
			// Send an immediate first check
			enqueueJob(ctx, jobsCh, target)

			for {
				delay := jitteredInterval(target.Interval)
				timer := time.NewTimer(delay)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
					enqueueJob(ctx, jobsCh, target)
				}
			}
		}(tt)
	}
}

// jitteredInterval returns the base interval plus a random jitter in ±jitterFraction.
// Ensures a minimum delay of 1ms to avoid hot loops when intervals are very small.
func jitteredInterval(interval time.Duration) time.Duration {
	if interval <= 0 {
		return time.Millisecond
	}
	maxJitter := time.Duration(float64(interval) * jitterFraction)
	jitter := time.Duration(rand.Int63n(int64(maxJitter)*2+1)) - maxJitter
	delay := interval + jitter
	if delay < time.Millisecond {
		return time.Millisecond
	}
	return delay
}

func enqueueJob(ctx context.Context, jobsCh chan<- CheckJob, target Target) {
	job := CheckJob{
		Target:      target,
		ScheduledAt: time.Now(),
		Attempt:     1,
	}

	// Non-blocking send: drop if buffer full (MVP)
	select {
	case jobsCh <- job:
	default:
		log.Printf("[WARN] jobsCh full; dropping job for target=%s", target.Name)
	case <-ctx.Done():
		return
	}
}
