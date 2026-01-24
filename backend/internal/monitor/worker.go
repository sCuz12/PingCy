package monitor

import (
	"context"
	"net/http"
	"sync"
)

// StartWorkers starts a fixed worker pool that consumes jobs from jobsCh,
// executes checks, and publishes results into resultsCh.
//
// - client should be a single reusable *http.Client shared by all workers.
// - resultsCh should be buffered to reduce stalling under load.
// - caller controls shutdown via ctx cancellation.
// - wg is optional but recommended so main() can wait for clean exit.
func StartWorkers(
	ctx context.Context,
	workerCount int,
	client *http.Client,
	jobsCh <-chan CheckJob,
	resultsCh chan<- CheckResult,
	wg *sync.WaitGroup,
) {
	if workerCount <= 0 {
		workerCount = 1
	}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return

				case job, ok := <-jobsCh:
					if !ok {
						return
					}

					// Per-job timeout context
					jobCtx, cancel := context.WithTimeout(ctx, job.Target.Timeout)
					result := CheckOnce(jobCtx, client, job.Target)
					cancel()

					// Fill fields that belong to the job, not the raw check
					result.Attempt = job.Attempt
					// Note: TargetName/URL are already set by CheckOnce, but safe either way:
					result.TargetName = job.Target.Name
					result.URL = job.Target.URL

					// Publish result (stop if shutting down)
					select {
					case resultsCh <- result:
					case <-ctx.Done():
						return
					}

					_ = workerID // keep for future logs (optional)
				}
			}
		}(i + 1)
	}
}
