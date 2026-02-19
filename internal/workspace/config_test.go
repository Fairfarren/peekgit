package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFileNotExist(t *testing.T) {
	root := t.TempDir()
	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(cfg.Repos) != 0 {
		t.Fatalf("repos=%v", cfg.Repos)
	}
}

func TestLoadConfigWithRepos(t *testing.T) {
	root := t.TempDir()
	content := "repos:\n  - apps/repo-a\n  - packages/repo-b\n"
	if err := os.WriteFile(filepath.Join(root, ConfigFileName), []byte(content), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(cfg.Repos) != 2 {
		t.Fatalf("repos count=%d", len(cfg.Repos))
	}
	if cfg.Repos[0] != "apps/repo-a" {
		t.Fatalf("repos[0]=%s", cfg.Repos[0])
	}
	if cfg.Repos[1] != "packages/repo-b" {
		t.Fatalf("repos[1]=%s", cfg.Repos[1])
	}
}

func TestLoadConfigEmptyRepos(t *testing.T) {
	root := t.TempDir()
	content := "repos:\n"
	if err := os.WriteFile(filepath.Join(root, ConfigFileName), []byte(content), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(cfg.Repos) != 0 {
		t.Fatalf("repos=%v", cfg.Repos)
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	root := t.TempDir()
	content := "repos: [invalid\n"
	if err := os.WriteFile(filepath.Join(root, ConfigFileName), []byte(content), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	_, err := LoadConfig(root)
	if err == nil {
		t.Fatalf("expected error for invalid yaml")
	}
}
