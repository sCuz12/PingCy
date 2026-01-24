package main

import (
	"context"
	"cy-platforms-status-monitor/internal/config"
	"cy-platforms-status-monitor/internal/monitor"
	"cy-platforms-status-monitor/internal/snapshot"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const CONFIGS_PATH = "./configs/config.yaml"

func main() {

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	cfg, err := config.Load(CONFIGS_PATH)
	if err != nil {
		log.Fatal(err)
	}

	// Root context for the whole app (later you’ll cancel this on SIGINT)
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
	_ = eventsCh // not used yet (MVP)

	targetsToMonitor := toMonitorTargets(cfg.Targets)

	monitor.StartWorkers(ctx, cfg.Monitoring.Workers, client, jobsCh, resultsCh, &workerWg)
	monitor.StartSchedulers(ctx, targetsToMonitor, jobsCh)

	go monitor.Aggregator(ctx, resultsCh, eventsCh)

	go func() {
		for e := range eventsCh {
			fmt.Println("EVENT")
			fmt.Println(e.TargetName, e.From, e.To)
		}
	}()

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
