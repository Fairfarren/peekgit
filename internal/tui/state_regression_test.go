package tui

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Fairfarren/peekgit/internal/gitcli"
	"github.com/Fairfarren/peekgit/internal/model"
	"github.com/Fairfarren/peekgit/internal/workspace"
	tea "github.com/charmbracelet/bubbletea"
)

func Test_列表导航_首尾停止且中间移动(t *testing.T) {
	for _, page := range []struct {
		name   string
		screen screen
		start  startTab
		detail tab
	}{
		{"账号PR", screenWorkspaces, startTabPR, tabPR}, {"账号Issue", screenWorkspaces, startTabIssue, tabIssue},
		{"详情PR", screenDetail, startTabWorkspace, tabPR}, {"详情Issue", screenDetail, startTabWorkspace, tabIssue},
	} {
		for _, tc := range []struct {
			name string
			from int
			key  tea.KeyType
			want int
		}{{"首项向上", 0, tea.KeyUp, 0}, {"次项向上", 1, tea.KeyUp, 0}, {"中间向下", 4, tea.KeyDown, 5}, {"末项向下", 9, tea.KeyDown, 9}} {
			t.Run(page.name+tc.name, func(t *testing.T) {
				a := fixtureApp()
				a.screen = page.screen
				a.startTab = page.start
				a.detailTab = page.detail
				a.startPRIdx = tc.from
				a.startIssueIdx = tc.from
				a.detailPRIdx = tc.from
				a.detailISIdx = tc.from

				a.Update(tea.KeyMsg{Type: tc.key})

				got := a.startPRIdx
				if page.screen == screenDetail {
					if page.detail == tabPR {
						got = a.detailPRIdx
					} else {
						got = a.detailISIdx
					}
				} else if page.start == startTabIssue {
					got = a.startIssueIdx
				}
				if got != tc.want {
					t.Fatalf("索引 = %d，期望 %d", got, tc.want)
				}
			})
		}
	}
}

func Test_刷新完成_计数归零且重复结果不重复扣减(t *testing.T) {
	cases := []struct {
		name     string
		messages []repoRefreshDoneMsg
		pending  int
		loading  bool
	}{
		{"过期结果", []repoRefreshDoneMsg{{seq: 6, status: model.RepoStatus{Path: "a"}}}, 2, true},
		{"首个结果", []repoRefreshDoneMsg{{seq: 7, status: model.RepoStatus{Path: "a"}}}, 1, true},
		{"重复结果", []repoRefreshDoneMsg{{seq: 7, status: model.RepoStatus{Path: "a"}}, {seq: 7, status: model.RepoStatus{Path: "a"}}}, 1, true},
		{"最后结果", []repoRefreshDoneMsg{{seq: 7, status: model.RepoStatus{Path: "a"}}, {seq: 7, status: model.RepoStatus{Path: "z"}}}, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := fixtureApp()
			a.screen = screenHome
			a.selectedIndex = 50
			a.refreshSeq = 7
			a.Update(refreshDoneMsg{seq: 7, repos: []workspace.RepoDir{{Name: "zeta", Path: "z"}, {Name: "alpha", Path: "a"}}})

			for _, msg := range tc.messages {
				a.Update(msg)
			}

			if a.repoRefreshPending != tc.pending || a.loading != tc.loading {
				t.Fatalf("待刷新 = %d，加载状态 = %v", a.repoRefreshPending, a.loading)
			}
		})
	}
}

func Test_刷新列表_保留顺序与末项选择(t *testing.T) {
	a := fixtureApp()
	a.selectedIndex = 20
	a.refreshSeq = 3

	a.Update(refreshDoneMsg{seq: 3, repos: []workspace.RepoDir{{Name: "zeta", Path: "z"}, {Name: "alpha", Path: "a"}, {Name: "middle", Path: "m"}}})

	got := []string{a.repos[0].Name, a.repos[1].Name, a.repos[2].Name}
	if !reflect.DeepEqual(got, []string{"alpha", "middle", "zeta"}) || a.selectedIndex != 2 || !a.loading {
		t.Fatalf("顺序 = %v，索引 = %d，加载 = %v", got, a.selectedIndex, a.loading)
	}
}

func Test_账号加载完成_将选择限制在新列表内(t *testing.T) {
	a := fixtureApp()
	a.startPRIdx = 50
	a.startIssueIdx = 50

	a.Update(accountRemoteLoadedMsg{prs: a.startPRs[:2], items: a.startIssues[:3]})

	if a.startPRIdx != 1 || a.startIssueIdx != 2 {
		t.Fatalf("PR 索引 = %d，Issue 索引 = %d", a.startPRIdx, a.startIssueIdx)
	}
}

func Test_窗口改变_设置差异窗口尺寸(t *testing.T) {
	a := fixtureApp()

	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	if a.diffViewport.Width != 98 || a.diffViewport.Height != 26 {
		t.Fatalf("窗口 = %dx%d", a.diffViewport.Width, a.diffViewport.Height)
	}
}

func Test_打开差异_仅PR且列表非空时加载(t *testing.T) {
	for _, tc := range []struct {
		name   string
		screen screen
		start  startTab
		detail tab
		empty  bool
		want   bool
	}{
		{"账号PR", screenWorkspaces, startTabPR, tabPR, false, true},
		{"账号Issue", screenWorkspaces, startTabIssue, tabIssue, false, false},
		{"空账号PR", screenWorkspaces, startTabPR, tabPR, true, false},
		{"详情PR", screenDetail, startTabWorkspace, tabPR, false, true},
		{"详情Issue", screenDetail, startTabWorkspace, tabIssue, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := fixtureApp()
			a.screen = tc.screen
			a.startTab = tc.start
			a.detailTab = tc.detail
			a.width = 100
			a.height = 30
			if tc.empty {
				a.startPRs = nil
			}

			_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

			got := a.screen == screenDiff && a.diffLoading && cmd != nil && a.diffViewport.Width == 98 && a.diffViewport.Height == 26
			if got != tc.want {
				t.Fatalf("差异窗口是否加载 = %v，期望 %v", got, tc.want)
			}
		})
	}
}

func Test_打开链接_使用当前页签选中项(t *testing.T) {
	for _, tc := range []struct {
		name   string
		screen screen
		start  startTab
		detail tab
		empty  bool
		want   string
	}{
		{"账号PR", screenWorkspaces, startTabPR, tabPR, false, "https://example.com/account-pr"},
		{"账号Issue", screenWorkspaces, startTabIssue, tabIssue, false, "https://example.com/account-issue"},
		{"工作区", screenWorkspaces, startTabWorkspace, tabPR, false, ""},
		{"空账号PR", screenWorkspaces, startTabPR, tabPR, true, ""},
		{"空账号Issue", screenWorkspaces, startTabIssue, tabIssue, true, ""},
		{"详情PR", screenDetail, startTabWorkspace, tabPR, false, "https://example.com/detail-pr"},
		{"详情Issue", screenDetail, startTabWorkspace, tabIssue, false, "https://example.com/detail-issue"},
		{"空详情Issue", screenDetail, startTabWorkspace, tabIssue, true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := fixtureApp()
			a.screen = tc.screen
			a.startTab = tc.start
			a.detailTab = tc.detail
			a.startPRs[0].HTMLURL = "https://example.com/account-pr"
			a.startIssues[0].HTMLURL = "https://example.com/account-issue"
			a.prList[0].HTMLURL = "https://example.com/detail-pr"
			a.issues[0].HTMLURL = "https://example.com/detail-issue"
			if tc.empty {
				a.startPRs = nil
				a.startIssues = nil
				a.prList = nil
				a.issues = nil
			}
			targets := stubBrowserTargets(t)
			var want []string
			if tc.want != "" {
				want = []string{tc.want}
			}

			_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
			if cmd != nil {
				cmd()
			}

			if !reflect.DeepEqual(*targets, want) {
				t.Fatalf("打开目标 = %q，期望 %q", *targets, want)
			}
		})
	}
}

func Test_差异导航_窗口边界与文件首尾(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		width, height, index int
		key                  tea.KeyType
		want                 int
	}{
		{"首项向上", 100, 20, 0, tea.KeyUp, 0}, {"末项向下", 100, 20, 5, tea.KeyDown, 5},
		{"中间向下", 100, 20, 2, tea.KeyDown, 3}, {"次项向上", 100, 20, 1, tea.KeyUp, 0},
		{"最小双栏宽度", 69, 20, 0, tea.KeyDown, 1}, {"最小双栏高度", 100, 10, 0, tea.KeyDown, 1},
		{"窄窗口", 68, 20, 0, tea.KeyDown, 0}, {"矮窗口", 100, 9, 0, tea.KeyDown, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := fixtureApp()
			a.screen = screenDiff
			a.width = tc.width
			a.height = tc.height
			a.diffFileIdx = tc.index

			a.Update(tea.KeyMsg{Type: tc.key})

			if a.diffFileIdx != tc.want {
				t.Fatalf("文件索引 = %d，期望 %d", a.diffFileIdx, tc.want)
			}
		})
	}
}

func Test_差异搜索_匹配顺序与双向循环(t *testing.T) {
	a := fixtureApp()
	a.screen = screenDiff
	a.diffContent = "hit\n空\nhit\n空\nhit"
	a.diffViewport.SetContent(a.diffContent)
	a.diffViewport.Height = 1
	a.searchMode = true
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hit")})
	a.Update(tea.KeyMsg{Type: tea.KeyEnter})
	for _, tc := range []struct {
		key           rune
		index, offset int
	}{{'n', 1, 2}, {'n', 2, 4}, {'n', 0, 0}, {'N', 2, 4}} {
		if tc.key == 'n' {
			a.jumpMatch(1)
		} else {
			a.jumpMatch(-1)
		}

		if a.matchIdx != tc.index || a.diffViewport.YOffset != tc.offset {
			t.Fatalf("匹配索引 = %d，滚动行 = %d", a.matchIdx, a.diffViewport.YOffset)
		}
	}
}

func Test_空搜索_恢复无匹配索引(t *testing.T) {
	a := fixtureApp()
	a.screen = screenDiff
	a.matchIdx = 3
	a.searchMode = true

	a.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if a.matchIdx != -1 {
		t.Fatalf("匹配索引 = %d", a.matchIdx)
	}
}

func Test_筛选收缩_选择回到最后匹配仓库(t *testing.T) {
	a := fixtureApp()
	a.screen = screenHome
	a.selectedIndex = 9
	a.filterText = "ta"

	view := a.View()

	if a.selectedIndex != 2 || !strings.Contains(view, "zeta") {
		t.Fatalf("筛选后的选择 = %d", a.selectedIndex)
	}
}

func Test_错误仓库_差异命令返回错误(t *testing.T) {
	a := fixtureApp()
	a.screen = screenDetail
	a.git = gitcli.NewWithExecutor(mockGitExecForTest{fn: func(...string) (string, error) { return "", errors.New("远程地址错误") }})
	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	msg := cmd().(diffLoadedMsg)

	if msg.err == nil || msg.err.Error() != "远程地址错误" {
		t.Fatalf("差异加载错误 = %v", msg.err)
	}
}

func Test_差异渲染_双栏窗口内部尺寸(t *testing.T) {
	a := fixtureApp()
	a.screen = screenDiff
	a.width = 120
	a.height = 20

	a.View()

	if a.diffViewport.Width != 78 || a.diffViewport.Height != 16 {
		t.Fatalf("差异内部尺寸 = %dx%d", a.diffViewport.Width, a.diffViewport.Height)
	}
}

func Test_单行差异窗口_仅显示返回操作(t *testing.T) {
	a := fixtureApp()
	a.screen = screenDiff
	a.height = 1

	got := a.View()

	if got != "[q] 返回" {
		t.Fatalf("单行输出 = %q", got)
	}
}

func Test_简单差异窗口_保留正文与底部操作(t *testing.T) {
	a := fixtureApp()
	a.screen = screenDiff
	a.width = 60
	a.height = 6
	a.diffViewport.SetContent("第一行\n第二行\n第三行\n第四行\n第五行")

	got := a.View()

	if a.diffViewport.Height != 4 || !strings.Contains(got, "第四行") || strings.Contains(got, "第五行") {
		t.Fatalf("简单差异输出 = %q，高度 = %d", got, a.diffViewport.Height)
	}
}

func Test_无有效工作区选择_页头不显示名称(t *testing.T) {
	for _, index := range []int{-1, 0, 1} {
		t.Run(string(rune('0'+index)), func(t *testing.T) {
			a := fixtureApp()
			a.screen = screenHome
			a.workspaces = nil
			a.selectedWsIndex = index
			if index == 1 {
				a.workspaces = []string{"不应显示"}
				a.selectedWsIndex = 1
			}

			got := a.View()

			if strings.Contains(got, "[不应显示]") {
				t.Fatalf("无效选择显示了工作区：%q", got)
			}
		})
	}
}

func Test_刷新无效工作区选择_不生成命令(t *testing.T) {
	a := fixtureApp()
	a.screen = screenHome
	a.selectedWsIndex = len(a.workspaces)

	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	if cmd != nil {
		t.Fatal("越界的工作区选择不应生成刷新命令")
	}
}

func Test_差异路径_刚好容纳时保持完整名称(t *testing.T) {
	a := fixtureApp()
	a.screen = screenDiff
	a.width = 69
	a.height = 15
	path := "123456789012345678901234567890"
	a.diffTree = BuildDiffTree([]FileDiff{{Path: path, Content: "内容"}})

	got := a.View()

	if !strings.Contains(got, path) {
		t.Fatalf("完整路径被截断：%q", got)
	}
}

func Test_空差异树_显示无文件变更(t *testing.T) {
	a := fixtureApp()
	a.screen = screenDiff
	a.width = 100
	a.height = 20
	a.diffTree = &DiffTree{Tree: &DiffNode{IsDir: true}}

	got := a.View()

	if !strings.Contains(got, "无文件变更") {
		t.Fatalf("空差异提示缺失：%q", got)
	}
}

func Test_仓库选择越界_详情页安全显示(t *testing.T) {
	a := fixtureApp()
	a.screen = screenDetail
	a.selectedIndex = len(a.repos)

	got := a.View()

	if strings.Contains(got, "branch: main") {
		t.Fatalf("无效选择显示了仓库：%q", got)
	}
}
