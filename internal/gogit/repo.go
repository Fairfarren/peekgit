package gogit

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Fairfarren/peekgit/internal/model"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// CLI 使用 go-git 库操作 Git 仓库
// 与 gitcli.CLI 保持相同的接口
type CLI struct{}

// New 创建一个新的 go-git CLI 实例
func New() *CLI {
	return &CLI{}
}

// RefreshRepo 获取仓库的完整状态信息
func (c *CLI) RefreshRepo(ctx context.Context, repoName string, repoPath string) model.RepoStatus {
	status := model.RepoStatus{
		Name:      repoName,
		Path:      repoPath,
		Sync:      model.SyncUnknown,
		UpdatedAt: time.Now(),
	}

	// 检查是否是 git 仓库
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		status.Error = model.RepoErrNotARepo
		return status
	}

	// 获取当前分支
	branch := c.currentBranch(ctx, repoPath)
	status.Branch = branch

	// 检查是否有远程
	_, err = repo.Remote("origin")
	if err != nil {
		status.Error = model.RepoErrNoRemote
		return status
	}

	// 解析上游
	upstream := c.resolveUpstream(ctx, repoPath, branch)
	status.Upstream = upstream

	if upstream != "" {
		ahead, behind, err := c.aheadBehind(ctx, repoPath, upstream)
		if err == nil {
			status.Ahead = ahead
			status.Behind = behind
			status.Sync = classifySync(ahead, behind)
		}
	}

	status.Dirty = c.isDirty(ctx, repoPath)
	return status
}

// currentBranch 获取当前分支名称
func (c *CLI) currentBranch(ctx context.Context, repoPath string) string {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "unknown"
	}

	ref, err := repo.Head()
	if err != nil {
		return "unknown"
	}

	// 检查是否是分支引用 (refs/heads/...)
	if ref.Name().IsBranch() {
		return ref.Name().Short()
	}

	// detached HEAD 状态，显示简短 commit hash
	return "detached@" + ref.Hash().String()[:7]
}

// resolveUpstream 解析分支的上游配置
func (c *CLI) resolveUpstream(ctx context.Context, repoPath string, branch string) string {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return ""
	}

	if branch == "" || strings.HasPrefix(branch, "detached@") {
		return ""
	}

	// 从配置中获取上游
	config, err := repo.Config()
	if err != nil {
		// 如果无法获取配置，尝试使用默认的 origin/branch
		fallback := "origin/" + branch
		_, err := repo.Reference(plumbing.NewRemoteReferenceName("origin", branch), true)
		if err == nil {
			return fallback
		}
		return ""
	}

	branchConfig := config.Branches[branch]
	if branchConfig.Remote != "" && branchConfig.Merge != "" {
		// 构建上游引用名称
		mergeRef := plumbing.ReferenceName(branchConfig.Merge)
		if mergeRef.IsBranch() {
			return fmt.Sprintf("%s/%s", branchConfig.Remote, mergeRef.Short())
		}
	}

	// 如果配置中没有上游，尝试使用默认的 origin/branch
	fallback := "origin/" + branch
	_, err = repo.Reference(plumbing.NewRemoteReferenceName("origin", branch), true)
	if err == nil {
		return fallback
	}

	return ""
}

// aheadBehind 计算本地与上游的提交差异
func (c *CLI) aheadBehind(ctx context.Context, repoPath string, upstream string) (int, int, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return 0, 0, err
	}

	// 获取本地 HEAD 引用
	headRef, err := repo.Head()
	if err != nil {
		return 0, 0, err
	}

	// 解析上游引用
	parts := strings.Split(upstream, "/")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("invalid upstream format: %s", upstream)
	}
	remote := parts[0]
	branchName := strings.Join(parts[1:], "/")

	upstreamRefName := plumbing.NewRemoteReferenceName(remote, branchName)
	upstreamRef, err := repo.Reference(upstreamRefName, true)
	if err != nil {
		return 0, 0, err
	}

	// 计算差异
	ahead, behind, err := c.countDivergence(repo, headRef.Hash(), upstreamRef.Hash())
	return ahead, behind, err
}

// countDivergence 计算两个提交之间的分叉数
func (c *CLI) countDivergence(repo *git.Repository, local, remote plumbing.Hash) (int, int, error) {
	// 找到共同祖先
	ancestor, err := c.findMergeBase(repo, local, remote)
	if err != nil {
		return 0, 0, err
	}

	// 计算本地到祖先的距离（ahead）
	ahead, err := c.countCommits(repo, local, ancestor)
	if err != nil {
		return 0, 0, err
	}

	// 计算远程到祖先的距离（behind）
	behind, err := c.countCommits(repo, remote, ancestor)
	if err != nil {
		return 0, 0, err
	}

	return ahead, behind, nil
}

// findMergeBase 查找两个提交的共同祖先
func (c *CLI) findMergeBase(repo *git.Repository, h1, h2 plumbing.Hash) (plumbing.Hash, error) {
	// 获取第一个提交的所有祖先
	ancestors := make(map[plumbing.Hash]bool)
	iter1, err := repo.Log(&git.LogOptions{
		From:  h1,
		Order: git.LogOrderCommitterTime,
	})
	if err != nil {
		return plumbing.ZeroHash, err
	}
	defer iter1.Close()

	err = iter1.ForEach(func(c *object.Commit) error {
		ancestors[c.Hash] = true
		return nil
	})
	if err != nil {
		return plumbing.ZeroHash, err
	}

	// 遍历第二个提交的祖先，找到第一个共同的
	iter2, err := repo.Log(&git.LogOptions{
		From:  h2,
		Order: git.LogOrderCommitterTime,
	})
	if err != nil {
		return plumbing.ZeroHash, err
	}
	defer iter2.Close()

	var mergeBase plumbing.Hash
	found := false
	err = iter2.ForEach(func(c *object.Commit) error {
		if ancestors[c.Hash] {
			mergeBase = c.Hash
			found = true
			return fmt.Errorf("found") // 停止遍历
		}
		return nil
	})

	if !found {
		return plumbing.ZeroHash, fmt.Errorf("no common ancestor found")
	}

	return mergeBase, err
}

// countCommits 计算从一个提交到另一个提交的步数
func (c *CLI) countCommits(repo *git.Repository, from, to plumbing.Hash) (int, error) {
	count := 0
	commitIter, err := repo.Log(&git.LogOptions{
		From:  from,
		Order: git.LogOrderCommitterTime,
	})
	if err != nil {
		return 0, err
	}
	defer commitIter.Close()

	err = commitIter.ForEach(func(c *object.Commit) error {
		if c.Hash == to {
			return fmt.Errorf("found") // 停止遍历
		}
		count++
		return nil
	})

	return count, nil
}

// isDirty 检查工作区是否有未提交的更改
func (c *CLI) isDirty(ctx context.Context, repoPath string) bool {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return false
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return false
	}

	status, err := worktree.Status()
	if err != nil {
		return false
	}

	return !status.IsClean()
}

// ListBranches 列出所有本地分支
func (c *CLI) ListBranches(ctx context.Context, repoPath string, dirty bool) ([]model.BranchInfo, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, err
	}

	// 获取当前 HEAD
	headRef, err := repo.Head()
	if err != nil {
		return nil, err
	}

	// 迭代所有分支引用
	branchIter, err := repo.Branches()
	if err != nil {
		return nil, err
	}
	defer branchIter.Close()

	var branches []model.BranchInfo
	err = branchIter.ForEach(func(ref *plumbing.Reference) error {
		if !ref.Name().IsBranch() {
			return nil
		}

		branchName := ref.Name().Short()

		// 判断是否是当前分支
		headHash := headRef.Hash()
		branchHash := ref.Hash()
		isCurrent := headHash == branchHash

		// 获取上游
		var upstream string
		config, err := repo.Config()
		if err == nil {
			branchConfig := config.Branches[branchName]
			if branchConfig.Remote != "" && branchConfig.Merge != "" {
				mergeRef := plumbing.ReferenceName(branchConfig.Merge)
				if mergeRef.IsBranch() {
					upstream = fmt.Sprintf("%s/%s", branchConfig.Remote, mergeRef.Short())
				}
			}
		}

		b := model.BranchInfo{
			Name:     branchName,
			Upstream: upstream,
			Current:  isCurrent,
			Dirty:    dirty,
		}

		// 如果有上游，计算差异
		if upstream != "" {
			ahead, behind, err := c.aheadBehind(ctx, repoPath, upstream)
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
		return nil
	})

	return branches, nil
}

// CheckoutBranch 切换到指定分支
func (c *CLI) CheckoutBranch(ctx context.Context, repoPath string, branchName string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return err
	}

	return worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(branchName),
	})
}

// Pull 拉取远程更新
// 注意：由于 go-git 对 SSH 认证支持有限，这里使用命令行方式
// 这样可以利用系统的 SSH agent 和密钥配置
func (c *CLI) Pull(ctx context.Context, repoPath string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "pull", "--quiet")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pull failed: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

// ParseOwnerRepoFromRemote 从远程 URL 解析 owner 和 repo
func (c *CLI) ParseOwnerRepoFromRemote(ctx context.Context, repoPath string) (string, string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", "", err
	}

	remote, err := repo.Remote("origin")
	if err != nil {
		return "", "", err
	}

	url := remote.Config().URLs[0]
	return ParseOwnerRepo(url)
}

// ParseOwnerRepo 从远程 URL 解析 owner 和 repo（复用 gitcli 的实现）
func ParseOwnerRepo(remoteURL string) (string, string, error) {
	s := strings.TrimSpace(remoteURL)
	s = strings.TrimSuffix(s, ".git")

	if strings.HasPrefix(s, "git@github.com:") {
		rest := strings.TrimPrefix(s, "git@github.com:")
		parts := strings.Split(rest, "/")
		if len(parts) == 2 {
			return parts[0], parts[1], nil
		}
	}

	if strings.HasPrefix(s, "https://github.com/") {
		rest := strings.TrimPrefix(s, "https://github.com/")
		parts := strings.Split(rest, "/")
		if len(parts) >= 2 {
			return parts[0], parts[1], nil
		}
	}

	return "", "", fmt.Errorf("unsupported remote url")
}

// classifySync 根据 ahead 和 behind 判断同步状态
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
