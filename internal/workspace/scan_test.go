package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsGitRepoDir(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	ok, err := IsGitRepo(repo)
	if err != nil || !ok {
		t.Fatalf("got (%v, %v)", ok, err)
	}
}

func TestIsGitRepoGitFile(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	gdir := filepath.Join(root, "actual.git")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.MkdirAll(gdir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git"), []byte("gitdir: ../actual.git\n"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	ok, err := IsGitRepo(repo)
	if err != nil || !ok {
		t.Fatalf("got (%v, %v)", ok, err)
	}
}

func TestScanRepos(t *testing.T) {
	root := t.TempDir()
	repo1 := filepath.Join(root, "a")
	repo2 := filepath.Join(root, "b")
	nonRepo := filepath.Join(root, "c")

	_ = os.MkdirAll(filepath.Join(repo1, ".git"), 0o755)
	_ = os.MkdirAll(filepath.Join(repo2, ".git"), 0o755)
	_ = os.MkdirAll(nonRepo, 0o755)

	repos, err := ScanRepos([]string{repo1, repo2, nonRepo})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("repo count = %d", len(repos))
	}
}

func TestScanReposSkipsMissing(t *testing.T) {
	repos, err := ScanRepos([]string{"does/not/exist"})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(repos) != 0 {
		t.Fatalf("repo count = %d, want 0", len(repos))
	}
}

func TestScanReposWildcard(t *testing.T) {
	root := t.TempDir()
	// Create multiple git repos under root
	repo1 := filepath.Join(root, "repo1")
	repo2 := filepath.Join(root, "repo2")
	nonRepo := filepath.Join(root, "non-git-dir")

	_ = os.MkdirAll(filepath.Join(repo1, ".git"), 0o755)
	_ = os.MkdirAll(filepath.Join(repo2, ".git"), 0o755)
	_ = os.MkdirAll(nonRepo, 0o755)

	// Test wildcard path /*
	repos, err := ScanRepos([]string{root + "/*"})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("repo count = %d, want 2", len(repos))
	}

	// Verify both repos are found
	found := make(map[string]bool)
	for _, r := range repos {
		found[r.Name] = true
	}
	if !found["repo1"] || !found["repo2"] {
		t.Fatalf("missing repos, found: %v", found)
	}
}
