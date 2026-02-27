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
		// Handle wildcard path ending with /*
		if strings.HasSuffix(p, "/*") || strings.HasSuffix(p, "\\*") {
			parentPath := p[:len(p)-2]
			expanded, err := expandWildcardPath(parentPath)
			if err != nil {
				continue
			}
			repos = append(repos, expanded...)
			continue
		}

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

// expandWildcardPath scans the parent directory and returns all git repo subdirectories
func expandWildcardPath(parentPath string) ([]RepoDir, error) {
	// Handle edge case where wildcard was applied to filesystem root
	// e.g., "/*" becomes "" or "C:\\*" becomes "C:"
	if parentPath == "" {
		parentPath = string(filepath.Separator)
	} else if len(parentPath) == 2 && parentPath[1] == ':' {
		// Windows drive letter without separator (e.g., "C:")
		parentPath = parentPath + string(filepath.Separator)
	}

	absParent, err := filepath.Abs(parentPath)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(absParent)
	if err != nil {
		return nil, err
	}

	var repos []RepoDir
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirPath := filepath.Join(absParent, entry.Name())
		ok, err := IsGitRepo(dirPath)
		if err != nil || !ok {
			continue
		}
		repos = append(repos, RepoDir{Name: entry.Name(), Path: dirPath})
	}
	return repos, nil
}
