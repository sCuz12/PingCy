package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	dbpool *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Handler {
	return &Handler{dbpool: db}
}

// GetUptime returns uptime stats for a target over a sliding window (default 24h).
func (h *Handler) GetUptime(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	target := strings.TrimSpace(r.URL.Query().Get("target"))
	if target == "" {
		http.Error(w, "missing target", http.StatusBadRequest)
		return
	}

	window := 24 * time.Hour
	if raw := strings.TrimSpace(r.URL.Query().Get("window")); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil || d <= 0 {
			http.Error(w, "invalid window duration", http.StatusBadRequest)
			return
		}
		window = d
	}

	from := time.Now().UTC().Add(-window)

	var total, up int64
	err := h.dbpool.QueryRow(
		r.Context(),
		`SELECT 
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE status = 'UP') AS up
		  FROM check_results
		  WHERE target_name = $1 AND checked_at >= $2`,
		target, from,
	).Scan(&total, &up)
	if err != nil {
		log.Printf("uptime query failed: %v", err)
		http.Error(w, "uptime query failed", http.StatusInternalServerError)
		return
	}

	var pct float64
	if total > 0 {
		pct = (float64(up) / float64(total)) * 100
	}

	resp := map[string]any{
		"target":       target,
		"window":       window.String(),
		"from":         from.Format(time.RFC3339),
		"total_checks": total,
		"total_up":     up,
		"uptime_pct":   pct,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "failed to encode uptime", http.StatusInternalServerError)
		return
	}
}

// GetUptimeAll returns uptime stats for all targets over a sliding window (default 24h).
func (h *Handler) GetUptimeAll(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	window := 24 * time.Hour
	if raw := strings.TrimSpace(r.URL.Query().Get("window")); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil || d <= 0 {
			http.Error(w, "invalid window duration", http.StatusBadRequest)
			return
		}
		window = d
	}
	from := time.Now().UTC().Add(-window)

	rows, err := h.dbpool.Query(
		r.Context(),
		`SELECT target_name,
		        COUNT(*) AS total,
		        COUNT(*) FILTER (WHERE status = 'UP') AS up
		   FROM check_results
		  WHERE checked_at >= $1
		  GROUP BY target_name
		  ORDER BY target_name`,
		from,
	)
	if err != nil {
		log.Printf("uptime all query failed: %v", err)
		http.Error(w, "uptime query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type item struct {
		Target      string  `json:"target"`
		Window      string  `json:"window"`
		From        string  `json:"from"`
		TotalChecks int64   `json:"total_checks"`
		TotalUp     int64   `json:"total_up"`
		UptimePct   float64 `json:"uptime_pct"`
	}

	list := make([]item, 0)
	for rows.Next() {
		var (
			target string
			total  int64
			up     int64
		)
		if err := rows.Scan(&target, &total, &up); err != nil {
			log.Printf("scan uptime all: %v", err)
			http.Error(w, "uptime scan failed", http.StatusInternalServerError)
			return
		}
		pct := 0.0
		if total > 0 {
			pct = (float64(up) / float64(total)) * 100
		}
		list = append(list, item{
			Target:      target,
			Window:      window.String(),
			From:        from.Format(time.RFC3339),
			TotalChecks: total,
			TotalUp:     up,
			UptimePct:   pct,
		})
	}
	if err := rows.Err(); err != nil {
		log.Printf("rows uptime all: %v", err)
		http.Error(w, "uptime rows failed", http.StatusInternalServerError)
		return
	}

	resp := map[string]any{
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"window":       window.String(),
		"from":         from.Format(time.RFC3339),
		"items":        list,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "failed to encode uptime", http.StatusInternalServerError)
		return
	}
}
