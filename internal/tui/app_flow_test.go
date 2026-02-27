package tui

import (
	"strings"
	"testing"

	"github.com/Fairfarren/peekgit/internal/config"
	"github.com/Fairfarren/peekgit/internal/model"
	"github.com/Fairfarren/peekgit/internal/workspace"
	tea "github.com/charmbracelet/bubbletea"
)

func newTestApp() *App {
	a := New(config.Config{Global: config.GlobalConfig{Workspaces: map[string][]string{"default": {"/tmp"}}}, IntervalSec: 300, Concurrency: 1, NoGitHub: true})
	a.width = 120
	a.height = 40
	a.repos = []model.RepoStatus{
		{Name: "repo-a", Path: "/tmp/repo-a", Branch: "main", Sync: model.SyncSynced},
		{Name: "repo-b", Path: "/tmp/repo-b", Branch: "dev", Sync: model.SyncAhead, Ahead: 2},
		{Name: "repo-c", Path: "/tmp/repo-c", Branch: "feat", Sync: model.SyncBehind, Behind: 1},
	}
	a.recomputeGrid()
	return a
}

func TestUpdateWindowSize(t *testing.T) {
	a := newTestApp()
	_, _ = a.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	if a.width != 80 || a.height != 20 {
		t.Fatalf("size not updated")
	}
}

func TestUpdateHomeNavigation(t *testing.T) {
	a := newTestApp()
	_, _ = a.updateHome(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if a.selectedIndex == 0 {
		t.Fatalf("expected moved")
	}
	_, _ = a.updateHome(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if a.selectedIndex != 0 {
		t.Fatalf("expected back to zero")
	}
}

func TestUpdateHomeEnterToDetail(t *testing.T) {
	a := newTestApp()
	_, cmd := a.updateHome(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if a.screen != screenDetail {
		t.Fatalf("expected detail screen")
	}
	if cmd == nil {
		t.Fatalf("expected load command")
	}
}

func TestUpdateHomeEmptyListBackToWorkspaces(t *testing.T) {
	a := newTestApp()
	a.screen = screenHome
	a.repos = nil
	a.filterMode = true
	a.filterText = "repo"

	_, _ = a.updateHome(tea.KeyMsg{Type: tea.KeyEsc})
	if a.screen != screenWorkspaces {
		t.Fatalf("expected workspaces screen")
	}
	if a.filterMode {
		t.Fatalf("expected filter mode off")
	}
	if a.filterText != "" {
		t.Fatalf("expected empty filter text")
	}

	a.screen = screenHome
	a.filterMode = true
	a.filterText = "repo"
	_, _ = a.updateHome(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if a.screen != screenWorkspaces {
		t.Fatalf("expected workspaces screen on q")
	}
}

func TestUpdateFilterInput(t *testing.T) {
	a := newTestApp()
	a.filterMode = true
	_, _ = a.updateFilterInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	_, _ = a.updateFilterInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if a.filterText != "re" {
		t.Fatalf("filter=%s", a.filterText)
	}
	_, _ = a.updateFilterInput(tea.KeyMsg{Type: tea.KeyBackspace})
	if a.filterText != "r" {
		t.Fatalf("filter=%s", a.filterText)
	}
	_, _ = a.updateFilterInput(tea.KeyMsg{Type: tea.KeyEsc})
	if a.filterMode {
		t.Fatalf("expected filter end")
	}
}

func TestUpdateDetailTabSwitch(t *testing.T) {
	a := newTestApp()
	a.screen = screenDetail
	a.detailTab = tabPR
	_, _ = a.updateDetail(tea.KeyMsg{Type: tea.KeyTab})
	if a.detailTab != tabIssue {
		t.Fatalf("tab=%v", a.detailTab)
	}
	_, _ = a.updateDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	if a.detailTab != tabPR {
		t.Fatalf("tab=%v", a.detailTab)
	}
}

func TestUpdateDetailBackHome(t *testing.T) {
	a := newTestApp()
	a.screen = screenDetail
	_, _ = a.updateDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if a.screen != screenHome {
		t.Fatalf("expected home")
	}
}

func TestViewsNotEmpty(t *testing.T) {
	a := newTestApp()
	if a.viewHome() == "" {
		t.Fatalf("home empty")
	}
	a.screen = screenDetail
	if a.viewDetail() == "" {
		t.Fatalf("detail empty")
	}
	a.screen = screenDiff
	a.diffViewport.SetContent("diff")
	if a.viewDiff() == "" {
		t.Fatalf("diff empty")
	}
}

func TestViewHomeMultiColumnCardsStayInSameRow(t *testing.T) {
	a := newTestApp()
	a.width = 120
	a.recomputeGrid()

	view := a.viewHome()
	lines := strings.Split(view, "\n")

	hasSameLine := false
	for _, line := range lines {
		if strings.Contains(line, "repo-a") && strings.Contains(line, "repo-b") {
			hasSameLine = true
			break
		}
	}

	if !hasSameLine {
		t.Fatalf("expected repo-a and repo-b to render in the same visual row, view:\n%s", view)
	}
}

func TestUpdateDetailPRAndIssueCursorMove(t *testing.T) {
	a := newTestApp()
	a.screen = screenDetail
	a.prList = []model.PullRequestItem{{Number: 1}, {Number: 2}}
	a.issues = []model.IssueItem{{Number: 1}, {Number: 2}}

	a.detailTab = tabPR
	_, _ = a.updateDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if a.detailPRIdx != 1 {
		t.Fatalf("pr idx=%d", a.detailPRIdx)
	}

	a.detailTab = tabIssue
	_, _ = a.updateDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if a.detailISIdx != 1 {
		t.Fatalf("issue idx=%d", a.detailISIdx)
	}
}

func TestUpdateDiffQBackToDetail(t *testing.T) {
	a := newTestApp()
	a.screen = screenDiff
	_, _ = a.updateDiff(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if a.screen != screenDetail {
		t.Fatalf("expected detail")
	}
}

func TestRemoteLoadedMsgUpdatesRepoCounters(t *testing.T) {
	a := newTestApp()
	a.selectedIndex = 0

	prCount := 2
	issueCount := 3
	_, _ = a.Update(remoteLoadedMsg{
		repoPath:  "/tmp/repo-a",
		prs:       []model.PullRequestItem{{Number: 1}, {Number: 2}},
		issues:    []model.IssueItem{{Number: 10}, {Number: 11}, {Number: 12}},
		prOpen:    &prCount,
		issueOpen: &issueCount,
	})

	if a.repos[0].PROpen == nil || *a.repos[0].PROpen != 2 {
		t.Fatalf("pr open not updated, got=%v", a.repos[0].PROpen)
	}
	if a.repos[0].IssueOpen == nil || *a.repos[0].IssueOpen != 3 {
		t.Fatalf("issue open not updated, got=%v", a.repos[0].IssueOpen)
	}
}

func TestViewHomeLoadingKeepsExistingCardsVisible(t *testing.T) {
	a := newTestApp()
	a.loading = true
	view := a.viewHome()
	if !strings.Contains(view, "刷新中...") {
		t.Fatalf("expected loading text")
	}
	if !strings.Contains(view, "repo-a") {
		t.Fatalf("expected existing cards to remain visible when loading")
	}
}

func TestRepoRefreshDoneMsgUpdatesOnlyTargetRepo(t *testing.T) {
	a := newTestApp()
	a.refreshSeq = 3
	a.repoRefreshing["/tmp/repo-a"] = true
	a.repoRefreshing["/tmp/repo-b"] = true
	a.repoRefreshPending = 2
	a.loading = true

	_, _ = a.Update(repoRefreshDoneMsg{
		seq: 3,
		status: model.RepoStatus{
			Name:   "repo-a",
			Path:   "/tmp/repo-a",
			Branch: "main",
			Sync:   model.SyncBehind,
			Behind: 2,
		},
	})

	if a.repos[0].Sync != model.SyncBehind || a.repos[0].Behind != 2 {
		t.Fatalf("expected repo-a to be updated independently")
	}
	if a.repos[1].Sync != model.SyncAhead || a.repos[1].Ahead != 2 {
		t.Fatalf("expected repo-b to keep previous state")
	}
	if a.repoRefreshing["/tmp/repo-a"] {
		t.Fatalf("expected repo-a refreshing state to be false")
	}
	if !a.repoRefreshing["/tmp/repo-b"] {
		t.Fatalf("expected repo-b refreshing state to stay true")
	}
	if !a.loading {
		t.Fatalf("expected loading to stay true while pending repos exist")
	}
}

func TestViewDiffKeepsFooterOnTinyHeight(t *testing.T) {
	a := newTestApp()
	a.screen = screenDiff
	a.height = 6
	a.diffContent = strings.Repeat("line\n", 20)
	a.diffViewport.SetContent(a.diffContent)

	view := a.viewDiff()
	lines := strings.Split(view, "\n")

	if len(lines) > a.height {
		t.Fatalf("expected rendered lines <= height, got %d > %d", len(lines), a.height)
	}
	if !strings.Contains(lines[len(lines)-1], "[q] back") {
		t.Fatalf("expected footer help on last line")
	}
}

func TestViewHomeTinyHeightHidesCards(t *testing.T) {
	a := newTestApp()
	a.height = 3

	view := a.viewHome()
	lines := strings.Split(view, "\n")

	if len(lines) > a.height {
		t.Fatalf("expected rendered lines <= height, got %d > %d", len(lines), a.height)
	}
	if strings.Contains(view, "repo-a") {
		t.Fatalf("expected cards to be hidden on tiny height")
	}
	if !strings.Contains(lines[len(lines)-1], "q/ESC") {
		t.Fatalf("expected footer help on last line")
	}
}

func TestViewWorkspacesTinyHeightHidesCards(t *testing.T) {
	a := newTestApp()
	a.height = 4

	view := a.viewWorkspaces()
	lines := strings.Split(view, "\n")

	if len(lines) > a.height {
		t.Fatalf("expected rendered lines <= height, got %d > %d", len(lines), a.height)
	}
	if strings.Contains(view, "default") {
		t.Fatalf("expected workspace cards to be hidden on tiny height")
	}
	if !strings.Contains(lines[len(lines)-1], "q 退出") {
		t.Fatalf("expected footer help on last line")
	}
}

func TestViewDiffUltraTinyHeightKeepsFooter(t *testing.T) {
	a := newTestApp()
	a.screen = screenDiff
	a.diffContent = strings.Repeat("line\n", 20)
	a.diffViewport.SetContent(a.diffContent)

	for _, h := range []int{1, 2} {
		a.height = h
		view := a.viewDiff()
		lines := strings.Split(view, "\n")

		if len(lines) > h {
			t.Fatalf("height=%d expected rendered lines <= height, got %d", h, len(lines))
		}
		if !strings.Contains(lines[len(lines)-1], "[q] back") {
			t.Fatalf("height=%d expected footer help on last line", h)
		}
	}
}

func TestCalculateScrollWindowBoundaries(t *testing.T) {
	start, end := calculateScrollWindow(10, 0, 3)
	if start != 0 || end != 3 {
		t.Fatalf("first-item boundary failed: got (%d,%d)", start, end)
	}

	start, end = calculateScrollWindow(10, 5, 3)
	if start != 4 || end != 7 {
		t.Fatalf("center window failed: got (%d,%d)", start, end)
	}

	start, end = calculateScrollWindow(10, 9, 3)
	if start != 7 || end != 10 {
		t.Fatalf("last-item boundary failed: got (%d,%d)", start, end)
	}
}

func TestViewDetailTinyHeightKeepsFooter(t *testing.T) {
	a := newTestApp()
	a.screen = screenDetail
	a.detailTab = tabPR
	a.height = 2
	a.prList = []model.PullRequestItem{{Number: 1, Title: "pr-1", Author: "u"}}

	view := a.viewDetail()
	lines := strings.Split(view, "\n")

	if len(lines) > a.height {
		t.Fatalf("expected rendered lines <= height, got %d > %d", len(lines), a.height)
	}
	if !strings.Contains(lines[len(lines)-1], "d: diff") {
		t.Fatalf("expected detail footer help on last line")
	}
}

func TestViewHomeAndWorkspacesSafeWhenColumnsZero(t *testing.T) {
	a := newTestApp()
	a.columns = 0

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("expected no panic when columns is zero: %v", r)
		}
	}()

	_ = a.viewHome()
	_ = a.viewWorkspaces()
}

func TestWorkspaceCheckDoneMsgOnlyUpdatesTargetWorkspace(t *testing.T) {
	a := New(config.Config{
		Global:      config.GlobalConfig{Workspaces: map[string][]string{"ws-a": {"/tmp"}, "ws-b": {"/tmp"}}},
		IntervalSec: 300,
		Concurrency: 1,
		NoGitHub:    true,
	})

	a.workspaceHasUpdate["ws-a"] = false
	a.workspaceHasUpdate["ws-b"] = true
	a.workspaceChecking["ws-a"] = true
	a.workspaceChecking["ws-b"] = true

	_, _ = a.Update(workspaceCheckDoneMsg{workspace: "ws-a", hasUpdate: true})

	if !a.workspaceHasUpdate["ws-a"] {
		t.Fatalf("expected ws-a update flag to be true")
	}
	if !a.workspaceHasUpdate["ws-b"] {
		t.Fatalf("expected ws-b update flag to remain true")
	}
	if a.workspaceChecking["ws-a"] {
		t.Fatalf("expected ws-a checking to be false after completion")
	}
	if !a.workspaceChecking["ws-b"] {
		t.Fatalf("expected ws-b checking state to remain unchanged")
	}
}

func TestWorkspaceCheckCmdStartsOnlyIdleWorkspaceChecks(t *testing.T) {
	a := New(config.Config{
		Global:      config.GlobalConfig{Workspaces: map[string][]string{"ws-a": {"/tmp"}, "ws-b": {"/tmp"}}},
		IntervalSec: 300,
		Concurrency: 1,
		NoGitHub:    true,
	})

	a.workspaceChecking["ws-a"] = true
	a.workspaceChecking["ws-b"] = false

	cmd := a.workspaceCheckCmd()
	if cmd == nil {
		t.Fatalf("expected command to start checks for idle workspaces")
	}
	if !a.workspaceChecking["ws-a"] {
		t.Fatalf("expected ws-a checking state to remain true")
	}
	if !a.workspaceChecking["ws-b"] {
		t.Fatalf("expected ws-b checking state to become true")
	}
}

func TestRefreshAllCmdClearsReposWhenWorkspaceSwitched(t *testing.T) {
	a := New(config.Config{
		Global: config.GlobalConfig{Workspaces: map[string][]string{
			"ws-a": {"/tmp"},
			"ws-b": {"/tmp"},
		}},
		IntervalSec: 300,
		Concurrency: 1,
		NoGitHub:    true,
	})

	a.reposWorkspace = "ws-a"
	a.repos = []model.RepoStatus{{Name: "old", Path: "/tmp/old", Sync: model.SyncSynced}}
	a.selectedWsIndex = 1

	cmd := a.refreshAllCmd()

	if cmd == nil {
		t.Fatalf("expected refresh command")
	}
	if len(a.repos) != 0 {
		t.Fatalf("expected stale repos to be cleared when switching workspace")
	}
}

func TestRefreshDoneMsgDeduplicatesReposByPath(t *testing.T) {
	a := newTestApp()
	a.refreshSeq = 5
	a.repos = nil

	_, cmd := a.Update(refreshDoneMsg{
		seq: 5,
		repos: []workspace.RepoDir{
			{Name: "repo-1", Path: "/tmp/repo-1"},
			{Name: "repo-1-dup", Path: "/tmp/repo-1"},
		},
	})

	if cmd == nil {
		t.Fatalf("expected follow-up refresh commands")
	}
	if len(a.repos) != 1 {
		t.Fatalf("expected duplicate paths to be deduplicated, got %d", len(a.repos))
	}
	if a.repoRefreshPending != 1 {
		t.Fatalf("expected pending count to match deduplicated repos, got %d", a.repoRefreshPending)
	}
}

func TestConfigReloadedMsgAppliesConfigAndKeepsSelectedWorkspace(t *testing.T) {
	a := New(config.Config{
		Global: config.GlobalConfig{Workspaces: map[string][]string{
			"ws-a": {"/tmp"},
			"ws-b": {"/tmp"},
		}},
		IntervalSec: 300,
		Concurrency: 1,
		NoGitHub:    true,
	})
	a.screen = screenWorkspaces
	a.selectedWsIndex = 1 // ws-b

	_, cmd := a.Update(configReloadedMsg{
		global: config.GlobalConfig{Workspaces: map[string][]string{
			"ws-b": {"/tmp"},
			"ws-c": {"/tmp"},
		}},
	})

	if cmd == nil {
		t.Fatalf("expected workspace check cmd after config reload")
	}
	if len(a.workspaces) != 2 || a.workspaces[0] != "ws-b" || a.workspaces[1] != "ws-c" {
		t.Fatalf("unexpected workspace list: %+v", a.workspaces)
	}
	if a.selectedWsIndex != 0 {
		t.Fatalf("expected selection to stay on ws-b, got index=%d", a.selectedWsIndex)
	}
}

func TestConfigReloadedMsgUnchangedDoesNotTriggerRefresh(t *testing.T) {
	a := New(config.Config{
		Global: config.GlobalConfig{Workspaces: map[string][]string{
			"default": {"/tmp"},
		}},
		IntervalSec: 300,
		Concurrency: 1,
		NoGitHub:    true,
	})
	a.screen = screenHome

	_, cmd := a.Update(configReloadedMsg{
		global: config.GlobalConfig{Workspaces: map[string][]string{
			"default": {"/tmp"},
		}},
	})

	if cmd != nil {
		t.Fatalf("expected nil cmd when config unchanged")
	}
}
