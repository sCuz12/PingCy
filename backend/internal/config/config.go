package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
)

type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Monitoring MonitoringConfig `yaml:"monitoring"`
	Targets    []Target         `yaml:"targets"`
}

type ServerConfig struct {
	Addr string `yaml:"addr"`
}

type MonitoringConfig struct {
	Workers       int    `yaml:"workers"`
	JobsBuffer    int    `yaml:"jobs_buffer"`
	ResultsBuffer int    `yaml:"results_buffer"`
	UserAgent     string `yaml:"user_agent"`
}
type Target struct {
	Name           string   `yaml:"name"`
	URL            string   `yaml:"url"`
	Method         string   `yaml:"method"`   // GET or HEAD
	Interval       string   `yaml:"interval"` // e.g. "30s"
	Timeout        string   `yaml:"timeout"`  // e.g. "5s"
	ExpectedStatus int      `yaml:"expected_status,omitempty"`
	Contains       string   `yaml:"contains,omitempty"`
	MaxBodyBytes   int64    `yaml:"max_body_bytes,omitempty"`
	Enabled        *bool    `yaml:"enabled,omitempty"`
	Tags           []string `yaml:"tags,omitempty"`

	// Parsed durations (filled after load)
	IntervalDur time.Duration `yaml:"-"`
	TimeoutDur  time.Duration `yaml:"-"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	applyDefaults(&cfg)

	if err := validateAndNormalize(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	// Server defaults
	if strings.TrimSpace(cfg.Server.Addr) == "" {
		cfg.Server.Addr = ":8080"
	}

	// Monitoring defaults
	if cfg.Monitoring.Workers <= 0 {
		cfg.Monitoring.Workers = 8
	}
	if cfg.Monitoring.JobsBuffer <= 0 {
		cfg.Monitoring.JobsBuffer = 200
	}
	if cfg.Monitoring.ResultsBuffer <= 0 {
		cfg.Monitoring.ResultsBuffer = 200
	}
	if strings.TrimSpace(cfg.Monitoring.UserAgent) == "" {
		cfg.Monitoring.UserAgent = "CyprusStatusMonitor/0.1"
	}

	// Target defaults
	for i := range cfg.Targets {
		t := &cfg.Targets[i]

		// enabled defaults to true
		if t.Enabled == nil {
			v := true
			t.Enabled = &v
		}

		if strings.TrimSpace(t.Method) == "" {
			t.Method = "GET"
		}
		if strings.TrimSpace(t.Interval) == "" {
			t.Interval = "30s"
		}
		if strings.TrimSpace(t.Timeout) == "" {
			t.Timeout = "5s"
		}
		if t.ExpectedStatus == 0 {
			t.ExpectedStatus = 200
		}
		if t.MaxBodyBytes == 0 {
			t.MaxBodyBytes = 64 * 1024 // 64KB
		}
	}
}

func validateAndNormalize(cfg *Config) error {
	if len(cfg.Targets) == 0 {
		return errors.New("config: no targets provided")
	}

	seen := make(map[string]struct{}, len(cfg.Targets))

	for i := range cfg.Targets {
		t := &cfg.Targets[i]

		t.Name = strings.TrimSpace(t.Name)
		t.URL = strings.TrimSpace(t.URL)
		t.Method = strings.ToUpper(strings.TrimSpace(t.Method))

		if t.Name == "" {
			return fmt.Errorf("config: target[%d] missing name", i)
		}
		if _, ok := seen[t.Name]; ok {
			return fmt.Errorf("config: duplicate target name %q", t.Name)
		}
		seen[t.Name] = struct{}{}

		if t.URL == "" {
			return fmt.Errorf("config: target %q missing url", t.Name)
		}
		if !strings.HasPrefix(t.URL, "http://") && !strings.HasPrefix(t.URL, "https://") {
			return fmt.Errorf("config: target %q url must start with http:// or https://", t.Name)
		}

		switch t.Method {
		case "GET", "HEAD":
		default:
			return fmt.Errorf("config: target %q invalid method %q (use GET or HEAD)", t.Name, t.Method)
		}

		intervalDur, err := time.ParseDuration(t.Interval)
		if err != nil {
			return fmt.Errorf("config: target %q invalid interval %q: %w", t.Name, t.Interval, err)
		}
		if intervalDur <= 0 {
			return fmt.Errorf("config: target %q interval must be > 0", t.Name)
		}
		t.IntervalDur = intervalDur

		timeoutDur, err := time.ParseDuration(t.Timeout)
		if err != nil {
			return fmt.Errorf("config: target %q invalid timeout %q: %w", t.Name, t.Timeout, err)
		}
		if timeoutDur <= 0 {
			return fmt.Errorf("config: target %q timeout must be > 0", t.Name)
		}
		t.TimeoutDur = timeoutDur

		if t.ExpectedStatus < 100 || t.ExpectedStatus > 599 {
			return fmt.Errorf("config: target %q expected_status must be 100..599", t.Name)
		}

		if t.MaxBodyBytes < 0 {
			return fmt.Errorf("config: target %q max_body_bytes cannot be negative", t.Name)
		}

		// If using HEAD, contains check won’t work (no body). Allow it but warn by failing fast for clarity.
		if t.Method == "HEAD" && strings.TrimSpace(t.Contains) != "" {
			return fmt.Errorf("config: target %q uses method HEAD but has contains check; use GET instead", t.Name)
		}
	}

	return nil
}
