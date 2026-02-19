# Repo Monitor TUI 需求文档（0 配置 Workspace 多仓库监控 + PR/Issues 只读）

## 0. 目标与范围

**目标**：在终端提供一个 TUI 工具，用于在一个顶层目录（workspace）下监控多个本地 Git 仓库，并只读查看远端（以 GitHub 为主）的 PR、PR Diff、Issues。

**核心价值**：

* 无需进入各仓库目录：在顶层一屏看完“哪些仓库有更新/待推送/异常”
* 选中仓库进入详情：查看 PR、issues、分支状态与 PR diff（只读）

**明确不做**：

* 不创建/修改/关闭 PR 或 Issue
* 不提交/merge/rebase（`pull` 可作为后续可选能力；MVP 不包含）

---

## 1. 0 配置原则（启动即用）

### 1.1 启动方式

用户在 workspace 目录运行：

* `repo-monitor`（默认扫描当前目录）

可选参数（不是配置文件）：

* `repo-monitor --workspace /path/to/ws`
* `repo-monitor --interval 300`
* `repo-monitor --concurrency 3`

### 1.2 默认行为（无任何配置文件）

* workspace = 当前工作目录
* 扫描范围 = 当前目录 **一级子目录**（scanDepth=1）
* 识别仓库 = 子目录内存在 `.git/`（或 gitfile 指向，见实现）
* 自动刷新间隔 = 300 秒（5 分钟）
* fetch 并发数 = 3
* PR/Issues：如果可获取 token 则展示，否则显示 `-`/“unauth”

### 1.3 GitHub 认证（0 配置）

按优先级自动尝试：

1. 环境变量 `GITHUB_TOKEN`
2. 如果安装了 GitHub CLI：执行 `gh auth token` 获取 token
3. 都没有：PR/Issues/Diff 功能降级不可用，但本地仓库状态仍可用

---

## 2. 用户场景

workspace 目录结构示例：

```
~/work/ws/
  repo-a/
  repo-b/
  repo-c/
```

用户诉求：

* 顶层看到多个仓库卡片：仓库名 + PR 数 + Issues 数 + 当前分支状态（待推送/有更新）
* 选中一个仓库进入详情：Tabs 切换 PR / Issues / Branches
* PR 支持查看 diff、review 代码改动（只读）

---

## 3. 功能需求

## 3.1 首页：Workspace 卡片列表

**展示方式**：根据当前终端宽度自适应排列卡片（支持 1 列或多列），并保留选中态。

**布局规则（新增）**：

* 终端宽度不足时：`1` 列（每行一个 card）
* 终端宽度足够时：自动计算多列（每行多个 card）
* 建议常量：`cardMinWidth=44`、`cardGap=2`
* 列数计算：`columns = max(1, floor((availableWidth + cardGap) / (cardMinWidth + cardGap)))`
* 卡片宽度：`cardWidth = floor((availableWidth - cardGap*(columns-1)) / columns)`
* 收到窗口尺寸变化后立即重排，但不改变当前选中 repo

**Card 展示字段**：

* Repo 名称（目录名）
* 当前分支名（branch）
* 分支同步状态（相对 upstream）：

  * `✓` 同步（ahead=0 & behind=0）
  * `↑N` 待推送
  * `↓N` 有更新可拉取
  * `↑N ↓M` diverged
  * `—` 无 upstream 或不可计算
* 工作区状态（可选）：

  * `✎` dirty（有未提交改动）
* PR 数量（open PR count；无 token 显示 `-`）
* Issues 数量（open issues count；无 token 显示 `-`）
* 错误（如果有）：

  * `! fetch` / `! auth` / `! no-remote` / `! not-a-repo`

**交互**：

* `↑↓` / `j k`：切换选中卡片
* `←→` / `h l`：在多列布局下横向切换（可选）
* `Enter`：进入该仓库详情页
* `r`：刷新（触发所有仓库的本地状态更新；可选同时刷新 counts）
* `/`：过滤（按 repo 名搜索，建议做）
* `q`：退出

---

## 3.2 仓库详情页：Tabs

顶部信息：

* `repo-name (branch: xxx  status: ↑/↓/✓/—)`
  Tabs：
* `PRs | Issues | Branches`
  内容区：
* 根据 tab 显示列表

通用交互：

* `Esc` / `Backspace`：返回首页
* `Tab` / `←→`：切 tab
* `1/2/3`：直达 PR/Issues/Branches
* `r`：刷新当前仓库远端数据（PR/Issues/branches）

---

## 3.3 PR Tab（只读）

列表字段：

* `#编号` + 标题
* 作者
* 更新时间
* 状态：open / draft（可选）

交互：

* `↑↓`：选择
* `d`：打开 diff 阅读页（MVP 推荐支持）
* `Enter`：PR 详情页（可选，MVP 可不做）
* `o`：浏览器打开 PR（可选）

---

## 3.4 PR Diff 阅读页（只读）

* 以 diff 文本滚动展示（viewport）
* 支持搜索：`/`、`n/p`
* `Esc` 返回 PR 列表

---

## 3.5 Issues Tab（只读）

列表字段：

* `#编号` + 标题
* labels（可选）
* 更新时间

交互：

* `↑↓` 选择
* `Enter` 打开 Issue 阅读页（正文+评论，可选）
* `o` 浏览器打开（可选）

---

## 3.6 Branches Tab（只读）

列表字段：

* 本地分支名
* upstream
* ahead/behind
* dirty（可选）
* 当前分支标识

---

## 4. 非功能需求

* UI 不阻塞：所有 I/O（fetch、API 请求）异步处理，显示 loading 状态
* 并发限制：本地 fetch 默认并发 3，避免对远端造成瞬时压力
* 兼容：macOS/Linux 优先；Windows 可作为后续
* 错误可视化：每个 repo 独立报错，不影响其他 repo 刷新
* 响应式布局：终端 resize 后首页卡片自动重排，避免换行错乱
* 样式可读性：状态信息（`✓`/`↑`/`↓`/`✎`/`!`）需具备稳定颜色语义与对比度

---

# 5. 技术栈与实现方式

## 5.1 技术栈

* Go 1.22+
* TUI：

  * `github.com/charmbracelet/bubbletea`
  * `github.com/charmbracelet/lipgloss`
  * `github.com/charmbracelet/bubbles`（list / viewport / textinput）
* 本地 Git：

  * `os/exec` 调用 git CLI
* GitHub：

  * `github.com/google/go-github/v57/github`
  * token 自动获取（env 或 gh）

---

## 5.2 关键实现方式（必须说明清楚）

### 5.2.1 扫描 workspace（0 配置）

* 默认扫描当前目录的一级子目录
* 识别 repo：

  * 目录中存在 `.git` **目录** 或 `.git` **文件**（worktree/submodule 场景）
  * `.git` 为文件时，解析其内容 `gitdir: <path>` 来确认

### 5.2.2 当前分支名

* `git symbolic-ref --short HEAD`
* 若失败：`git rev-parse --short HEAD` → `detached@<sha>`

### 5.2.3 upstream 获取与回退策略

* 优先：`git rev-parse --abbrev-ref --symbolic-full-name @{upstream}`
* 若失败：尝试 `origin/<branch>`（先检查 ref 是否存在）
* 再失败：显示 `no-upstream`，状态为 `—`

### 5.2.4 ahead/behind 计算

* `git rev-list --left-right --count HEAD...<upstreamRef>`
* 解析输出得到 `ahead`, `behind`

### 5.2.5 dirty 判断（可选但建议）

* `git status --porcelain` 非空 → `✎`

### 5.2.6 PR/Issues 数据获取（只读）

* 从 origin remote URL 推断 `owner/repo`：

  * 支持 `git@github.com:owner/repo.git`
  * 支持 `https://github.com/owner/repo.git`
* PR 列表：GitHub API 列 open PR（可分页）
* Issues 列表：列 open issues（排除 PR issue，可设置过滤）
* PR diff：`PullRequests.GetRaw(... Diff)` 拉取 diff 文本

### 5.2.7 缓存与刷新策略

* 本地状态：

  * 自动：每 300 秒触发刷新（ticker）
  * 手动：`r` 立即刷新
* 远端 API：

  * 进入详情页才拉列表（lazy load）
  * 首页 counts 可先显示 `-`，或仅对选中项实时拉（建议 MVP 简化）
  * 所有 API 加 60s 缓存（避免反复切 tab 触发多次请求）

### 5.2.8 首页卡片自适应布局（新增）

* 在 `tea.WindowSizeMsg` 中维护 `width/height`，触发首页布局重算
* 计算 `availableWidth`（终端宽度减去容器 padding/margin）
* 按 `cardMinWidth` 与 `cardGap` 计算列数与每列宽度
* 使用 `lipgloss.Width/Height/Size` 校验卡片渲染尺寸，避免错位
* 使用 `lipgloss.JoinHorizontal` 组装单行卡片，再用 `lipgloss.JoinVertical` 拼接多行
* 性能取舍：优先简单公式与分行渲染，避免在 MVP 引入额外网格依赖

### 5.2.9 Lip Gloss 样式建议（新增）

* 卡片分层：容器样式（全局） + 卡片样式（默认） + 选中卡片样式（高亮）
* 色彩策略：优先 `lipgloss.AdaptiveColor`，兼容浅色/深色终端背景
* 边框策略：默认卡片用 `NormalBorder`，选中卡片可用 `RoundedBorder` 或高亮边框色
* 间距策略：卡片内统一 `Padding(0~1, 1~2)`，卡片间固定 `cardGap`
* 文本策略：标题/状态/错误分层显示，避免同一行信息过密

---

# 6. TUI 原型图（ASCII）

## 6.1 首页：卡片列表（宽屏多列示例）

```
┌──────────────────────────────────────────────────────────────┐
│ Repo Monitor  (ws: ~/work/ws)                 [r] refresh    │
│ token: github ✓ / unauth                     [q] quit        │
├──────────────────────────────────────────────────────────────┤
│  ↑/↓/←/→: select   Enter: open repo   /: filter               │
├──────────────────────────────────────────────────────────────┤
│ ╔══════════════════════════╗  ┌────────────────────────────┐ │
│ ║ repo-a                ✓  ║  │ repo-b                ↑3   │ │
│ ║ branch: main  PR 3  I 1  ║  │ branch: dev   PR -  I -    │ │
│ ║ status: ↓2               ║  │ status: ↑3                 │ │
│ ╚══════════════════════════╝  └────────────────────────────┘ │
│ ┌────────────────────────────┐  ┌────────────────────────────┐ │
│ │ repo-c            ↑1 ↓4 ✎  │  │ repo-d            ! fetch  │ │
│ │ branch: feat  PR 2  I 0    │  │ branch: main  PR -  I -    │ │
│ │ status: diverged           │  │ status: auth/no-remote     │ │
│ └────────────────────────────┘  └────────────────────────────┘ │
├──────────────────────────────────────────────────────────────┤
│ Legend: ✓ synced  ↑ ahead(push)  ↓ behind(pull)  ✎ dirty     │
└──────────────────────────────────────────────────────────────┘
```

## 6.1.1 首页：卡片列表（窄屏单列示例）

```
┌──────────────────────────────────────────────────────┐
│ Repo Monitor (ws: ~/work/ws)            [r] refresh │
│ token: github ✓ / unauth                [q] quit    │
├──────────────────────────────────────────────────────┤
│  ↑/↓/←/→: select   Enter: open repo   /: filter     │
├──────────────────────────────────────────────────────┤
│ ╔══════════════════════════════════════════════════╗ │
│ ║ repo-a                                      ✓   ║ │
│ ║ branch: main                   PR 3 Issues 1    ║ │
│ ║ status: ↓2                                      ║ │
│ ╚══════════════════════════════════════════════════╝ │
│ ┌──────────────────────────────────────────────────┐ │
│ │ repo-b                                    ↑3    │ │
│ │ branch: dev                    PR - Issues -    │ │
│ │ status: ↑3                                      │ │
│ └──────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────┘
```

## 6.2 详情页：Tabs + PR 列表

```
┌──────────────────────────────────────────────────────────────┐
│ repo-a  (branch: main  ↓2)                          [Esc] back│
├──────────────────────────────────────────────────────────────┤
│ Tabs:  [PRs]  Issues  Branches                 [r] refresh   │
├──────────────────────────────────────────────────────────────┤
│ ↑/↓: select   d: diff   o: open(opt)                          │
├──────────────────────────────────────────────────────────────┤
│ > #128 Fix nil pointer in parser           alice   updated 1d │
│   #127 Refactor scheduler worker pool     bob     updated 3d │
│   #123 Feature workspace card view        carol   updated 7d │
└──────────────────────────────────────────────────────────────┘
```

## 6.3 Diff 阅读页

```
┌──────────────────────────────────────────────────────────────┐
│ repo-a  PR #128  Diff                                  [Esc] │
├──────────────────────────────────────────────────────────────┤
│ Scroll: j/k PgUp/PgDn   / search   n/p next/prev match        │
├──────────────────────────────────────────────────────────────┤
│ diff --git a/scheduler.go b/scheduler.go                      │
│ index 4d2..9a1 100644                                         │
│ --- a/scheduler.go                                            │
│ +++ b/scheduler.go                                            │
│ @@ -41,6 +41,12 @@ func Run() {                                │
│ +  if repo == nil {                                           │
│ +    return                                                   │
│ +  }                                                          │
│   ...                                                         │
└──────────────────────────────────────────────────────────────┘
```

## 6.4 Issues Tab / Branches Tab（略，同前版本）

---

# 7. CLI 参数（0 配置但可覆盖）

* `--workspace <path>`：指定 workspace（默认当前目录）
* `--interval <sec>`：自动刷新间隔（默认 300）
* `--concurrency <n>`：fetch 并发（默认 3）
* `--no-github`：禁用 GitHub 功能（只看本地状态，可选）

---

# 8. 项目结构建议（Go）

```
repo-monitor/
  cmd/repo-monitor/main.go
  internal/
    tui/          # bubbletea: models/views/msgs
    workspace/    # scan repos
    git/          # git cli wrapper
    provider/
      github/     # PR/issues/diff client
    scheduler/    # ticker + worker pool
    cache/        # time-based cache
```

---

# 9. 里程碑与验收

### MVP-1（纯本地监控）

* 0 配置启动：扫描并显示卡片列表
* 显示当前分支 + ahead/behind + dirty + 错误
* 首页卡片支持按宽度自适应（宽屏多列、窄屏单列）
* `r` 刷新不阻塞 UI
* Enter 进入详情页 + Tabs UI（内容可先空）

### MVP-2（GitHub 只读）

* 自动 token 获取（env / gh）
* PR / Issues 列表
* PR diff 阅读页

### MVP-3（体验优化）

* 首页 PR/Issue counts（GraphQL 或缓存策略）
* `/` 搜索过滤
* 错误分类更友好
* 首页卡片样式体系优化（颜色语义、选中态、边框层级、信息密度）
