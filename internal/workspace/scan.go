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

func ScanRepos(configuredPaths []string) ([]RepoDir, error) {
	return scanConfiguredPaths(configuredPaths)
}

func scanConfiguredPaths(paths []string) ([]RepoDir, error) {
	repos := make([]RepoDir, 0, len(paths))
	for _, p := range paths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			continue
		}

		ok, err := IsGitRepo(absPath)
		if err != nil || !ok {
			continue
		}
		repos = append(repos, RepoDir{Name: filepath.Base(absPath), Path: absPath})
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
