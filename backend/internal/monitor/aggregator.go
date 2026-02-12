package monitor

import (
	"context"
	"cy-platforms-status-monitor/internal/snapshot"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Aggregator(ctx context.Context, resCh <-chan CheckResult, eventsCh chan<- Event, db *pgxpool.Pool) {
	state := make(map[string]*State)

	for {
		select {
		case <-ctx.Done():
			return
		case res, ok := <-resCh:
			//graceful exit
			if !ok {
				return
			}

			if db != nil {
				_ = persistCheckResult(ctx, db, res) // best-effort; ignore error for now
			}

			st := state[res.TargetName]
			if st == nil {
				// Try to hydrate from DB so we keep streaks across restarts.
				loaded, err := loadStateFromDB(ctx, db, res.TargetName)
				if err != nil && !errorsIsContextDeadline(err) {
					log.Printf("aggregator: fallback to empty state for %s: %v", res.TargetName, err)
				}
				if loaded == nil {
					loaded = &State{Name: res.TargetName, URL: res.URL}
				}

				state[res.TargetName] = loaded
				st = loaded
			}
			
			prevUp := st.LastUp
			
			updateState(st, res)
			
			if st.TotalChecks > 1 && prevUp != res.Up {
				event := Event{
					TargetName: res.TargetName,
					URL:        res.URL,
					From:       prevUp,
					To:         res.Up,
					At:         res.At,
					Reason: res.Error,
					StatusCode: res.StatusCode,
				}
				//push to events
				select {
				case eventsCh <- event:
				case <-ctx.Done():

				}
			}
			//build snapshot
			snapshot.Publish(buildSnapshot(state))
		}
	}
}

func buildSnapshot(states map[string]*State) snapshot.Snapshot {
	all := make([]snapshot.StateDTO, 0, len(states))
	byName := make(map[string]snapshot.StateDTO, len(states))

	for _, st := range states {
		dto := snapshot.StateDTO{
			Name:        st.Name,
			URL:         st.URL,
			Up:          st.LastUp,
			LastChecked: st.LastChecked.UTC().Format(time.RFC3339),
			LatencyMs:   st.LastLatency.Milliseconds(),
			StatusCode:  st.LastStatusCode,
			LastError:   st.LastError,

			ConsecutiveSuccess: st.ConsecutiveSuccess,
			ConsecutiveFail:    st.ConsecutiveFail,
			TotalChecks:        st.TotalChecks,
			TotalFails:         st.TotalFails,
		}

		all = append(all, dto)
		byName[dto.Name] = dto
	}

	return snapshot.Snapshot{
		All:    all,
		ByName: byName,
	}
}

func updateState(state *State, res CheckResult) {
	state.LastChecked = time.Now()
	state.LastUp = res.Up
	state.Name = res.TargetName
	state.LastLatency = res.Latency
	state.LastStatusCode = res.StatusCode
	state.URL = res.URL
	state.TotalChecks++

	if res.Up {
		state.ConsecutiveSuccess++
		state.ConsecutiveFail = 0
	} else {
		state.TotalFails++
		state.ConsecutiveFail++
		state.LastError = res.Error
	}

}

// loadStateFromDB tries to reconstruct the last known state for a target from check_results.
func loadStateFromDB(ctx context.Context, db *pgxpool.Pool, target string) (*State, error) {
	if db == nil {
		return nil, errors.New("db pool nil")
	}

	var (
		checkedAt  time.Time
		status     string
		statusCode int
		latencyMs  int64
		errText    *string
	)

	err := db.QueryRow(ctx, `
		SELECT checked_at, status, COALESCE(status_code, 0), COALESCE(latency_ms, 0), error
		  FROM check_results
		 WHERE target_name = $1
		 ORDER BY checked_at DESC
		 LIMIT 1`,
		target,
	).Scan(&checkedAt, &status, &statusCode, &latencyMs, &errText)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	st := &State{
		Name:           target,
		LastChecked:    checkedAt,
		LastUp:         strings.EqualFold(status, "UP"),
		LastLatency:    time.Duration(latencyMs) * time.Millisecond,
		LastStatusCode: statusCode,
		LastError:      "",
	}
	if errText != nil {
		st.LastError = *errText
	}

	// Totals
	var total, fails int64
	if err := db.QueryRow(ctx,
		`SELECT COUNT(*), COUNT(*) FILTER (WHERE status <> 'UP')
		   FROM check_results
		  WHERE target_name = $1`,
		target,
	).Scan(&total, &fails); err == nil {
		st.TotalChecks = int(total)
		st.TotalFails = int(fails)
	}

	// Streak: fetch recent rows until a different status appears.
	rows, err := db.Query(ctx,
		`SELECT status
		   FROM check_results
		  WHERE target_name = $1
		  ORDER BY checked_at DESC
		  LIMIT 100`,
		target,
	)
	if err == nil {
		defer rows.Close()
		streak := 0
		var firstStatus string
		for rows.Next() {
			var s string
			if scanErr := rows.Scan(&s); scanErr != nil {
				break
			}
			if streak == 0 {
				firstStatus = s
			}
			if !strings.EqualFold(s, firstStatus) {
				break
			}
			streak++
		}
		if st.LastUp {
			st.ConsecutiveSuccess = streak
			st.ConsecutiveFail = 0
		} else {
			st.ConsecutiveFail = streak
			st.ConsecutiveSuccess = 0
		}
	}

	return st, nil
}

func persistCheckResult(ctx context.Context, db *pgxpool.Pool, res CheckResult) error {
	status := "DOWN"
	switch {
	case res.Up:
		status = "UP"
	case errorsIsContextDeadline(errors.New(res.Error)) || strings.Contains(strings.ToLower(res.Error), "timeout"):
		status = "TIMEOUT"
	}

	checkedAt := res.At
	if checkedAt.IsZero() {
		checkedAt = time.Now()
	}

	_, err := db.Exec(ctx, `
		INSERT INTO check_results
			(target_name, checked_at, status, status_code, latency_ms, error, probe)
		VALUES
			($1, $2, $3, $4, $5, $6, $7)
	`, res.TargetName, checkedAt, status, res.StatusCode, res.Latency.Milliseconds(), nullableString(res.Error, res.Validation), "primary")

	return err
}

func nullableString(parts ...string) any {
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			return p
		}
	}
	return nil
}
