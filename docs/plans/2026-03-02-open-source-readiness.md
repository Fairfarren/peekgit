# 项目开源准备 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为 `peekgit` 补齐最小可用的开源治理、协作与自动化基础设施。

**Architecture:** 以“最小可运行开源仓库”为目标，优先补齐法律与协作入口文件，再补齐 GitHub 模板和 CI。所有改动限定在文档和仓库配置层，不触碰业务代码与 `libs` 目录（本仓库无该目录），确保风险可控。

**Tech Stack:** Go 1.24、GitHub Actions、Markdown、YAML

---

### Task 1: 补齐开源治理核心文件

**Files:**
- Create: `LICENSE`
- Create: `CONTRIBUTING.md`
- Create: `CODE_OF_CONDUCT.md`
- Create: `SECURITY.md`
- Create: `CHANGELOG.md`

**Step 1: 草拟文件骨架**

输出每个文件的最小必要章节（目的、范围、流程、联系方式）。

**Step 2: 写入可直接使用的模板内容**

保证内容完整、语言清晰、可执行，不使用占位符式空话。

**Step 3: 自检一致性**

检查是否和仓库现状一致（默认分支、测试命令、Go 版本）。

### Task 2: 补齐 GitHub 协作与质量门禁

**Files:**
- Create: `.github/workflows/ci.yml`
- Create: `.github/ISSUE_TEMPLATE/bug_report.yml`
- Create: `.github/ISSUE_TEMPLATE/feature_request.yml`
- Create: `.github/pull_request_template.md`

**Step 1: 定义 CI 最小检查集**

包含 `go test ./...` 与 `go vet ./...`，触发事件覆盖 push 与 PR。

**Step 2: 定义 Issue/PR 模板**

保证提问信息充分，避免低质量反馈进入维护流程。

**Step 3: 对齐仓库维护方式**

模板中明确复现步骤、预期行为、影响范围与回归检查。

### Task 3: README 建立开源入口

**Files:**
- Modify: `README.md`

**Step 1: 增加开源协作章节**

新增“许可证、行为准则、贡献指南、安全策略、变更记录”入口链接。

**Step 2: 核对链接有效性**

确保所有链接目标文件存在且名称一致。

### Task 4: 验证与交付

**Files:**
- Verify: `LICENSE`
- Verify: `CONTRIBUTING.md`
- Verify: `CODE_OF_CONDUCT.md`
- Verify: `SECURITY.md`
- Verify: `CHANGELOG.md`
- Verify: `.github/workflows/ci.yml`
- Verify: `.github/ISSUE_TEMPLATE/bug_report.yml`
- Verify: `.github/ISSUE_TEMPLATE/feature_request.yml`
- Verify: `.github/pull_request_template.md`
- Verify: `README.md`

**Step 1: 运行项目测试**

Run: `go test ./...`
Expected: PASS

**Step 2: 结构检查**

Run: `git status --short`
Expected: 仅出现预期新增/修改文件

**Step 3: 最终审阅**

确认新增内容都与“开源准备”目标直接相关，无无意义改动。
