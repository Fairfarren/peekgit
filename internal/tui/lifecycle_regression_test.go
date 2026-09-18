package tui

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/Fairfarren/peekgit/internal/config"
	"github.com/Fairfarren/peekgit/internal/model"
	ghprovider "github.com/Fairfarren/peekgit/internal/provider/github"
	"github.com/Fairfarren/peekgit/internal/workspace"
	tea "github.com/charmbracelet/bubbletea"
)

func stubLifecycle(t *testing.T) {
	originalTick := scheduleTick
	originalScan := scanWorkspaceReposFn
	scheduleTick = func(duration time.Duration, fn func(time.Time) tea.Msg) tea.Cmd {
		return func() tea.Msg { return fn(time.Unix(0, 0).Add(duration)) }
	}
	scanWorkspaceReposFn = func(config.Config, []string) ([]workspace.RepoDir, error) { return nil, nil }
	t.Cleanup(func() { scheduleTick = originalTick; scanWorkspaceReposFn = originalScan })
}

func commandMessages(cmd tea.Cmd) []string {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var messages []string
		for _, child := range batch {
			messages = append(messages, commandMessages(child)...)
		}
		return messages
	}
	if tick, ok := msg.(tickMsg); ok {
		return []string{fmt.Sprintf("刷新间隔:%s", time.Time(tick).Sub(time.Unix(0, 0)))}
	}
	if tick, ok := msg.(configWatchTickMsg); ok {
		return []string{fmt.Sprintf("配置间隔:%s", time.Time(tick).Sub(time.Unix(0, 0)))}
	}
	return []string{fmt.Sprintf("%T", msg)}
}

func Test_初始化_按配置安排定时与工作区刷新(t *testing.T) {
	for _, tc := range []struct {
		name            string
		mode, workspace bool
		want            []string
	}{
		{"空配置", false, false, []string{"刷新间隔:5m0s", "spinner.TickMsg", "配置间隔:2s"}},
		{"常规工作区", false, true, []string{"刷新间隔:5m0s", "spinner.TickMsg", "配置间隔:2s", "tui.workspaceCheckDoneMsg"}},
		{"扫描工作区", true, true, []string{"刷新间隔:5m0s", "spinner.TickMsg", "tui.workspaceCheckDoneMsg", "tui.refreshDoneMsg"}},
		{"扫描空配置", true, false, []string{"刷新间隔:5m0s", "spinner.TickMsg"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubLifecycle(t)
			a := New(config.Config{NoGitHub: true, IntervalSec: 300, Concurrency: 1, WorkspaceMode: tc.mode})
			if tc.workspace {
				a.workspaces = []string{"项目"}
			}

			got := commandMessages(a.Init())

			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("初始化消息 = %v，期望 %v", got, tc.want)
			}
		})
	}
}

func Test_定时刷新_只在仓库页空闲时重载(t *testing.T) {
	for _, tc := range []struct {
		name               string
		screen             screen
		loading, workspace bool
		want               []string
	}{
		{"工作区空闲", screenWorkspaces, false, false, []string{"刷新间隔:5m0s"}},
		{"仓库空闲", screenHome, false, true, []string{"刷新间隔:5m0s", "tui.workspaceCheckDoneMsg", "tui.refreshDoneMsg"}},
		{"仓库加载中", screenHome, true, true, []string{"刷新间隔:5m0s", "tui.workspaceCheckDoneMsg"}},
		{"详情页", screenDetail, false, true, []string{"刷新间隔:5m0s", "tui.workspaceCheckDoneMsg"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubLifecycle(t)
			a := New(config.Config{NoGitHub: true, IntervalSec: 300, Concurrency: 1})
			a.screen = tc.screen
			a.loading = tc.loading
			if tc.workspace {
				a.workspaces = []string{"项目"}
			}

			_, cmd := a.Update(tickMsg(time.Time{}))
			got := commandMessages(cmd)

			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("定时消息 = %v，期望 %v", got, tc.want)
			}
		})
	}
}

func Test_扫描模式初始化_保留扫描计数与安全并发数(t *testing.T) {
	stubLifecycle(t)
	scanWorkspaceReposFn = func(config.Config, []string) ([]workspace.RepoDir, error) {
		return []workspace.RepoDir{{Name: "a"}, {Name: "b"}}, nil
	}

	a := New(config.Config{NoGitHub: true, WorkspaceMode: true, Global: config.GlobalConfig{Workspaces: config.WorkspaceMap{"项目": nil}}})

	if a.workspaceCounts["项目"] != 2 || cap(a.refreshLimiter) != 1 || a.matchIdx != -1 {
		t.Fatalf("计数 = %v，并发 = %d，匹配索引 = %d", a.workspaceCounts, cap(a.refreshLimiter), a.matchIdx)
	}
}

func Test_配置变化_更新工作区并按当前页面加载(t *testing.T) {
	for _, page := range []screen{screenWorkspaces, screenHome} {
		t.Run(fmt.Sprint(page), func(t *testing.T) {
			stubLifecycle(t)
			a := New(config.Config{NoGitHub: true, IntervalSec: 300})
			a.screen = page
			global := config.GlobalConfig{Workspaces: config.WorkspaceMap{"新项目": nil}}
			want := []string{"tui.workspaceCheckDoneMsg"}
			if page == screenHome {
				want = append(want, "tui.refreshDoneMsg")
			}

			_, cmd := a.Update(configReloadedMsg{global: global})
			got := commandMessages(cmd)

			if !reflect.DeepEqual(got, want) {
				t.Fatalf("配置变化消息 = %v，期望 %v", got, want)
			}
		})
	}
}

func Test_空刷新结果_结束加载并清除选择(t *testing.T) {
	a := fixtureApp()
	a.loading = true
	a.selectedIndex = 9

	a.Update(refreshDoneMsg{seq: a.refreshSeq})

	if a.loading || a.selectedIndex != 0 || len(a.repos) != 0 {
		t.Fatalf("加载 = %v，选择 = %d，仓库数 = %d", a.loading, a.selectedIndex, len(a.repos))
	}
}

func Test_重复完成消息_待刷新数不会变负(t *testing.T) {
	a := fixtureApp()
	a.repoRefreshing["a"] = true
	a.repoRefreshPending = 0

	a.Update(repoRefreshDoneMsg{seq: a.refreshSeq, status: model.RepoStatus{Path: "a"}})

	if a.repoRefreshPending != 0 {
		t.Fatalf("待刷新数 = %d", a.repoRefreshPending)
	}
}

func Test_新刷新_递增序号且忽略旧结果(t *testing.T) {
	stubLifecycle(t)
	a := fixtureApp()
	a.gh = ghprovider.New(nil, true)
	a.screen = screenHome
	a.refreshSeq = 7

	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	msg := cmd().(refreshDoneMsg)

	if msg.seq != 8 {
		t.Fatalf("刷新序号 = %d", msg.seq)
	}
}

func Test_扫描模式无工作区_初始化不发起仓库刷新(t *testing.T) {
	stubLifecycle(t)
	a := New(config.Config{NoGitHub: true, WorkspaceMode: true})

	a.Init()

	if a.refreshSeq != 0 {
		t.Fatalf("无工作区时出现刷新序号 %d", a.refreshSeq)
	}
}

func Test_无工作区按回车_保持工作区页面(t *testing.T) {
	a := New(config.Config{NoGitHub: true})

	a.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if a.screen != screenWorkspaces {
		t.Fatalf("页面 = %v", a.screen)
	}
}

func Test_空输入退格_不改变输入(t *testing.T) {
	for _, search := range []bool{false, true} {
		t.Run(fmt.Sprint(search), func(t *testing.T) {
			a := fixtureApp()
			a.searchMode = search
			a.filterMode = !search

			a.Update(tea.KeyMsg{Type: tea.KeyBackspace})

			if a.searchInput != "" || a.filterText != "" {
				t.Fatalf("搜索 = %q，过滤 = %q", a.searchInput, a.filterText)
			}
		})
	}
}

func Test_无失败拉取结果_清除旧错误(t *testing.T) {
	a := fixtureApp()
	a.errText = "旧错误"

	a.Update(pullAllDoneMsg{completed: 1, failed: 0, lastErr: fmt.Errorf("旧失败")})

	if a.errText != "" {
		t.Fatalf("仍有错误提示 %q", a.errText)
	}
}

func Test_刷新账号_提示持续两秒(t *testing.T) {
	a := fixtureApp()
	a.startTab = startTabPR
	original := currentTime
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	currentTime = func() time.Time { return now }
	t.Cleanup(func() { currentTime = original })

	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	if a.startRefreshNoticeUntil != now.Add(2*time.Second) {
		t.Fatalf("提示截止时间 = %v", a.startRefreshNoticeUntil)
	}
}

func Test_配置重排_保留原首个工作区选择(t *testing.T) {
	stubLifecycle(t)
	a := New(config.Config{NoGitHub: true})
	a.workspaces = []string{"zeta"}
	a.selectedWsIndex = 0

	a.Update(configReloadedMsg{global: config.GlobalConfig{Workspaces: config.WorkspaceMap{"alpha": nil, "zeta": nil}}})

	if a.selectedWsIndex != 1 {
		t.Fatalf("重排后选择 = %d", a.selectedWsIndex)
	}
}
