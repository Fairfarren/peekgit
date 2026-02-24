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

配置好 `~/.config/peekgit/config.json` 后，在任意目录直接运行：

```bash
repo-monitor
```

程序会首先展示工作区（Workspace）列表卡片，选中后按 Enter 即可进入多仓库监控面板，查看具体的仓库状态。

## 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--interval` | 300 | 自动刷新间隔（秒） |
| `--concurrency` | 3 | 并发 fetch 数量 |
| `--no-github` | false | 禁用 GitHub 功能（PR、Issues） |

```bash
repo-monitor --interval 60 --concurrency 5
```

## 全局配置文件

PeekGit 使用全局配置文件 `~/.config/peekgit/config.json` 来定义你的工作区和对应监控的 Git 仓库路径。

你可以将常用的项目分门别类配置在里面（支持 `~` 展开到用户目录）：

```json
{
  "workspaces": {
    "Fairfarren": [
      "~/work/www/peekgit",
      "~/open-source/awesome-project"
    ],
    "My Work": [
      "~/company/frontend",
      "~/company/backend-api"
    ]
  }
}
```

**行为规则**：

- 程序启动时默认优先展示你在 `json` 中配置的全部 Workspaces 卡片列表。
- 选择某个 Workspace 后，加载该 Workspace 中定义的所有仓库。

## GitHub 集成

程序会自动尝试获取 GitHub Token，用于显示 PR 和 Issues 信息。支持以下方式（按优先级）：

1. 环境变量 `GITHUB_TOKEN`
2. `gh auth token`（GitHub CLI）

未认证时程序正常运行，只是不显示 PR / Issues 数据。

## 键盘操作

### 工作区列表视图 (Workspaces)

| 按键 | 操作 |
|------|------|
| `↑↓←→` / `h j k l` | 选择工作区卡片 |
| `Space` / `Enter` | 进入相应工作区的仓库列表 |
| `q` | 退出 |

### 仓库列表视图 (Home)

| 按键 | 操作 |
|------|------|
| `↑↓←→` / `h j k l` | 选择仓库 |
| `Enter` | 进入仓库详情 |
| `/` | 过滤仓库名称 |
| `r` | 手动刷新 |
| `f` | pull 当前仓库 |
| `F` | pull 所有仓库 |
| `g` | 在当前仓库目录打开 lazygit |
| `q` / `Esc` | 返回工作区列表 |

### 详情页

| 按键 | 操作 |
|------|------|
| `Tab` / `←→` | 切换 PR / Issues 标签 |
| `1` `2` | 快速切换标签 |
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
| `q` | 返回详情页 |

## 状态图标

### 仓库状态（详情页）

| 图标 | 含义 |
|------|------|
| ✓ | 与远端同步 |
| ↑ | 本地领先远端 |
| ↓ | 本地落后远端 |
| ✎ | 有未提交的修改 |
| ! | 错误（无远端、fetch 失败等） |

### Workspace 卡片状态

| 图标 | 含义 |
|------|------|
| ↻ | 正在检查远端更新 |
| ↓ | 有仓库需要 pull |

## 项目结构

```
cmd/repo-monitor/      CLI 入口
internal/
  config/              命令行参数解析与 config.json 解析
  workspace/           仓库有效性验证与绝对路径扫描
  gitcli/              本地 Git 操作
  provider/github/     GitHub API 集成与缓存
  model/               共享数据模型
  tui/                 终端界面
  cache/               TTL 缓存
```
