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
	cfg, err := Parse([]string{"-interval", "60", "-concurrency", "5", "-no-github"})
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if cfg.IntervalSec != 60 {
		t.Errorf("got %d, want 60", cfg.IntervalSec)
	}
	if cfg.Concurrency != 5 {
		t.Errorf("got %d, want 5", cfg.Concurrency)
	}
	if !cfg.NoGitHub {
		t.Errorf("got false, want true")
	}
}

func TestParseFlags(t *testing.T) {
	cfg, err := Parse([]string{"--no-github"})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if !cfg.NoGitHub {
		t.Fatalf("expected no-github true")
	}
}
