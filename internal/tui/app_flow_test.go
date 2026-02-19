package tui

import (
	"strings"
	"testing"

	"github.com/Fairfarren/peekgit/internal/config"
	"github.com/Fairfarren/peekgit/internal/model"
	tea "github.com/charmbracelet/bubbletea"
)

func newTestApp() *App {
	a := New(config.Config{Workspace: "/tmp", IntervalSec: 300, Concurrency: 1, NoGitHub: true})
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
	_, _ = a.updateDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	if a.detailTab != tabBranch {
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

func TestUpdateDiffSearchControl(t *testing.T) {
	a := newTestApp()
	a.screen = screenDiff
	a.diffContent = "a\nmatch\nb"
	a.diffViewport.SetContent(a.diffContent)
	_, _ = a.updateDiff(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !a.searchMode {
		t.Fatalf("expected search mode")
	}
	a.searchInput = "match"
	_, _ = a.updateSearchInput(tea.KeyMsg{Type: tea.KeyEnter})
	if a.searchMode {
		t.Fatalf("search mode should close")
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

func TestUpdateDiffEscBackToDetail(t *testing.T) {
	a := newTestApp()
	a.screen = screenDiff
	_, _ = a.updateDiff(tea.KeyMsg{Type: tea.KeyEsc})
	if a.screen != screenDetail {
		t.Fatalf("expected detail")
	}
}

func TestJumpMatchNoMatchSafe(t *testing.T) {
	a := newTestApp()
	a.matches = nil
	a.matchIdx = -1
	a.jumpMatch(1)
	if a.matchIdx != -1 {
		t.Fatalf("matchIdx=%d", a.matchIdx)
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
