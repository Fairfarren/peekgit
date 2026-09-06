package workspace

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type RepoDir struct {
	Name string
	Path string
}

func ScanRepos(configuredPaths []string) ([]RepoDir, error) {
	return scanConfiguredPaths(configuredPaths)
}

func ScanReposWithDepth(root string, depth int) ([]RepoDir, error) {
	if depth <= 0 {
		depth = 0
	}
	absRoot, _ := filepath.Abs(root)

	repos := make([]RepoDir, 0)
	seen := make(map[string]struct{})

	walkDirectory(absRoot, depth, 0, seen, &repos)
	sort.Slice(repos, func(i, j int) bool { return repos[i].Path < repos[j].Path })
	return repos, nil
}

func addRepoIfFound(path string, seen map[string]struct{}, repos *[]RepoDir) {
	ok, err := IsGitRepo(path)
	if err == nil && ok {
		if _, exists := seen[path]; !exists {
			seen[path] = struct{}{}
			*repos = append(*repos, RepoDir{Name: filepath.Base(path), Path: path})
		}
	}
}

func walkDirectory(path string, depth, d int, seen map[string]struct{}, repos *[]RepoDir) {
	addRepoIfFound(path, seen, repos)

	if d >= depth {
		return
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != ".git" {
			walkDirectory(filepath.Join(path, entry.Name()), depth, d+1, seen, repos)
		}
	}
}

func expandIfWildcard(p string) ([]RepoDir, bool) {
	if strings.HasSuffix(p, "/*") || strings.HasSuffix(p, "\\*") {
		parentPath := p[:len(p)-2]
		expanded, err := expandWildcardPath(parentPath)
		if err == nil {
			return expanded, true
		}
		return nil, true
	}
	return nil, false
}

func scanConfiguredPaths(paths []string) ([]RepoDir, error) {
	repos := make([]RepoDir, 0, len(paths))
	for _, p := range paths {
		if expanded, handled := expandIfWildcard(p); handled {
			if len(expanded) > 0 {
				repos = append(repos, expanded...)
			}
			continue
		}

		absPath, _ := filepath.Abs(p)

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

	return isGitWorktreeFile(path, gitPath)
}

func isGitWorktreeFile(repoPath, gitFilePath string) (bool, error) {
	b, err := os.ReadFile(gitFilePath)
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
		gdir = filepath.Join(repoPath, gdir)
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

func normalizeParentPath(parentPath string) string {
	if parentPath == "" {
		return string(filepath.Separator)
	}
	if len(parentPath) == 2 && parentPath[1] == ':' {
		return parentPath + string(filepath.Separator)
	}
	return parentPath
}

// expandWildcardPath scans the parent directory and returns all git repo subdirectories
func expandWildcardPath(parentPath string) ([]RepoDir, error) {
	absParent, _ := filepath.Abs(normalizeParentPath(parentPath))

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
