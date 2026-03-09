# 变更记录

本文件用于记录 `peekgit` 的重要变更。

格式参考 Keep a Changelog，版本号遵循 Semantic Versioning。

## [Unreleased]

### Added

- 新增 `--workspaces` 命令行参数，支持扫描当前目录作为工作区，支持可选深度（`--workspaces` 默认深度为 1，`--workspaces=0` 仅检查当前目录本身，`--workspaces=n` 检查到第 n 层子目录）
- 重新设计 PR/Issues 列表界面，使用对齐的表格行、相对更新时间显示和稳定的选择高亮
- 命令帮助信息固定显示在底部，详情加载提示靠近项目标题
- 优化表格头部样式和选中项颜色（绿色高亮）
- 新增 `.air.toml` 配置文件，支持本地开发热重载工作流
- 补充配置解析、表格渲染和底部行为相关的测试覆盖
- 新增开源协作与治理基础文件（许可证、贡献指南、行为准则、安全策略）
- 新增 GitHub Issue/PR 模板与基础 CI 工作流

## [v0.1.19]

### Added
- 新增工作区扫描模式（Workspace Scan Mode），支持通过命令行参数快速扫描目录
- 统一 PR 和 Issues 表格的交互体验（UX）

## [v0.1.18]

### Changed
- 全面美化 UI/UX 视觉效果
- 优化超大 PR Diff 的加载与显示支持

### Fixed
- 修复界面动画效果相关的异常

## [v0.1.17]

### Added
- 在 README 中补充环境要求、Lazygit 安装说明及项目致谢

## [v0.1.16]

### Added
- 首页 PR 列表支持按下 `d` 键直接查看代码 Diff

## [v0.1.15]

### Fixed
- Release 工作流改用 GitHub App Token 检出代码，以确保有权限推送更新

## [v0.1.14]

### Fixed
- 移除 `.goreleaser.yaml` 中重复的 `skip_upload` 配置项

## [v0.1.13]

### Changed
- 统一 Release 流程，合并 Manifest 更新逻辑并修复路径处理问题

## [v0.1.12]

### Changed
- 合并 Homebrew 和 Scoop 的构建提交，消除冗余的 GitHub Action 运行记录

## [v0.1.11]

### Fixed
- 修复 Release 工作流中重复执行相同功能的问题

## [v0.1.10]

### Fixed
- 在 CI 配置中将 `RELEASE_APP_ID` 从变量（vars）改为加密密钥（secrets）

## [v0.1.9]

### Fixed
- GoReleaser 流程改用 GitHub App Token 进行身份验证

## [v0.1.8]

### Fixed
- 修复 Homebrew 和 Scoop 的自动更新 CI 流程

## [v0.1.7]

### Fixed
- 修复发布流程中的权限失败问题
- 统一 `peekgit` 命令入口

## [v0.1.6]

### Fixed
- 修正 GoReleaser 配置，确保 Homebrew formula 发布到正确的 tap 仓库

## [v0.1.5]

### Changed
- 全面优化 TUI 界面视觉效果

## [v0.1.4]

### Fixed
- 移除 PR 合并后重复触发的 CI 运行

## [v0.1.3]

### Fixed
- 修复发布流程中因受保护分支导致的推送失败问题

## [v0.1.2]

### Added
- 优化 Windows 安装说明与 Scoop 安装清单

## [v0.1.1]

### Changed
- 更新 CI/CD 配置以支持自动小版本发布

## [v0.1.0]

### Added
- 初始版本发布
- 将命令名从 `repo-monitor` 正式更名为 `peekgit`
- 添加 Homebrew 和 Scoop 的发布配置
