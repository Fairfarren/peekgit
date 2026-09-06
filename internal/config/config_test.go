package config

import (
	"errors"
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

func TestParseVersion(t *testing.T) {
	tests := [][]string{
		{"--version"},
		{"-version"},
		{"-v"},
	}

	for _, tc := range tests {
		cfg, err := Parse(tc)
		if err != nil {
			t.Fatalf("Parse(%v) error: %v", tc, err)
		}
		if !cfg.ShowVersion {
			t.Errorf("Parse(%v) ShowVersion = false, want true", tc)
		}
	}
}

func TestParseHelp(t *testing.T) {
	tests := [][]string{
		{"--help"},
		{"-h"},
	}

	for _, tc := range tests {
		_, err := Parse(tc)
		if err == nil {
			t.Fatalf("Parse(%v) error = nil, want flag.ErrHelp", tc)
		}
	}
}

func TestValidateLimits(t *testing.T) {
	// Case 1: negative and zero
	interval := -1
	concurrency := 0
	validateLimits(&interval, &concurrency)
	if interval != DefaultIntervalSec {
		t.Errorf("interval = %d, want %d", interval, DefaultIntervalSec)
	}
	if concurrency != DefaultConcurrency {
		t.Errorf("concurrency = %d, want %d", concurrency, DefaultConcurrency)
	}

	// Case 2: zero and negative
	interval = 0
	concurrency = -1
	validateLimits(&interval, &concurrency)
	if interval != DefaultIntervalSec {
		t.Errorf("interval = %d, want %d", interval, DefaultIntervalSec)
	}
	if concurrency != DefaultConcurrency {
		t.Errorf("concurrency = %d, want %d", concurrency, DefaultConcurrency)
	}

	// Case 3: positive values should be preserved
	interval = 10
	concurrency = 5
	validateLimits(&interval, &concurrency)
	if interval != 10 {
		t.Errorf("interval = %d, want %d", interval, 10)
	}
	if concurrency != 5 {
		t.Errorf("concurrency = %d, want %d", concurrency, 5)
	}
}

func TestLoadGlobalConfigErrors(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	// Case 1: Config does not exist -> returns empty map, nil error
	cfg, err := LoadGlobalConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Workspaces) != 0 {
		t.Fatalf("expected empty workspaces, got %v", cfg.Workspaces)
	}

	// Case 2: Config directory is not readable / file is broken JSON
	configDir := filepath.Join(home, ".config", "peekgit")
	_ = os.MkdirAll(configDir, 0o755)
	_ = os.WriteFile(filepath.Join(configDir, "config.json"), []byte("invalid json"), 0o644)
	_, err = LoadGlobalConfig()
	if err == nil {
		t.Fatalf("expected error on invalid json, got nil")
	}

	// Case 3: config.json is a directory (os.ReadFile returns EISDIR, not IsNotExist)
	_ = os.Remove(filepath.Join(configDir, "config.json"))
	_ = os.Mkdir(filepath.Join(configDir, "config.json"), 0o755)
	_, err = LoadGlobalConfig()
	if err == nil {
		t.Fatalf("expected error when config.json is directory, got nil")
	}

	// Case 4: HOME is unset
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	_, err = LoadGlobalConfig()
	if err == nil {
		t.Fatalf("expected error when HOME is unset, got nil")
	}
}

func TestBuildWorkspaceConfigGetwdError(t *testing.T) {
	orig := getwd
	defer func() { getwd = orig }()

	getwd = func() (string, error) {
		return "", errors.New("simulated getwd error")
	}

	_, err := buildWorkspaceConfig(10, 5, false, 1)
	if err == nil {
		t.Fatal("expected error from buildWorkspaceConfig, got nil")
	}
}

func TestParseInvalidFlag(t *testing.T) {
	_, err := Parse([]string{"--invalid-flag"})
	if err == nil {
		t.Fatalf("expected error on invalid flag, got nil")
	}
}

func TestParseLoadGlobalConfigError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	configDir := filepath.Join(home, ".config", "peekgit")
	_ = os.MkdirAll(configDir, 0o755)
	_ = os.WriteFile(filepath.Join(configDir, "config.json"), []byte("{broken json"), 0o644)

	_, err := Parse([]string{})
	if err == nil {
		t.Fatalf("expected error when global config is broken, got nil")
	}
}
