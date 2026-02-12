package main

import (
	"context"
	"cy-platforms-status-monitor/internal/config"
	"cy-platforms-status-monitor/internal/handlers"
	"cy-platforms-status-monitor/internal/monitor"
	"cy-platforms-status-monitor/internal/snapshot"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

const CONFIGS_PATH = "./configs/config.yaml"

func main() {

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	godotenv.Load(".env")

	// Database: require DATABASE_URL and establish a pooled connection.
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL env var is required")
	}

	poolCfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatalf("failed to parse db config: %v", err)
	}
	// Supabase/PgBouncer (transaction pooling) rejects prepared statements.
	poolCfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	dbpool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		log.Fatalf("failed to create db pool: %v", err)
	}
	defer dbpool.Close()

	if err := dbpool.Ping(context.Background()); err != nil {
		log.Fatalf("database ping failed: %v", err)
	}

	cfg, err := config.Load(CONFIGS_PATH)
	if err != nil {
		log.Fatal(err)
	}

	// Root context for the whole app
	ctx := context.Background()

	client := monitor.NewHTTPClient(monitor.HTTPClientConfig{
		Timeout:         10 * time.Second,
		UserAgent:       cfg.Monitoring.UserAgent,
		MaxIdleConns:    100,
		IdleConnTimeout: 90 * time.Second,
	})

	var workerWg sync.WaitGroup

	jobsCh := make(chan monitor.CheckJob, 200)
	resultsCh := make(chan monitor.CheckResult, 200)
	eventsCh := make(chan monitor.Event, 50)

	targetsToMonitor := toMonitorTargets(cfg.Targets)

	monitor.StartWorkers(ctx, cfg.Monitoring.Workers, client, jobsCh, resultsCh, &workerWg)
	monitor.StartSchedulers(ctx, targetsToMonitor, jobsCh)

	go monitor.Aggregator(ctx, resultsCh, eventsCh, dbpool)

	go monitor.IncidentCollector(ctx, eventsCh,dbpool)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	})
	r.Get("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ss := snapshot.Get().All

		if err := json.NewEncoder(w).Encode(ss); err != nil {
			http.Error(w, "failed to encode status", http.StatusInternalServerError)
			return
		}
	})

	h := handlers.New(dbpool)
	r.Get("/uptime", h.GetUptime)
	r.Get("/uptime/all", h.GetUptimeAll)
	// Serve Vite build output from /app/web/dist
	fs := http.FileServer(http.Dir("./web/dist"))

	// Assets (Vite outputs /assets/*)
	r.Handle("/assets/*", fs)

	// SPA fallback: serve index.html for everything else that isn't an API route
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./web/dist/index.html")
	})

	fmt.Println(cfg)
	http.ListenAndServe(":8080", r)
}

func toMonitorTargets(ct []config.Target) []monitor.Target {
	out := make([]monitor.Target, 0, len(ct))
	for _, t := range ct {
		enabled := true
		if t.Enabled != nil {
			enabled = *t.Enabled
		}

		out = append(out, monitor.Target{
			Name:           t.Name,
			URL:            t.URL,
			Method:         t.Method,
			Interval:       t.IntervalDur,
			Timeout:        t.TimeoutDur,
			ExpectedStatus: t.ExpectedStatus,
			Contains:       t.Contains,
			MaxBodyBytes:   t.MaxBodyBytes,
			Enabled:        enabled,
			Tags:           t.Tags,
		})
	}

	return out
}
