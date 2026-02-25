package gitcli

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Fairfarren/peekgit/internal/model"
)

type Executor interface {
	Run(ctx context.Context, dir string, args ...string) (string, error)
}

type CLI struct {
	exec Executor
}

func New() *CLI { return &CLI{exec: osExecutor{}} }

func NewWithExecutor(ex Executor) *CLI { return &CLI{exec: ex} }

type osExecutor struct{}

func (o osExecutor) Run(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", errors.New(strings.TrimSpace(stderr.String()))
		}
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func (c *CLI) RefreshRepo(ctx context.Context, repoName string, repoPath string) model.RepoStatus {
	status := model.RepoStatus{Name: repoName, Path: repoPath, Sync: model.SyncUnknown, UpdatedAt: time.Now()}

	if _, err := c.exec.Run(ctx, repoPath, "rev-parse", "--is-inside-work-tree"); err != nil {
		status.Error = model.RepoErrNotARepo
		return status
	}

	branch := c.currentBranch(ctx, repoPath)
	status.Branch = branch

	if _, err := c.exec.Run(ctx, repoPath, "remote", "get-url", "origin"); err != nil {
		status.Error = model.RepoErrNoRemote
	}

	if status.Error != model.RepoErrNoRemote {
		if _, err := c.exec.Run(ctx, repoPath, "fetch", "origin", "--prune", "--quiet"); err != nil {
			status.Error = model.RepoErrFetch
		}
	}

	upstream := c.resolveUpstream(ctx, repoPath, branch)
	status.Upstream = upstream

	if upstream != "" {
		ahead, behind, err := c.aheadBehind(ctx, repoPath, upstream)
		if err == nil {
			status.Ahead = ahead
			status.Behind = behind
			status.Sync = classifySync(ahead, behind)
		} else {
			status.Sync = model.SyncUnknown
		}
	} else {
		status.Sync = model.SyncUnknown
	}

	status.Dirty = c.isDirty(ctx, repoPath)
	return status
}

func (c *CLI) currentBranch(ctx context.Context, repoPath string) string {
	branch, err := c.exec.Run(ctx, repoPath, "symbolic-ref", "--short", "HEAD")
	if err == nil && branch != "" {
		return branch
	}
	sha, err := c.exec.Run(ctx, repoPath, "rev-parse", "--short", "HEAD")
	if err != nil || sha == "" {
		return "unknown"
	}
	return "detached@" + sha
}

func (c *CLI) resolveUpstream(ctx context.Context, repoPath string, branch string) string {
	up, err := c.exec.Run(ctx, repoPath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err == nil && up != "" {
		return up
	}
	if branch == "" || strings.HasPrefix(branch, "detached@") {
		return ""
	}
	fallback := "origin/" + branch
	if _, err := c.exec.Run(ctx, repoPath, "show-ref", "--verify", "refs/remotes/"+fallback); err == nil {
		return fallback
	}
	return ""
}

func (c *CLI) aheadBehind(ctx context.Context, repoPath string, upstream string) (int, int, error) {
	out, err := c.exec.Run(ctx, repoPath, "rev-list", "--left-right", "--count", "HEAD..."+upstream)
	if err != nil {
		return 0, 0, err
	}
	parts := strings.Fields(out)
	if len(parts) != 2 {
		return 0, 0, errors.New("unexpected rev-list output")
	}
	ahead, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	behind, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return ahead, behind, nil
}

func (c *CLI) isDirty(ctx context.Context, repoPath string) bool {
	out, err := c.exec.Run(ctx, repoPath, "status", "--porcelain")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) != ""
}

func classifySync(ahead int, behind int) model.SyncState {
	switch {
	case ahead == 0 && behind == 0:
		return model.SyncSynced
	case ahead > 0 && behind == 0:
		return model.SyncAhead
	case ahead == 0 && behind > 0:
		return model.SyncBehind
	case ahead > 0 && behind > 0:
		return model.SyncDiverged
	default:
		return model.SyncUnknown
	}
}

func (c *CLI) ParseOwnerRepoFromRemote(ctx context.Context, repoPath string) (string, string, error) {
	url, err := c.exec.Run(ctx, repoPath, "remote", "get-url", "origin")
	if err != nil {
		return "", "", err
	}
	return ParseOwnerRepo(url)
}

func ParseOwnerRepo(remoteURL string) (string, string, error) {
	s := strings.TrimSpace(remoteURL)
	s = strings.TrimSuffix(s, ".git")

	if strings.HasPrefix(s, "git@github.com:") {
		rest := strings.TrimPrefix(s, "git@github.com:")
		parts := splitGitHubPath(rest)
		if len(parts) >= 2 {
			return parts[0], parts[1], nil
		}
	}

	if strings.HasPrefix(s, "https://github.com/") {
		rest := strings.TrimPrefix(s, "https://github.com/")
		parts := splitGitHubPath(rest)
		if len(parts) >= 2 {
			return parts[0], parts[1], nil
		}
	}

	return "", "", errors.New("unsupported remote url")
}

func splitGitHubPath(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" && p != "." {
			out = append(out, p)
		}
	}
	return out
}

func (c *CLI) ListBranches(ctx context.Context, repoPath string, dirty bool) ([]model.BranchInfo, error) {
	out, err := c.exec.Run(ctx, repoPath, "for-each-ref", "--format=%(refname:short)|%(upstream:short)|%(HEAD)", "refs/heads")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) == "" {
		return []model.BranchInfo{}, nil
	}
	lines := strings.Split(out, "\n")
	branches := make([]model.BranchInfo, 0, len(lines))
	for _, line := range lines {
		parts := strings.Split(line, "|")
		if len(parts) != 3 {
			continue
		}
		current := strings.TrimSpace(parts[2]) == "*"
		b := model.BranchInfo{Name: parts[0], Upstream: parts[1], Current: current, Dirty: dirty}
		if b.Upstream != "" {
			ahead, behind, err := c.aheadBehind(ctx, repoPath, b.Upstream)
			if err == nil {
				b.Ahead = ahead
				b.Behind = behind
				b.SyncSymbol = model.SyncSymbol(classifySync(ahead, behind), ahead, behind)
			} else {
				b.SyncSymbol = "—"
			}
		} else {
			b.SyncSymbol = "—"
		}
		branches = append(branches, b)
	}
	return branches, nil
}

func (c *CLI) CheckoutBranch(ctx context.Context, repoPath string, branchName string) error {
	_, err := c.exec.Run(ctx, repoPath, "checkout", branchName)
	return err
}

// Pull executes git pull for the current branch in the specified repository.
func (c *CLI) Pull(ctx context.Context, repoPath string) error {
	_, err := c.exec.Run(ctx, repoPath, "pull", "--quiet")
	return err
}

// HasPendingChanges checks if a repository has pending changes that need attention.
// It checks for:
// - Local uncommitted changes (dirty working tree)
// - Local commits not pushed to remote (ahead > 0)
// This is a lightweight check that performs a quick git fetch.
func (c *CLI) HasPendingChanges(ctx context.Context, repoPath string) bool {
	// Check if working tree is dirty
	if c.isDirty(ctx, repoPath) {
		return true
	}

	// Get current branch
	branch := c.currentBranch(ctx, repoPath)
	if branch == "" {
		return false
	}

	// Try to get upstream branch
	upstream := c.resolveUpstream(ctx, repoPath, branch)
	if upstream == "" {
		// No upstream configured, can't check for unpushed commits
		return false
	}

	// Quick fetch to get remote info (with short timeout to be lightweight)
	fetchCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if _, err := c.exec.Run(fetchCtx, repoPath, "fetch", "origin", "--quiet"); err != nil {
		// Fetch failed, can't determine ahead/behind
		return false
	}

	// Check if local is ahead of remote
	ahead, behind, err := c.aheadBehind(ctx, repoPath, upstream)
	if err != nil {
		return false
	}

	// Has pending if ahead (local has commits not pushed) or behind (needs pull)
	return ahead > 0 || behind > 0
}

// HasRemoteUpdate checks if a repository has remote updates that need to be pulled.
// It checks if local branch is behind remote (needs pull).
// This is a lightweight check that performs a quick git fetch.
func (c *CLI) HasRemoteUpdate(ctx context.Context, repoPath string) bool {
	// Get current branch
	branch := c.currentBranch(ctx, repoPath)
	if branch == "" {
		return false
	}

	// Try to get upstream branch
	upstream := c.resolveUpstream(ctx, repoPath, branch)
	if upstream == "" {
		// No upstream configured, can't check for remote updates
		return false
	}

	// Quick fetch to get remote info (with short timeout to be lightweight)
	fetchCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if _, err := c.exec.Run(fetchCtx, repoPath, "fetch", "origin", "--quiet"); err != nil {
		// Fetch failed, can't determine behind
		return false
	}

	// Check if local is behind remote (needs pull)
	_, behind, err := c.aheadBehind(ctx, repoPath, upstream)
	if err != nil {
		return false
	}

	return behind > 0
}
