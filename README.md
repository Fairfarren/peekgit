# PeekGit

终端里的多仓库监控面板。一次性查看 workspace 下所有 Git 仓库的分支状态、同步情况，以及 GitHub PR / Issues。

## 安装

```bash
go install github.com/Fairfarren/peekgit/cmd/repo-monitor@latest
```

或者克隆后本地构建：

```bash
git clone https://github.com/Fairfarren/peekgit.git
cd peekgit
go build -o peekgit ./cmd/repo-monitor
```

## 快速开始

进入包含多个仓库的目录，直接运行：

```bash
cd ~/work/my-projects
repo-monitor
```

程序会自动扫描当前目录下的所有 Git 仓库，以卡片形式展示每个仓库的状态。

## 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--workspace` | 当前目录 | workspace 根目录路径 |
| `--interval` | 300 | 自动刷新间隔（秒） |
| `--concurrency` | 3 | 并发 fetch 数量 |
| `--no-github` | false | 禁用 GitHub 功能（PR、Issues） |

```bash
repo-monitor --workspace ~/work/projects --interval 60 --concurrency 5
```

## 配置文件

在 workspace 根目录创建 `.peekgit.yaml`，可以指定要监控的仓库路径（相对路径）。

```yaml
# .peekgit.yaml
# 在 workspace 根目录放置此文件，指定要监控的仓库路径（相对路径）
# 配置后只监控列出的仓库；没有此文件则自动扫描直接子目录

repos:
  - discord-notify
  - game_portal_react
  - game_portal_admin/apps/shell_game_portal_admin_react
  - game_portal_admin/apps/user_transaction_admin_react
  - game_portal_admin/packages/game_portal_admin_shared_react
```

**行为规则**：

- 有 `.peekgit.yaml` 且 `repos` 非空 → 只监控配置中列出的仓库
- 没有配置文件或 `repos` 为空 → 自动扫描 workspace 下的直接子目录

这对 monorepo 场景特别有用——仓库可能嵌套在多层目录中，通过配置文件可以精确指定要监控的路径。

## GitHub 集成

程序会自动尝试获取 GitHub Token，用于显示 PR 和 Issues 信息。支持以下方式（按优先级）：

1. 环境变量 `GITHUB_TOKEN`
2. `gh auth token`（GitHub CLI）

未认证时程序正常运行，只是不显示 PR / Issues 数据。

## 键盘操作

### 首页

| 按键 | 操作 |
|------|------|
| `↑↓←→` / `h j k l` | 选择仓库 |
| `Enter` | 进入仓库详情 |
| `/` | 过滤仓库名称 |
| `r` | 手动刷新 |
| `q` | 退出 |

### 详情页

| 按键 | 操作 |
|------|------|
| `Tab` / `←→` | 切换 PR / Issues / Branches 标签 |
| `1` `2` `3` | 快速切换标签 |
| `↑↓` | 选择列表项 |
| `d` | 查看 PR Diff（PR 标签下） |
| `o` | 在浏览器中打开 |
| `r` | 刷新远端数据 |
| `Esc` | 返回首页 |

### Diff 查看

| 按键 | 操作 |
|------|------|
| `↑↓` / `j k` | 滚动 |
| `/` | 搜索 |
| `n` / `p` | 下一个 / 上一个匹配 |
| `Esc` | 返回详情页 |

## 状态图标

| 图标 | 含义 |
|------|------|
| ✓ | 与远端同步 |
| ↑ | 本地领先远端 |
| ↓ | 本地落后远端 |
| ✎ | 有未提交的修改 |
| ! | 错误（无远端、fetch 失败等） |

## 项目结构

```
cmd/repo-monitor/      CLI 入口
internal/
  config/              命令行参数解析
  workspace/           仓库扫描与配置文件加载
  gitcli/              本地 Git 操作
  provider/github/     GitHub API 集成与缓存
  model/               共享数据模型
  tui/                 终端界面
  cache/               TTL 缓存
```
