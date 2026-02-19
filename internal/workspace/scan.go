package workspace

import (
	"os"
	"path/filepath"
	"strings"
)

type RepoDir struct {
	Name string
	Path string
}

func ScanRepos(root string, configuredPaths []string) ([]RepoDir, error) {
	if len(configuredPaths) > 0 {
		return scanConfiguredPaths(root, configuredPaths)
	}
	return scanDirectChildren(root)
}

func scanConfiguredPaths(root string, paths []string) ([]RepoDir, error) {
	repos := make([]RepoDir, 0, len(paths))
	for _, rel := range paths {
		absPath := filepath.Join(root, rel)
		ok, err := IsGitRepo(absPath)
		if err != nil || !ok {
			continue
		}
		repos = append(repos, RepoDir{Name: filepath.Base(rel), Path: absPath})
	}
	return repos, nil
}

func scanDirectChildren(root string) ([]RepoDir, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	repos := make([]RepoDir, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		repoPath := filepath.Join(root, entry.Name())
		ok, err := IsGitRepo(repoPath)
		if err != nil || !ok {
			continue
		}
		repos = append(repos, RepoDir{Name: entry.Name(), Path: repoPath})
	}
	return repos, nil
}

func IsGitRepo(path string) (bool, error) {
	gitPath := filepath.Join(path, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	if info.IsDir() {
		return true, nil
	}

	b, err := os.ReadFile(gitPath)
	if err != nil {
		return false, err
	}
	line := strings.TrimSpace(string(b))
	if !strings.HasPrefix(line, "gitdir:") {
		return false, nil
	}
	gdir := strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
	if gdir == "" {
		return false, nil
	}
	if !filepath.IsAbs(gdir) {
		gdir = filepath.Join(path, gdir)
	}
	st, err := os.Stat(gdir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return st.IsDir(), nil
}
