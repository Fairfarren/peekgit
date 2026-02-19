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

func isPathTraversal(rel string) bool {
	if rel == ".." {
		return true
	}
	if strings.HasPrefix(rel, "../") {
		return true
	}
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return true
	}
	if filepath.IsAbs(rel) {
		return true
	}
	return false
}

func scanConfiguredPaths(root string, paths []string) ([]RepoDir, error) {
	// 将 root 转为绝对路径，用于后续验证
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	repos := make([]RepoDir, 0, len(paths))
	for _, rel := range paths {
		// 安全检查：拒绝绝对路径
		if filepath.IsAbs(rel) {
			continue
		}

		absPath := filepath.Join(absRoot, rel)

		// 安全检查：确保最终路径在 workspace 根目录内
		// 通过计算相对路径来验证，如果相对路径以 .. 开头则说明在 workspace 之外
		relToRoot, err := filepath.Rel(absRoot, absPath)
		if err != nil {
			continue
		}
		if isPathTraversal(relToRoot) {
			continue
		}

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

	repos := make([]RepoDir, 0, len(entries)+1) // +1 预留给根目录

	// 首先检查根目录本身是否是 Git 仓库
	if ok, err := IsGitRepo(root); err == nil && ok {
		repos = append(repos, RepoDir{Name: filepath.Base(root), Path: root})
	}

	// 然后扫描子目录
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
