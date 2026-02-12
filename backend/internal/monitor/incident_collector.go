package monitor

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// IncidentCollector listens to events and records incident lifecycles in the DB.
// An incident starts when a target transitions from UP->DOWN (or TIMEOUT), and
// ends when it returns to UP. Only one open incident per (target, probe) exists.
func IncidentCollector(ctx context.Context, eventsCh <-chan Event, dbpool *pgxpool.Pool) {
	go func() {
		for e := range eventsCh {
			if dbpool == nil {
				continue
			}
			if err := persistIncident(ctx, dbpool, e); err != nil {
				log.Printf("incident persist failed for %s: %v", e.TargetName, err)
			}
		}
	}()
}

// persistIncident upserts incidents table according to transition events.
func persistIncident(ctx context.Context, db *pgxpool.Pool, ev Event) error {
	// When we go DOWN -> open incident; when we go UP -> close existing.
	if ev.To == false { // going down
		// Insert only if there isn't an active (ended_at IS NULL) incident already.
		_, err := db.Exec(ctx, `
            INSERT INTO incidents (
                target_name, probe,
                started_at,
                start_status,
                start_status_code,
                start_error
            )
            SELECT $1, $2, $3, $4, NULLIF($5,0), NULLIF($6,'')
            WHERE NOT EXISTS (
                SELECT 1 FROM incidents WHERE target_name = $1 AND probe = $2 AND ended_at IS NULL
            )
        `, ev.TargetName, "primary", ev.At, statusFromEvent(ev), ev.StatusCode, ev.Reason)
		return err
	}

	// ev.To == true: close any active incident for this target.
	_, err := db.Exec(ctx, `
        UPDATE incidents
           SET ended_at = $1,
               end_status = 'UP',
               end_status_code = NULLIF($2,0),
               end_error = NULLIF($3,''), 
               updated_at = now()
         WHERE target_name = $4
           AND probe = $5
           AND ended_at IS NULL
    `, ev.At,ev.StatusCode, ev.Reason, ev.TargetName, "primary")
	return err
}

// statusFromEvent maps the "To" bool into incident status text.
func statusFromEvent(ev Event) string {
	if ev.To {
		return "UP"
	}
	// DOWN/unknown. Caller can override if needed.
	return "DOWN"
}

// EventForTest is a helper for tests to avoid importing monitor everywhere.
func EventForTest(target string, from, to bool, at time.Time) Event {
	return Event{TargetName: target, From: from, To: to, At: at}
}

// Ensure pgx.ErrNoRows is linked for potential future uses.
var _ = pgx.ErrNoRows
