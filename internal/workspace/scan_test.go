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

	repos, err := ScanRepos(root, nil)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("repo count = %d", len(repos))
	}
}

func TestScanReposWithConfiguredPaths(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "apps", "my-repo")
	_ = os.MkdirAll(filepath.Join(deep, ".git"), 0o755)
	_ = os.MkdirAll(filepath.Join(root, "apps", "not-repo"), 0o755)

	repos, err := ScanRepos(root, []string{"apps/my-repo", "apps/not-repo"})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("repo count = %d, want 1", len(repos))
	}
	if repos[0].Name != "my-repo" {
		t.Fatalf("name = %s, want my-repo", repos[0].Name)
	}
	if repos[0].Path != deep {
		t.Fatalf("path = %s, want %s", repos[0].Path, deep)
	}
}

func TestScanReposConfiguredPathsSkipsMissing(t *testing.T) {
	root := t.TempDir()

	repos, err := ScanRepos(root, []string{"does/not/exist"})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(repos) != 0 {
		t.Fatalf("repo count = %d, want 0", len(repos))
	}
}

func TestIsGitRepoInvalidGitFile(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git"), []byte("invalid"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	ok, err := IsGitRepo(repo)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if ok {
		t.Fatalf("expected false")
	}
}

func TestIsGitRepoMissingGitDirFromGitFile(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git"), []byte("gitdir: ../missing.git\n"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	ok, err := IsGitRepo(repo)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if ok {
		t.Fatalf("expected false")
	}
}

func TestScanReposRejectsAbsolutePath(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	_ = os.MkdirAll(filepath.Join(outside, "secret-repo", ".git"), 0o755)

	repos, err := ScanRepos(root, []string{outside})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(repos) != 0 {
		t.Fatalf("should reject absolute path, got %d repos", len(repos))
	}
}

func TestScanReposRejectsPathTraversal(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Dir(root)
	outside := filepath.Join(parent, "outside-repo")
	_ = os.MkdirAll(filepath.Join(outside, ".git"), 0o755)
	defer os.RemoveAll(outside)

	repos, err := ScanRepos(root, []string{"../outside-repo"})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(repos) != 0 {
		t.Fatalf("should reject path traversal, got %d repos", len(repos))
	}
}
