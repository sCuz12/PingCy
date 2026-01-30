package monitor

import (
	"context"
	"log"
	"time"
)

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

			ticker := time.NewTicker(target.Interval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					enqueueJob(ctx, jobsCh, target)
				}
			}
		}(tt)
	}
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
