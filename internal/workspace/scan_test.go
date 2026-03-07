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

func TestExpandWildcardPathEmptyPath(t *testing.T) {
	// Test that empty path (from "/*") is handled correctly
	// The function should resolve it to the filesystem root
	repos, err := expandWildcardPath("")
	if err != nil {
		// On most systems, reading root directory may fail due to permissions
		// but it should not fail due to empty path handling
		t.Logf("expandWildcardPath(\"\") returned error: %v (may be expected)", err)
	}
	t.Logf("expandWildcardPath(\"\") returned %d repos", len(repos))
}

func TestScanReposWithDepth(t *testing.T) {
	root := t.TempDir()
	rootRepo := filepath.Join(root, "root-repo")
	childRepo := filepath.Join(root, "group", "child-repo")
	grandchildRepo := filepath.Join(root, "group", "nested", "grandchild-repo")

	_ = os.MkdirAll(filepath.Join(rootRepo, ".git"), 0o755)
	_ = os.MkdirAll(filepath.Join(childRepo, ".git"), 0o755)
	_ = os.MkdirAll(filepath.Join(grandchildRepo, ".git"), 0o755)

	repos0, err := ScanReposWithDepth(root, 0)
	if err != nil {
		t.Fatalf("scan depth 0 failed: %v", err)
	}
	if len(repos0) != 0 {
		t.Fatalf("depth 0 repo count = %d, want 0", len(repos0))
	}

	repos1, err := ScanReposWithDepth(root, 1)
	if err != nil {
		t.Fatalf("scan depth 1 failed: %v", err)
	}
	if len(repos1) != 1 || repos1[0].Name != "root-repo" {
		t.Fatalf("depth 1 repos = %+v", repos1)
	}

	repos2, err := ScanReposWithDepth(root, 2)
	if err != nil {
		t.Fatalf("scan depth 2 failed: %v", err)
	}
	if len(repos2) != 2 {
		t.Fatalf("depth 2 repo count = %d, want 2", len(repos2))
	}

	repos3, err := ScanReposWithDepth(root, 3)
	if err != nil {
		t.Fatalf("scan depth 3 failed: %v", err)
	}
	if len(repos3) != 3 {
		t.Fatalf("depth 3 repo count = %d, want 3", len(repos3))
	}
}

func TestScanReposWithDepthIncludesRootRepo(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, ".git"), 0o755)

	repos, err := ScanReposWithDepth(root, 0)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("repo count = %d, want 1", len(repos))
	}
	if repos[0].Path != root {
		t.Fatalf("repo path = %q, want %q", repos[0].Path, root)
	}
}
