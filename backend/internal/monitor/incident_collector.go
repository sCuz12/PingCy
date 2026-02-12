package monitor

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-telegram/bot"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// IncidentCollector listens to events and records incident lifecycles in the DB.
// An incident starts when a target transitions from UP->DOWN (or TIMEOUT), and
// ends when it returns to UP. Only one open incident per (target, probe) exists.
func IncidentCollector(ctx context.Context, eventsCh <-chan Event, dbpool *pgxpool.Pool, tbot *bot.Bot, chatID int64) {
	go func() {
		for e := range eventsCh {
			if dbpool == nil {
				continue
			}
			if err := persistIncident(ctx, dbpool, e); err != nil {
				log.Printf("incident persist failed for %s: %v", e.TargetName, err)
			}

			if tbot != nil {
				var msg string
				if !e.To {
					msg = formatTelegramDownMessage(e)
				} else {
					msg = formatTelegramUpMessage(e)
				}

				if _, err := tbot.SendMessage(ctx, &bot.SendMessageParams{
					ChatID: chatID,
					Text:   msg,
				}); err != nil {
					log.Printf("telegram send failed for %s: %v", e.TargetName, err)
				}
			}
		}
	}()
}

func formatTelegramDownMessage(ev Event) string {
	statusLine := "Status: "
	switch {
	case ev.StatusCode == 0 && ev.Reason != "":
		statusLine += fmt.Sprintf("TIMEOUT (%s)", ev.Reason)
	case ev.StatusCode == 0:
		statusLine += "TIMEOUT"
	case ev.StatusCode >= 500:
		statusLine += fmt.Sprintf("HTTP %d (server error)", ev.StatusCode)
	default:
		statusLine += fmt.Sprintf("HTTP %d", ev.StatusCode)
	}

	if ev.Reason != "" && ev.StatusCode != 0 {
		statusLine += fmt.Sprintf(" — %s", ev.Reason)
	}

	return fmt.Sprintf("🚨 DOWN: %s\n%s\nProbe: primary\nAt: %s",
		ev.TargetName,
		statusLine,
		ev.At.UTC().Format("2006-01-02 15:04 MST"),
	)
}

func formatTelegramUpMessage(ev Event) string {
	statusLine := "Status: "
	if ev.StatusCode == 0 {
		statusLine += "UP"
	} else {
		statusLine += fmt.Sprintf("HTTP %d", ev.StatusCode)
		if ev.Reason != "" {
			statusLine += fmt.Sprintf(" — %s", ev.Reason)
		}
	}

	return fmt.Sprintf("✅ UP: %s\n%s\nProbe: primary\nAt: %s",
		ev.TargetName,
		statusLine,
		ev.At.UTC().Format("2006-01-02 15:04 MST"),
	)
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
    `, ev.At, ev.StatusCode, ev.Reason, ev.TargetName, "primary")
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
