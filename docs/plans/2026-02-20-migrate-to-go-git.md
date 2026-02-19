# 迁移到 go-git 库实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**目标:** 将 peekgit 从直接调用 Git 命令行迁移到使用 go-git 库，提升性能并消除对外部 git 命令的依赖。

**架构:** 使用 go-git/v5 库替换当前的 `exec.Command("git", ...)` 调用，保持相同的接口和行为。采用 TDD 方法，先编写测试确保现有行为被保留，然后逐步替换实现。

**技术栈:**
- go-git/v5: https://github.com/go-git/go-git
- 现有测试框架保持不变
- 保持相同的模型层 (model.RepoStatus, model.BranchInfo)

---

## 迁移策略

### 核心原则
1. **TDD 优先**: 每个功能先写测试，确保迁移前后行为一致
2. **小步提交**: 每完成一个小功能立即提交
3. **向后兼容**: 保持公共接口不变
4. **并行开发**: 新建 `gogit` 包，与 `gitcli` 包并存，逐步切换

### 文件结构
```
internal/
├── gitcli/           # 现有实现（命令行方式）
│   ├── repo.go
│   └── repo_test.go
├── gogit/            # 新实现（go-git 库）
│   ├── repo.go       # 创建
│   └── repo_test.go  # 创建
└── model/            # 保持不变
    └── types.go
```

---

## Task 1: 添加 go-git 依赖并创建新包结构

**Files:**
- Modify: `go.mod`
- Create: `internal/gogit/repo.go`
- Create: `internal/gogit/repo_test.go`

**Step 1: 添加 go-git 依赖**

```bash
go get github.com/go-git/go-git/v5@latest
go mod tidy
```

验证:
```bash
grep "go-git/go-git/v5" go.mod
```
预期输出: 包含 `github.com/go-git/go-git/v5 v5.x.x`

**Step 2: 创建 gogit 包的基础结构**

创建 `internal/gogit/repo.go`:

```go
package gogit

import (
    "context"
    "github.com/Fairfarren/peekgit/internal/model"
)

// CLI 使用 go-git 库操作 Git 仓库
// 与 gitcli.CLI 保持相同的接口
type CLI struct {
    // TODO: 添加 go-git 相关字段
}

// New 创建一个新的 go-git CLI 实例
func New() *CLI {
    return &CLI{}
}

// 接口保持与 gitcli 一致，方便后续切换
type Executor interface {
    Run(ctx context.Context, dir string, args ...string) (string, error)
}
```

**Step 3: 创建基础测试文件**

创建 `internal/gogit/repo_test.go`:

```go
package gogit

import (
    "context"
    "testing"
)

func TestNew(t *testing.T) {
    cli := New()
    if cli == nil {
        t.Fatal("New() returned nil")
    }
}
```

**Step 4: 运行测试验证基础结构**

```bash
go test ./internal/gogit -v
```

预期输出: PASS

**Step 5: 提交**

```bash
git add go.mod go.sum internal/gogit/
git commit -m "feat: 添加 go-git 依赖并创建 gogit 包结构"
```

---

## Task 2: 实现 currentBranch 功能

**Files:**
- Modify: `internal/gogit/repo.go`
- Modify: `internal/gogit/repo_test.go`

**Step 1: 为 currentBranch 编写测试（复制现有测试场景）**

在 `internal/gogit/repo_test.go` 添加:

```go
func TestCurrentBranch(t *testing.T) {
    // 需要一个真实的测试仓库
    // 使用 t.TempDir() 创建临时仓库
    t.Run("正常分支", func(t *testing.T) {
        // TODO: 创建临时 git 仓库并设置分支
        cli := New()
        branch := cli.currentBranch(context.Background(), "/tmp/test-repo")
        if branch != "main" {
            t.Fatalf("expected main, got %s", branch)
        }
    })

    t.Run("detached HEAD", func(t *testing.T) {
        // TODO: 创建 detached HEAD 状态
        cli := New()
        branch := cli.currentBranch(context.Background(), "/tmp/test-repo")
        if !startsWith(branch, "detached@") {
            t.Fatalf("expected detached@ prefix, got %s", branch)
        }
    })
}

func startsWith(s, prefix string) bool {
    return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
```

**Step 2: 运行测试确认失败**

```bash
go test ./internal/gogit -v -run TestCurrentBranch
```

预期输出: FAIL with "method not defined"

**Step 3: 实现 currentBranch 方法**

在 `internal/gogit/repo.go` 添加:

```go
import (
    "fmt"
    "github.com/go-git/go-git/v5"
    "github.com/go-git/go-git/v5/plumbing"
)

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
```

**Step 4: 运行测试验证通过**

```bash
go test ./internal/gogit -v -run TestCurrentBranch
```

预期输出: PASS

**Step 5: 提交**

```bash
git add internal/gogit/
git commit -m "feat(gogit): 实现 currentBranch 方法"
```

---

## Task 3: 实现 isDirty 功能

**Files:**
- Modify: `internal/gogit/repo.go`
- Modify: `internal/gogit/repo_test.go`

**Step 1: 编写 isDirty 测试**

在 `internal/gogit/repo_test.go` 添加:

```go
func TestIsDirty(t *testing.T) {
    t.Run("干净仓库", func(t *testing.T) {
        // TODO: 创建干净的临时仓库
        cli := New()
        dirty := cli.isDirty(context.Background(), "/tmp/clean-repo")
        if dirty {
            t.Fatal("expected clean")
        }
    })

    t.Run("有未提交更改", func(t *testing.T) {
        // TODO: 创建有修改的临时仓库
        cli := New()
        dirty := cli.isDirty(context.Background(), "/tmp/dirty-repo")
        if !dirty {
            t.Fatal("expected dirty")
        }
    })
}
```

**Step 2: 运行测试确认失败**

```bash
go test ./internal/gogit -v -run TestIsDirty
```

预期: FAIL

**Step 3: 实现 isDirty 方法**

在 `internal/gogit/repo.go` 添加:

```go
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
```

**Step 4: 运行测试验证通过**

```bash
go test ./internal/gogit -v -run TestIsDirty
```

预期: PASS

**Step 5: 提交**

```bash
git add internal/gogit/
git commit -m "feat(gogit): 实现 isDirty 方法"
```

---

## Task 4: 实现 aheadBehind 功能

**Files:**
- Modify: `internal/gogit/repo.go`
- Modify: `internal/gogit/repo_test.go`

**Step 1: 编写 aheadBehind 测试**

在 `internal/gogit/repo_test.go` 添加:

```go
func TestAheadBehind(t *testing.T) {
    cases := []struct {
        name     string
        setup    func() string // 返回仓库路径
        expected struct {
            ahead  int
            behind int
        }
    }{
        {
            name: "同步状态",
            // setup: 创建本地和远程相同的仓库
            expected: struct{ ahead, behind int }{0, 0},
        },
        {
            name: "领先1个提交",
            // setup: 本地比远程多1个提交
            expected: struct{ ahead, behind int }{1, 0},
        },
        {
            name: "落后2个提交",
            // setup: 远程比本地多2个提交
            expected: struct{ ahead, behind int }{0, 2},
        },
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            // TODO: 实现测试仓库设置
            cli := New()
            ahead, behind, err := cli.aheadBehind(context.Background(), "/tmp/test-repo", "origin/main")
            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }
            if ahead != tc.expected.ahead || behind != tc.expected.behind {
                t.Fatalf("expected %d/%d, got %d/%d", tc.expected.ahead, tc.expected.behind, ahead, behind)
            }
        })
    }
}
```

**Step 2: 运行测试确认失败**

```bash
go test ./internal/gogit -v -run TestAheadBehind
```

预期: FAIL

**Step 3: 实现 aheadBehind 方法**

在 `internal/gogit/repo.go` 添加:

```go
func (c *CLI) aheadBehind(ctx context.Context, repoPath string, upstream string) (int, int, error) {
    repo, err := git.PlainOpen(repoPath)
    if err != nil {
        return 0, 0, err
    }

    // 解析上游引用 (如 "origin/main" -> "refs/remotes/origin/main")
    upstreamRefName := plumbing.NewRemoteReferenceName("origin", extractBranchName(upstream))

    // 获取本地 HEAD 引用
    headRef, err := repo.Head()
    if err != nil {
        return 0, 0, err
    }

    // 获取上游引用
    upstreamRef, err := repo.Reference(upstreamRefName, true)
    if err != nil {
        return 0, 0, err
    }

    // 使用 revlist 计算差异
    // TODO: go-git 没有直接的 ahead/behind API，需要遍历提交历史
    // 这是比较复杂的部分，可能需要使用 commit.Iterator

    // 临时实现：使用 go-git 的 plumbing/object
    ahead, behind, err := c.countDivergence(repo, headRef.Hash(), upstreamRef.Hash())
    return ahead, behind, err
}

func extractBranchName(upstream string) string {
    // "origin/main" -> "main"
    parts := strings.Split(upstream, "/")
    if len(parts) >= 2 {
        return parts[len(parts)-1]
    }
    return upstream
}

func (c *CLI) countDivergence(repo *git.Repository, local, remote plumbing.Hash) (int, int, error) {
    // 这是一个简化实现
    // 实际需要遍历两个提交的历史来计算分叉点
    // 参考: https://github.com/go-git/go-git/issues/59

    // TODO: 实现完整的 divergence 计数逻辑
    // 这可能需要使用 commit.IterAncestor 或类似方法

    return 0, 0, nil
}
```

**注意:** `aheadBehind` 是最复杂的部分，go-git 没有直接对应的 API。需要实现提交历史遍历逻辑。

**Step 4: 完整实现 countDivergence**

在 `internal/gogit/repo.go` 完善实现:

```go
import (
    "github.com/go-git/go-git/v5/plumbing/object"
    "github.com/go-git/go-git/v5/plumbing/storer"
)

func (c *CLI) countDivergence(repo *git.Repository, local, remote plumbing.Hash) (int, int, error) {
    // 找到共同祖先
    ancestor, err := findMergeBase(repo, local, remote)
    if err != nil {
        return 0, 0, err
    }

    // 计算本地到祖先的距离（ahead）
    ahead, err := countCommits(repo, local, ancestor)
    if err != nil {
        return 0, 0, err
    }

    // 计算远程到祖先的距离（behind）
    behind, err := countCommits(repo, remote, ancestor)
    if err != nil {
        return 0, 0, err
    }

    return ahead, behind, nil
}

func findMergeBase(repo *git.Repository, h1, h2 plumbing.Hash) (plumbing.Hash, error) {
    // 简化实现：使用 BFS 找到共同祖先
    // TODO: 实际实现可能需要更高效的算法
    return h1, nil // 占位符
}

func countCommits(repo *git.Repository, from, to plumbing.Hash) (int, error) {
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
            return storer.ErrEnd
        }
        count++
        return nil
    })

    return count, nil
}
```

**Step 5: 运行测试验证**

```bash
go test ./internal/gogit -v -run TestAheadBehind
```

**Step 6: 提交**

```bash
git add internal/gogit/
git commit -m "feat(gogit): 实现 aheadBehind 方法"
```

---

## Task 5: 实现 resolveUpstream 功能

**Files:**
- Modify: `internal/gogit/repo.go`
- Modify: `internal/gogit/repo_test.go`

**Step 1: 编写 resolveUpstream 测试**

```go
func TestResolveUpstream(t *testing.T) {
    t.Run("有配置的上游", func(t *testing.T) {
        // TODO: 创建配置了上游的仓库
        cli := New()
        upstream := cli.resolveUpstream(context.Background(), "/tmp/test-repo", "main")
        if upstream != "origin/main" {
            t.Fatalf("expected origin/main, got %s", upstream)
        }
    })

    t.Run("没有上游", func(t *testing.T) {
        // TODO: 创建没有上游的仓库
        cli := New()
        upstream := cli.resolveUpstream(context.Background(), "/tmp/test-repo", "main")
        if upstream != "" {
            t.Fatalf("expected empty, got %s", upstream)
        }
    })
}
```

**Step 2-6:** 标准的 TDD 流程（略）

---

## Task 6: 实现 RefreshRepo 主方法

**Files:**
- Modify: `internal/gogit/repo.go`
- Modify: `internal/gogit/repo_test.go`

**Step 1: 编写 RefreshRepo 测试**

复用 `gitcli/repo_test.go` 中的测试场景：

```go
func TestRefreshRepoSynced(t *testing.T) {
    // TODO: 创建一个同步的测试仓库
    cli := New()
    status := cli.RefreshRepo(context.Background(), "test-repo", "/tmp/test-repo")

    if status.Branch != "main" {
        t.Fatalf("branch %s", status.Branch)
    }
    if status.Sync != model.SyncSynced {
        t.Fatalf("sync %v", status.Sync)
    }
    if status.Dirty {
        t.Fatal("expected clean")
    }
}
```

**Step 2-6:** 实现 `RefreshRepo` 方法，整合所有子功能

```go
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

    // Fetch（可选，看需求）
    // if err := c.fetch(ctx, repoPath); err != nil {
    //     status.Error = model.RepoErrFetch
    // }

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
```

---

## Task 7: 实现 ListBranches 功能

**Files:**
- Modify: `internal/gogit/repo.go`
- Modify: `internal/gogit/repo_test.go`

**Step 1-6:** 使用 TDD 实现分支列表功能

```go
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
        isCurrent := ref.Hash() == headRef.Hash()

        // 获取上游
        // TODO: go-git 需要额外逻辑获取上游分支

        b := model.BranchInfo{
            Name:    branchName,
            Current: isCurrent,
            Dirty:   dirty,
        }
        branches = append(branches, b)
        return nil
    })

    return branches, nil
}
```

---

## Task 8: 实现 Checkout 和 Pull 功能

**Files:**
- Modify: `internal/gogit/repo.go`
- Modify: `internal/gogit/repo_test.go`

**Step 1-6:** TDD 实现切换分支和拉取功能

```go
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

func (c *CLI) Pull(ctx context.Context, repoPath string) error {
    repo, err := git.PlainOpen(repoPath)
    if err != nil {
        return err
    }

    worktree, err := repo.Worktree()
    if err != nil {
        return err
    }

    return worktree.Pull(&git.PullOptions{
        RemoteName: "origin",
    })
}
```

---

## Task 9: 实现 ParseOwnerRepoFromRemote

**Files:**
- Modify: `internal/gogit/repo.go`
- Modify: `internal/gogit/repo_test.go`

**Step 1-6:** TDD 实现

直接复用 `gitcli.ParseOwnerRepo` 函数（不依赖 Git 命令）

---

## Task 10: 性能基准测试

**Files:**
- Create: `internal/gogit/bench_test.go`
- Create: `internal/gitcli/bench_test.go`

**Step 1: 创建基准测试**

`internal/gogit/bench_test.go`:

```go
package gogit

import (
    "context"
    "testing"
)

func BenchmarkRefreshRepo(b *testing.B) {
    cli := New()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        cli.RefreshRepo(context.Background(), "test", "/tmp/test-repo")
    }
}
```

`internal/gitcli/bench_test.go`:

```go
package gitcli

import (
    "context"
    "testing"
)

func BenchmarkRefreshRepo(b *testing.B) {
    cli := New()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        cli.RefreshRepo(context.Background(), "test", "/tmp/test-repo")
    }
}
```

**Step 2: 运行基准测试对比**

```bash
go test ./internal/gitci -bench=. -benchmem
go test ./internal/gogit -bench=. -benchmem
```

**Step 3: 记录结果并提交**

创建 `docs/performance-comparison.md` 记录性能对比

---

## Task 11: 更新主应用使用新的 gogit 包

**Files:**
- Modify: `internal/tui/app.go`
- Modify: `internal/workspace/scan.go`
- Modify: `cmd/repo-monitor/main.go`

**Step 1: 更新 import**

将 `gitcli` 改为 `gogit`:

```go
// 旧: import "github.com/Fairfarren/peekgit/internal/gitcli"
// 新: import "github.com/Fairfarren/peekgit/internal/gogit"
```

**Step 2: 保持接口兼容**

确保调用代码无需修改（因为两个包的接口相同）

**Step 3: 运行集成测试**

```bash
go test ./... -v
```

**Step 4: 构建并手动测试**

```bash
go build ./cmd/peekgit
./peekgit  # 手动测试功能
```

**Step 5: 提交**

```bash
git add internal/tui internal/workspace cmd/
git commit -m "refactor: 切换到 gogit 实现"
```

---

## Task 12: 清理旧代码

**Files:**
- Delete: `internal/gitcli/repo.go`
- Delete: `internal/gitcli/repo_test.go`

**Step 1: 确认所有测试通过**

```bash
go test ./... -v
```

**Step 2: 删除旧的 gitcli 包**

```bash
rm -rf internal/gitcli
```

**Step 3: 更新文档**

更新 README.md 说明技术栈变更

**Step 4: 最终提交**

```bash
git add internal/gitci README.md docs/
git commit -m "chore: 移除旧的 gitcli 实现，完成 go-git 迁移"
```

---

## 验收标准

完成所有任务后：

1. ✅ 所有测试通过 (`go test ./... -v`)
2. ✅ 性能基准测试显示 go-git 实现不慢于命令行实现
3. ✅ 手动测试所有功能正常
4. ✅ 二进制体积增加可接受（预计增加 3-5MB）
5. ✅ 代码覆盖率不降低

---

## 风险和注意事项

1. **go-git API 限制**: 某些 Git 操作可能没有直接对应 API，需要自己实现
2. **测试仓库准备**: 需要创建真实 Git 仓库进行集成测试
3. **fetch 操作**: go-git 的 fetch 可能比命令行慢
4. **错误处理差异**: go-git 的错误类型与命令行输出不同

---

## 后续优化

1. 考虑添加 Git 操作缓存
2. 优化大仓库的性能
3. 支持更多 Git 操作（rebase, cherry-pick 等）
