package config

import "testing"

func TestParseDefaults(t *testing.T) {
	cfg, err := Parse([]string{})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if cfg.IntervalSec != DefaultIntervalSec {
		t.Fatalf("interval = %d", cfg.IntervalSec)
	}
	if cfg.Concurrency != DefaultConcurrency {
		t.Fatalf("concurrency = %d", cfg.Concurrency)
	}
}

func TestParseNormalizeInvalidValues(t *testing.T) {
	cfg, err := Parse([]string{"--interval", "0", "--concurrency", "-1"})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if cfg.IntervalSec != DefaultIntervalSec {
		t.Fatalf("interval = %d", cfg.IntervalSec)
	}
	if cfg.Concurrency != DefaultConcurrency {
		t.Fatalf("concurrency = %d", cfg.Concurrency)
	}
}

func TestParseFlags(t *testing.T) {
	cfg, err := Parse([]string{"--workspace", ".", "--no-github"})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if cfg.Workspace == "." {
		t.Fatalf("workspace should be absolute")
	}
	if !cfg.NoGitHub {
		t.Fatalf("expected no-github true")
	}
}
