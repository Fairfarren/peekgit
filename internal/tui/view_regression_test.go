package tui

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Fairfarren/peekgit/internal/config"
	"github.com/Fairfarren/peekgit/internal/model"
	ghprovider "github.com/Fairfarren/peekgit/internal/provider/github"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	gh "github.com/google/go-github/v57/github"
	"github.com/muesli/termenv"
)

// 编译时嵌入已核对的终端输出，单测不访问真实文件系统。
//
//go:embed testdata/views/*.golden
var viewFixtures embed.FS

func fixtureApp() *App {
	a := New(config.Config{NoGitHub: true, IntervalSec: 300, Concurrency: 1})
	a.width = 120
	a.height = 40
	a.gh = ghprovider.NewWithClient(gh.NewClient(nil))
	a.workspaces = []string{"alpha", "beta", "delta", "gamma", "omega", "project", "tools", "web", "worker", "zeta"}
	a.cfg.Global.Workspaces = config.WorkspaceMap{}
	for i, name := range a.workspaces {
		a.workspaceCounts[name] = i
		a.repos = append(a.repos, model.RepoStatus{Name: name, Path: "/projects/" + name, Branch: "main", Sync: model.SyncSynced})
		a.startPRs = append(a.startPRs, model.AccountPullRequestItem{Number: 10000 + i, Title: "修复窗口布局与键盘导航", RepoFull: "example/" + name, StateLabel: "OPEN", CIStatus: "SUCCESS"})
		a.startIssues = append(a.startIssues, model.AccountIssueItem{Number: 10000 + i, Title: "检查边界与加载状态", RepoFull: "example/" + name, Labels: []string{"bug", "ui"}})
		a.prList = append(a.prList, model.PullRequestItem{Number: 10000 + i, Title: "修复窗口布局与键盘导航", Author: "developer", HeadBranch: "feature", BaseBranch: "main"})
		a.issues = append(a.issues, model.IssueItem{Number: 10000 + i, Title: "检查边界与加载状态", Labels: []string{"bug", "ui"}})
	}
	a.startPRs[0].StateLabel = ""
	a.startIssues[0].Labels = nil
	a.issues[0].Labels = nil
	a.workspaceHasUpdate["beta"] = true
	a.workspaceChecking["delta"] = true
	a.repos[1].Dirty = true
	a.repos[2].Sync = model.SyncAhead
	a.repos[2].Ahead = 3
	files := []FileDiff{
		{Path: "a.go", Content: "第一行\n第二行\n第三行", IsNew: true, AddLines: 2},
		{Path: "empty.go", Content: "空白变更"},
		{Path: "internal/very-long-directory-name/nested/very-long-source-file-name.go", Content: strings.Repeat("上下文\n", 30), AddLines: 12, DelLines: 4},
		{Path: "internal/very-long-directory-name/nested/z.go", Content: "删除", IsDelete: true, DelLines: 5},
		{Path: "internal/z.go", Content: "普通修改", AddLines: 1},
		{Path: "z.go", Content: "最后一项", IsBinary: true},
	}
	a.diffTree = BuildDiffTree(files)
	a.diffContent = "第一行\n第二行\n第三行\n第四行\n第五行\n第六行\n第七行\n第八行\n第九行\n第十行"
	a.diffViewport.SetContent(a.diffContent)
	return a
}

func Test_页面输出_保持布局选择与滚动内容(t *testing.T) {
	previous := lipgloss.ColorProfile()
	dark := lipgloss.HasDarkBackground()
	lipgloss.SetColorProfile(termenv.TrueColor)
	lipgloss.SetHasDarkBackground(true)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous); lipgloss.SetHasDarkBackground(dark) })
	screens := []struct {
		name   string
		screen screen
		start  startTab
		detail tab
	}{
		{"工作区", screenWorkspaces, startTabWorkspace, tabPR},
		{"账号PR", screenWorkspaces, startTabPR, tabPR},
		{"账号Issue", screenWorkspaces, startTabIssue, tabPR},
		{"仓库", screenHome, startTabWorkspace, tabPR},
		{"仓库PR", screenDetail, startTabWorkspace, tabPR},
		{"仓库Issue", screenDetail, startTabWorkspace, tabIssue},
		{"差异", screenDiff, startTabWorkspace, tabPR},
	}
	sizes := []struct{ width, height, index int }{
		{40, 4, 0}, {68, 9, 4}, {69, 10, 4}, {79, 12, 0}, {80, 18, 9}, {119, 13, 4}, {120, 18, 0}, {199, 24, 4}, {200, 18, 9},
	}
	for _, screenCase := range screens {
		for _, size := range sizes {
			name := fmt.Sprintf("%s_%dx%d_%d", screenCase.name, size.width, size.height, size.index)
			t.Run(name, func(t *testing.T) {
				a := fixtureApp()
				a.screen = screenCase.screen
				a.startTab = screenCase.start
				a.detailTab = screenCase.detail
				a.selectedIndex = size.index
				a.selectedWsIndex = size.index
				a.startPRIdx = size.index
				a.startIssueIdx = size.index
				a.detailPRIdx = size.index
				a.detailISIdx = size.index
				a.diffFileIdx = size.index % len(a.diffTree.Files)
				a.diffFocusLeft = size.index == 0
				a.Update(tea.WindowSizeMsg{Width: size.width, Height: size.height})

				got := a.View()

				path := "testdata/views/" + name + ".golden"
				want, err := viewFixtures.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var lines []string
				if err := json.Unmarshal(want, &lines); err != nil {
					t.Fatal(err)
				}
				expected := strings.Join(lines, "\n")
				if got != expected {
					t.Fatalf("终端输出发生变化：\n实际：%q\n期望：%q", got, expected)
				}
			})
		}
	}
}
