package config

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestLoadGlobalConfigExpandHomePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	configDir := filepath.Join(home, ".config", "peekgit")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	content := []byte("{\"workspaces\":{\"ws\":[\"~/projects/repo1\",\"~\",\"relative/path\"]}}")
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), content, 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	cfg, err := LoadGlobalConfig()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	paths := cfg.Workspaces["ws"]
	if len(paths) != 3 {
		t.Fatalf("paths count = %d", len(paths))
	}

	if paths[0] != filepath.Join(home, "projects", "repo1") {
		t.Fatalf("paths[0] = %q", paths[0])
	}
	if paths[1] != home {
		t.Fatalf("paths[1] = %q", paths[1])
	}
	if paths[2] != "relative/path" {
		t.Fatalf("paths[2] = %q", paths[2])
	}
}
