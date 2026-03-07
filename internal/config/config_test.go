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

func TestParseWorkspaceFlagNoValueUsesDepthOne(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	root := t.TempDir()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	absRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}

	cfg, err := Parse([]string{"-workspaces"})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if !cfg.WorkspaceMode {
		t.Fatalf("workspace mode should be enabled")
	}
	if cfg.WorkspaceDepth != 1 {
		t.Fatalf("workspace depth = %d, want 1", cfg.WorkspaceDepth)
	}
	if cfg.WorkspaceRoot != absRoot {
		t.Fatalf("workspace root = %q, want %q", cfg.WorkspaceRoot, absRoot)
	}
	paths := cfg.Global.Workspaces[absRoot]
	if len(paths) != 1 || paths[0] != absRoot {
		t.Fatalf("unexpected workspace paths: %+v", paths)
	}
}

func TestParseWorkspaceFlagWithDepth(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	root := t.TempDir()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}

	cfg, err := Parse([]string{"-workspaces", "2"})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if !cfg.WorkspaceMode {
		t.Fatalf("workspace mode should be enabled")
	}
	if cfg.WorkspaceDepth != 2 {
		t.Fatalf("workspace depth = %d, want 2", cfg.WorkspaceDepth)
	}
}

func TestParseWorkspaceFlagNormalizesNegativeDepth(t *testing.T) {
	cfg, err := Parse([]string{"-workspaces=-3"})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if cfg.WorkspaceDepth != 0 {
		t.Fatalf("workspace depth = %d, want 0", cfg.WorkspaceDepth)
	}
}

func TestParseWorkspaceFlagIgnoresBrokenGlobalConfig(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	configDir := filepath.Join(home, ".config", "peekgit")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte("{broken"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	root := t.TempDir()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}

	cfg, err := Parse([]string{"-workspaces"})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if !cfg.WorkspaceMode {
		t.Fatalf("workspace mode should be enabled")
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
