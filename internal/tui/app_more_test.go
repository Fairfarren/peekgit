package tui

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Fairfarren/peekgit/internal/config"
	"github.com/Fairfarren/peekgit/internal/gitcli"
	"github.com/Fairfarren/peekgit/internal/model"
	ghprovider "github.com/Fairfarren/peekgit/internal/provider/github"
	"github.com/Fairfarren/peekgit/internal/workspace"
	tea "github.com/charmbracelet/bubbletea"
	gh "github.com/google/go-github/v57/github"
)

func TestAppViewsAllScreens(t *testing.T) {
	a := newTestApp()
	a.width = 100
	a.height = 30

	// 1. screenWorkspaces
	a.screen = screenWorkspaces
	a.startTab = startTabWorkspace
	a.workspaces = []string{"ws1", "ws2"}
	if v := a.View(); v == "" {
		t.Fatal("empty view for screenWorkspaces tabWorkspace")
	}

	// startTabPR
	a.startTab = startTabPR
	a.startPRs = []model.AccountPullRequestItem{
		{Number: 1, Title: "PR One", RepoFull: "o/r", CIStatus: "SUCCESS", StateLabel: "OPEN"},
	}
	if v := a.View(); v == "" {
		t.Fatal("empty view for screenWorkspaces tabPR")
	}
	a.startLoading = true
	if v := a.View(); v == "" {
		t.Fatal("empty view for screenWorkspaces tabPR loading")
	}
	a.startLoading = false
	a.startPRErr = "some PR error"
	if v := a.View(); v == "" {
		t.Fatal("empty view for screenWorkspaces tabPR error")
	}

	// startTabIssue
	a.startTab = startTabIssue
	a.startIssues = []model.AccountIssueItem{
		{Number: 2, Title: "Issue Two", RepoFull: "o/r", StateLabel: "OPEN | 我创建", Labels: []string{"bug"}},
	}
	if v := a.View(); v == "" {
		t.Fatal("empty view for screenWorkspaces tabIssue")
	}
	a.startLoading = true
	if v := a.View(); v == "" {
		t.Fatal("empty view for screenWorkspaces tabIssue loading")
	}
	a.startLoading = false
	a.startIssueErr = "some issue error"
	if v := a.View(); v == "" {
		t.Fatal("empty view for screenWorkspaces tabIssue error")
	}

	// 2. screenHome
	a.screen = screenHome
	a.loading = false
	if v := a.View(); v == "" {
		t.Fatal("empty view for screenHome")
	}
	a.loading = true
	if v := a.View(); v == "" {
		t.Fatal("empty view for screenHome loading")
	}
	a.loading = false
	a.filterMode = true
	a.filterText = "abc"
	if v := a.View(); v == "" {
		t.Fatal("empty view for screenHome filterMode")
	}
	a.filterMode = false

	// 3. screenDetail
	a.screen = screenDetail
	a.detailTab = tabPR
	a.prList = []model.PullRequestItem{
		{Number: 10, Title: "Detail PR", Author: "alice", UpdatedAt: time.Now(), Draft: true},
	}
	if v := a.View(); v == "" {
		t.Fatal("empty view for screenDetail tabPR")
	}
	a.detailTab = tabIssue
	a.issues = []model.IssueItem{
		{Number: 20, Title: "Detail Issue", UpdatedAt: time.Now(), Labels: []string{"help-wanted"}},
	}
	if v := a.View(); v == "" {
		t.Fatal("empty view for screenDetail tabIssue")
	}
	a.remoteErr = "some remote error"
	if v := a.View(); v == "" {
		t.Fatal("empty view for screenDetail error")
	}
	a.remoteErr = ""

	// 4. screenDiff
	a.screen = screenDiff
	a.diffTree = nil
	a.diffContent = "diff content"
	a.diffViewport.SetContent(a.diffContent)
	if v := a.View(); v == "" {
		t.Fatal("empty view for screenDiff simple")
	}

	// screenDiff with file tree (split mode)
	a.diffTree = &DiffTree{
		Files: []FileDiff{
			{Path: "file1.go", IsNew: true, AddLines: 5},
			{Path: "dir/file2.go", IsDelete: true, DelLines: 3},
		},
	}
	a.diffTree.Tree = buildTree(a.diffTree.Files)
	a.diffTree.FileList = flattenTree(a.diffTree.Tree)
	a.diffFileIdx = 0
	a.diffFocusLeft = true
	if v := a.View(); v == "" {
		t.Fatal("empty view for screenDiff split focusLeft")
	}
	a.diffFocusLeft = false
	if v := a.View(); v == "" {
		t.Fatal("empty view for screenDiff split focusRight")
	}
	a.searchMode = true
	a.searchInput = "query"
	if v := a.View(); v == "" {
		t.Fatal("empty view for screenDiff searchMode")
	}
}

func TestAppKeyHandlingWorkspacesScreen(t *testing.T) {
	a := newTestApp()
	a.screen = screenWorkspaces
	a.startTab = startTabWorkspace
	a.workspaces = []string{"ws1", "ws2", "ws3"}
	a.columns = 2

	// tab navigation
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	if a.startTab != startTabPR {
		t.Fatalf("expected startTabPR, got %v", a.startTab)
	}
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	if a.startTab != startTabIssue {
		t.Fatalf("expected startTabIssue, got %v", a.startTab)
	}
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	if a.startTab != startTabWorkspace {
		t.Fatalf("expected startTabWorkspace, got %v", a.startTab)
	}
	a.Update(tea.KeyMsg{Type: tea.KeyTab})
	if a.startTab != startTabPR {
		t.Fatalf("expected startTabPR after tab, got %v", a.startTab)
	}

	// navigation keys in startTabPR
	a.startPRs = []model.AccountPullRequestItem{{Number: 1}, {Number: 2}}
	a.startPRIdx = 0
	a.Update(tea.KeyMsg{Type: tea.KeyDown})
	if a.startPRIdx != 1 {
		t.Fatalf("expected startPRIdx 1, got %d", a.startPRIdx)
	}
	a.Update(tea.KeyMsg{Type: tea.KeyUp})
	if a.startPRIdx != 0 {
		t.Fatalf("expected startPRIdx 0, got %d", a.startPRIdx)
	}
	a.Update(tea.KeyMsg{Type: tea.KeyRight}) // switches tab
	if a.startTab != startTabIssue {
		t.Fatalf("expected startTabIssue after right, got %v", a.startTab)
	}
	a.Update(tea.KeyMsg{Type: tea.KeyLeft}) // switches tab
	if a.startTab != startTabPR {
		t.Fatalf("expected startTabPR after left, got %v", a.startTab)
	}

	// navigation keys in startTabIssue
	a.startTab = startTabIssue
	a.startIssues = []model.AccountIssueItem{{Number: 1}, {Number: 2}}
	a.startIssueIdx = 0
	a.Update(tea.KeyMsg{Type: tea.KeyDown})
	if a.startIssueIdx != 1 {
		t.Fatalf("expected startIssueIdx 1, got %d", a.startIssueIdx)
	}
	a.Update(tea.KeyMsg{Type: tea.KeyUp})
	if a.startIssueIdx != 0 {
		t.Fatalf("expected startIssueIdx 0, got %d", a.startIssueIdx)
	}

	// navigation in startTabWorkspace
	a.startTab = startTabWorkspace
	a.selectedWsIndex = 0
	a.Update(tea.KeyMsg{Type: tea.KeyRight})
	if a.selectedWsIndex != 1 {
		t.Fatalf("expected selectedWsIndex 1, got %d", a.selectedWsIndex)
	}
	a.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if a.selectedWsIndex != 0 {
		t.Fatalf("expected selectedWsIndex 0, got %d", a.selectedWsIndex)
	}
	a.Update(tea.KeyMsg{Type: tea.KeyDown})
	a.Update(tea.KeyMsg{Type: tea.KeyUp})

	// action keys: "r", "o", "d", "enter", "q"
	origBrowser := openBrowser
	openBrowser = func(string) error { return nil }
	defer func() { openBrowser = origBrowser }()

	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	a.startTab = startTabPR
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	a.Update(tea.KeyMsg{Type: tea.KeyEnter})

	a.startTab = startTabIssue
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})

	a.screen = screenWorkspaces
	a.startTab = startTabWorkspace
	a.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if a.screen != screenHome {
		t.Fatalf("expected enter on workspace to go to screenHome, got %v", a.screen)
	}

	// quit
	a.screen = screenWorkspaces
	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit command on 'q'")
	}
}

func TestAppKeyHandlingHomeScreen(t *testing.T) {
	a := newTestApp()
	a.screen = screenHome
	a.filterMode = false

	// navigation
	a.Update(tea.KeyMsg{Type: tea.KeyRight})
	a.Update(tea.KeyMsg{Type: tea.KeyLeft})
	a.Update(tea.KeyMsg{Type: tea.KeyDown})
	a.Update(tea.KeyMsg{Type: tea.KeyUp})

	// actions: r, /, f, F, g, space, enter
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if !a.loading {
		t.Fatal("expected loading to be true after 'r'")
	}

	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !a.filterMode {
		t.Fatal("expected filterMode true after '/'")
	}

	// filter input typing
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	a.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	a.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if a.filterMode {
		t.Fatal("expected filterMode false after enter")
	}

	// git actions f, F, g
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})

	// enter moves to detail
	a.filterMode = false
	a.filterText = ""
	a.repos = []model.RepoStatus{{Name: "repo-a", Path: "/tmp/repo-a"}}
	a.selectedIndex = 0
	a.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if a.screen != screenDetail {
		t.Fatalf("expected screenDetail, got %v", a.screen)
	}

	// exit key q / esc
	a.screen = screenHome
	a.filterText = "text"
	a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if a.filterText != "" {
		t.Fatalf("expected filterText cleared on esc")
	}
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if a.screen != screenWorkspaces {
		t.Fatalf("expected screenWorkspaces on q, got %v", a.screen)
	}
}

func TestAppKeyHandlingDetailScreen(t *testing.T) {
	a := newTestApp()
	a.screen = screenDetail
	a.detailTab = tabPR
	a.prList = []model.PullRequestItem{{Number: 1}, {Number: 2}}
	a.issues = []model.IssueItem{{Number: 10}, {Number: 20}}

	origBrowser := openBrowser
	openBrowser = func(string) error { return nil }
	defer func() { openBrowser = origBrowser }()

	// tab navigation
	a.Update(tea.KeyMsg{Type: tea.KeyTab})
	if a.detailTab != tabIssue {
		t.Fatalf("expected tabIssue, got %v", a.detailTab)
	}
	a.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if a.detailTab != tabPR {
		t.Fatalf("expected tabPR, got %v", a.detailTab)
	}
	a.Update(tea.KeyMsg{Type: tea.KeyRight})
	if a.detailTab != tabIssue {
		t.Fatalf("expected tabIssue, got %v", a.detailTab)
	}
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	if a.detailTab != tabPR {
		t.Fatalf("expected tabPR, got %v", a.detailTab)
	}
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	if a.detailTab != tabIssue {
		t.Fatalf("expected tabIssue, got %v", a.detailTab)
	}

	// selection movement
	a.Update(tea.KeyMsg{Type: tea.KeyDown})
	if a.detailISIdx != 1 {
		t.Fatalf("expected detailISIdx 1, got %d", a.detailISIdx)
	}
	a.Update(tea.KeyMsg{Type: tea.KeyUp})
	if a.detailISIdx != 0 {
		t.Fatalf("expected detailISIdx 0, got %d", a.detailISIdx)
	}

	a.detailTab = tabPR
	a.Update(tea.KeyMsg{Type: tea.KeyDown})
	if a.detailPRIdx != 1 {
		t.Fatalf("expected detailPRIdx 1, got %d", a.detailPRIdx)
	}
	a.Update(tea.KeyMsg{Type: tea.KeyUp})
	if a.detailPRIdx != 0 {
		t.Fatalf("expected detailPRIdx 0, got %d", a.detailPRIdx)
	}

	// actions: r, o, d, enter
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	a.screen = screenDetail
	a.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// q returns to home
	a.screen = screenDetail
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if a.screen != screenHome {
		t.Fatalf("expected screenHome, got %v", a.screen)
	}
}

func TestAppKeyHandlingDiffScreen(t *testing.T) {
	a := newTestApp()
	a.screen = screenDiff
	a.diffFocusLeft = true
	a.diffTree = &DiffTree{
		Files: []FileDiff{
			{Path: "f1.go", Content: "diff1"},
			{Path: "f2.go", Content: "diff2"},
		},
	}
	a.diffTree.Tree = buildTree(a.diffTree.Files)
	a.diffTree.FileList = flattenTree(a.diffTree.Tree)
	a.diffFileIdx = 0

	// toggle focus
	a.Update(tea.KeyMsg{Type: tea.KeyTab})
	if a.diffFocusLeft {
		t.Fatal("expected diffFocusLeft false after tab")
	}
	a.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !a.diffFocusLeft {
		t.Fatal("expected diffFocusLeft true after tab")
	}
	a.Update(tea.KeyMsg{Type: tea.KeyRight})
	if a.diffFocusLeft {
		t.Fatal("expected diffFocusLeft false after right")
	}
	a.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if !a.diffFocusLeft {
		t.Fatal("expected diffFocusLeft true after left")
	}

	// left panel movement
	a.Update(tea.KeyMsg{Type: tea.KeyDown})
	if a.diffFileIdx != 1 {
		t.Fatalf("expected diffFileIdx 1, got %d", a.diffFileIdx)
	}
	a.Update(tea.KeyMsg{Type: tea.KeyUp})
	if a.diffFileIdx != 0 {
		t.Fatalf("expected diffFileIdx 0, got %d", a.diffFileIdx)
	}

	// right panel scrolling
	a.diffFocusLeft = false
	a.Update(tea.KeyMsg{Type: tea.KeyDown})
	a.Update(tea.KeyMsg{Type: tea.KeyUp})
	a.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	a.Update(tea.KeyMsg{Type: tea.KeyCtrlU})

	// search keys
	a.searchMode = true
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	a.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if a.searchMode {
		t.Fatal("expected searchMode false after enter")
	}

	// exit diff
	a.diffSourceScreen = screenDetail
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if a.screen != screenDetail {
		t.Fatalf("expected screenDetail, got %v", a.screen)
	}

	// exit diff when opened from account PR
	a.screen = screenDiff
	a.diffSourceScreen = screenWorkspaces
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if a.screen != screenWorkspaces {
		t.Fatalf("expected screenWorkspaces, got %v", a.screen)
	}
}

func TestAppMessageHandling(t *testing.T) {
	a := newTestApp()

	// 1. tickMsg
	a.loading = false
	a.Update(tickMsg(time.Now()))
	a.loading = true
	a.Update(tickMsg(time.Now()))

	// 2. configWatchTickMsg
	a.Update(configWatchTickMsg(time.Now()))

	// 3. handleDiffLoaded
	a.Update(diffLoadedMsg{err: errors.New("diff err")})
	if a.errText != "diff err" {
		t.Fatalf("unexpected diff errText: %v", a.errText)
	}
	a.Update(diffLoadedMsg{
		content: "raw diff",
		files:   []FileDiff{{Path: "f1.go", Content: "diff content"}},
	})
	if a.diffContent != "raw diff" {
		t.Fatalf("expected raw diff content, got %s", a.diffContent)
	}

	// 4. handlePullDone
	a.repoRefreshing["/tmp/repo-a"] = true
	a.Update(pullDoneMsg{repoPath: "/tmp/repo-a", err: errors.New("pull err")})
	if !strings.Contains(a.errText, "pull 失败") {
		t.Fatalf("expected pull error recorded")
	}
	a.Update(pullDoneMsg{repoPath: "/tmp/repo-a"})
	if a.repoRefreshing["/tmp/repo-a"] {
		t.Fatal("expected repoRefreshing cleared")
	}

	// 5. handlePullAllDone
	a.Update(pullAllDoneMsg{completed: 1, failed: 1, lastErr: errors.New("pullall err")})
	if !strings.Contains(a.errText, "pull 完成") {
		t.Fatal("expected pullall error set")
	}
	a.Update(pullAllDoneMsg{completed: 2, failed: 0})
	if a.errText != "" {
		t.Fatal("expected empty errText on success")
	}

	// 6. handleLazygitDone
	a.Update(lazygitDoneMsg{err: errors.New("lazygit err")})
	if !strings.Contains(a.errText, "lazygit 执行失败") {
		t.Fatal("expected lazygit error recorded")
	}
	a.Update(lazygitDoneMsg{})

	// 7. workspaceCheckDoneMsg
	a.workspaceHasUpdate["ws1"] = false
	a.Update(workspaceCheckDoneMsg{workspace: "ws1", hasUpdate: true})
	if !a.workspaceHasUpdate["ws1"] {
		t.Fatal("expected ws1 marked hasUpdate")
	}
	a.Update(workspaceCheckDoneMsg{workspace: "ws_nonexistent", hasUpdate: true})

	// 8. accountRemoteLoadedMsg
	a.startLoading = true
	a.Update(accountRemoteLoadedMsg{
		prs:      []model.AccountPullRequestItem{{Number: 1}},
		items:    []model.AccountIssueItem{{Number: 2}},
		prErr:    "pr-err",
		issueErr: "issue-err",
	})
	if a.startLoading || a.startPRErr != "pr-err" || a.startIssueErr != "issue-err" {
		t.Fatal("unexpected accountRemoteLoadedMsg handling")
	}

	// 9. refreshDoneMsg errors and mismatch
	a.refreshSeq = 10
	a.loading = true
	a.Update(refreshDoneMsg{seq: 5, repos: nil}) // seq mismatch ignored
	if !a.loading {
		t.Fatal("expected loading to remain true on seq mismatch")
	}
	a.Update(refreshDoneMsg{seq: 10, err: errors.New("refresh failed")})
	if a.loading {
		t.Fatal("expected loading false on error")
	}

	// 10. repoRefreshDoneMsg
	a.Update(repoRefreshDoneMsg{
		seq:    a.refreshSeq,
		status: model.RepoStatus{Name: "repo-a", Path: "/tmp/repo-a", Branch: "feat", Ahead: 3},
	})
	a.Update(repoRefreshDoneMsg{
		seq:    a.refreshSeq,
		status: model.RepoStatus{Name: "unknown", Path: "/tmp/unknown-path"},
	})

	// 11. remoteLoadedMsg
	a.screen = screenDetail
	a.selectedIndex = 0
	a.repos = []model.RepoStatus{{Path: "/tmp/repo-a"}}
	a.Update(remoteLoadedMsg{
		repoPath:  "/tmp/repo-a",
		prs:       []model.PullRequestItem{{Number: 1}},
		issues:    []model.IssueItem{{Number: 2}},
		remoteErr: "remote failed",
	})
	if a.remoteErr != "remote failed" {
		t.Fatalf("expected remoteErr, got %s", a.remoteErr)
	}
}

func TestAppCommandsExecution(t *testing.T) {
	origBrowser := openBrowser
	openBrowser = func(string) error { return nil }
	defer func() { openBrowser = origBrowser }()

	a := newTestApp()

	// tickCmd
	cmd := tickCmd(300)
	if cmd == nil {
		t.Fatal("expected tickCmd")
	}

	// configWatchTickCmd
	cmd = configWatchTickCmd()
	if cmd == nil {
		t.Fatal("expected configWatchTickCmd")
	}

	// scanWorkspaceRepos
	a.cfg.WorkspaceMode = true
	a.cfg.WorkspaceRoot = t.TempDir()
	_, _ = a.scanWorkspaceRepos([]string{"/tmp"})
	a.cfg.WorkspaceMode = false
	_, _ = a.scanWorkspaceRepos([]string{"/tmp"})

	// refreshRepoCmd
	cmd = a.refreshRepoCmd(1, "repo", "/tmp/nonexistent")
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(repoRefreshDoneMsg); !ok {
			t.Fatalf("expected repoRefreshDoneMsg, got %T", msg)
		}
	}

	// loadRemoteCmd
	cmd = a.loadRemoteCmd(model.RepoStatus{Path: "/tmp/nonexistent"})
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(remoteLoadedMsg); !ok {
			t.Fatalf("expected remoteLoadedMsg, got %T", msg)
		}
	}

	// loadPRDiffCmd
	cmd = a.loadPRDiffCmd(model.RepoStatus{Path: "/tmp/nonexistent"}, 1)
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(diffLoadedMsg); !ok {
			t.Fatalf("expected diffLoadedMsg, got %T", msg)
		}
	}

	// loadAccountPRDiffCmd
	cmd = a.loadAccountPRDiffCmd("owner/repo", 1)
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(diffLoadedMsg); !ok {
			t.Fatalf("expected diffLoadedMsg, got %T", msg)
		}
	}

	// fetchDiffOrFiles
	_ = a.fetchDiffOrFiles(context.Background(), "owner", "repo", 1)

	// openCurrentURLCmd
	a.detailTab = tabPR
	a.prList = []model.PullRequestItem{{HTMLURL: "https://example.com/pr/1"}}
	cmd = a.openCurrentURLCmd()
	if cmd != nil {
		cmd()
	}
	a.detailTab = tabIssue
	a.issues = []model.IssueItem{{HTMLURL: "https://example.com/issue/1"}}
	cmd = a.openCurrentURLCmd()
	if cmd != nil {
		cmd()
	}

	// openWorkspaceTabCurrentURLCmd
	a.startTab = startTabPR
	a.startPRs = []model.AccountPullRequestItem{{HTMLURL: "https://example.com/startpr"}}
	cmd = a.openWorkspaceTabCurrentURLCmd()
	if cmd != nil {
		cmd()
	}
	a.startTab = startTabIssue
	a.startIssues = []model.AccountIssueItem{{HTMLURL: "https://example.com/startissue"}}
	cmd = a.openWorkspaceTabCurrentURLCmd()
	if cmd != nil {
		cmd()
	}

	// browserOpenCmd
	c := browserOpenCmd("https://example.com")
	if c == nil {
		t.Fatal("expected non-nil exec.Cmd from browserOpenCmd")
	}

	// workspaceCheckOneCmd
	cmd = a.workspaceCheckOneCmd("ws1", []string{"/tmp/nonexistent"})
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(workspaceCheckDoneMsg); !ok {
			t.Fatalf("expected workspaceCheckDoneMsg, got %T", msg)
		}
	}

	// reloadGlobalConfigCmd
	cmd = a.reloadGlobalConfigCmd()
	if cmd != nil {
		cmd()
	}

	// pullCurrentCmd
	a.repos = []model.RepoStatus{{Path: "/tmp/repo-a"}}
	a.selectedIndex = 0
	cmd = a.pullCurrentCmd()
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(pullDoneMsg); !ok {
			t.Fatalf("expected pullDoneMsg, got %T", msg)
		}
	}

	// pullAllCmd
	cmd = a.pullAllCmd()
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(pullAllDoneMsg); !ok {
			t.Fatalf("expected pullAllDoneMsg, got %T", msg)
		}
	}

	// runLazygitCmd
	cmd = a.runLazygitCmd("/tmp")
	if cmd != nil {
		_ = cmd
	}

	// loadAccountRemoteCmd
	cmd = a.loadAccountRemoteCmd()
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(accountRemoteLoadedMsg); !ok {
			t.Fatalf("expected accountRemoteLoadedMsg, got %T", msg)
		}
	}
}

func TestAppHelpers(t *testing.T) {
	// calculateFileTreeOffset
	if off := calculateFileTreeOffset(0, 5, 10, 2); off != 0 {
		t.Fatalf("expected 0, got %d", off)
	}
	if off := calculateFileTreeOffset(0, 30, 10, 8); off <= 0 {
		t.Fatalf("expected positive offset, got %d", off)
	}
	if off := calculateFileTreeOffset(15, 30, 10, 1); off != 0 {
		t.Fatalf("expected offset 0 when selection near top, got %d", off)
	}

	// renderDirTreeLine
	dirLine := renderDirTreeLine("  ", "a_very_long_directory_name_that_needs_to_be_truncated", 15)
	if dirLine == "" || !strings.Contains(dirLine, "…") {
		t.Fatalf("expected truncated dir line, got %s", dirLine)
	}

	// renderFileDiffTreeLine
	tl := treeLine{
		indent:    1,
		name:      "very_long_file_name_for_testing_truncation.go",
		file:      &FileDiff{IsNew: true, AddLines: 10, DelLines: 2},
		fileIndex: 0,
	}
	fl := renderFileDiffTreeLine(tl, 20, true, true)
	if fl == "" {
		t.Fatal("expected non-empty file diff line")
	}

	// getFileStatusIcon
	if icon := getFileStatusIcon(nil); icon == "" {
		t.Fatal("expected non-empty icon for nil")
	}
	if icon := getFileStatusIcon(&FileDiff{IsDelete: true}); icon == "" {
		t.Fatal("expected non-empty icon for delete")
	}

	// getFileStatsStr
	if s := getFileStatsStr(nil); s != "" {
		t.Fatalf("expected empty stats for nil, got %q", s)
	}
	if s := getFileStatsStr(&FileDiff{AddLines: 0, DelLines: 0}); s != "" {
		t.Fatalf("expected empty stats for 0 lines, got %q", s)
	}
	if s := getFileStatsStr(&FileDiff{AddLines: 5, DelLines: 2}); s == "" {
		t.Fatal("expected non-empty stats for 5/2 lines")
	}

	// formatIssueCell & truncateWithEllipsis
	if cell := formatIssueCell("hello", 10); cell != "hello     " {
		t.Fatalf("unexpected cell format: %q", cell)
	}
	if cell := formatIssueCell("hello world long", 8); !strings.Contains(cell, "…") {
		t.Fatalf("expected ellipsis in cell: %q", cell)
	}
	if tr := truncateWithEllipsis("short", 10); tr != "short" {
		t.Fatalf("expected unchanged: %q", tr)
	}
	if tr := truncateWithEllipsis("a", 0); tr != "" {
		t.Fatalf("expected empty: %q", tr)
	}
	if tr := truncateWithEllipsis("abc", 1); tr != "…" {
		t.Fatalf("expected ellipsis: %q", tr)
	}

	// maxIssueNumberWidthDetail & maxPRNumberWidthDetail
	a := newTestApp()
	a.issues = []model.IssueItem{{Number: 12345}, {Number: 9}}
	if w := maxIssueNumberWidthDetail(a.issues); w != 6 {
		t.Fatalf("expected 6 (#12345), got %d", w)
	}
	a.prList = []model.PullRequestItem{{Number: 123}, {Number: 4567}}
	if w := maxPRNumberWidthDetail(a.prList); w != 5 {
		t.Fatalf("expected 5 (#4567), got %d", w)
	}

	// prStartLabelSummary
	if s := prStartLabelSummary("OPEN", "SUCCESS"); !strings.Contains(s, "CI:SUCCESS") {
		t.Fatalf("expected CI:SUCCESS, got %s", s)
	}
	if s := prStartLabelSummary("", ""); !strings.Contains(s, "UNKNOWN") {
		t.Fatalf("expected UNKNOWN, got %s", s)
	}

	// issueTableColumnWidths
	wTitle, wLabels, wUpdated := issueTableColumnWidths(100, 5)
	if wTitle <= 0 || wLabels <= 0 || wUpdated <= 0 {
		t.Fatalf("invalid column widths: %d, %d, %d", wTitle, wLabels, wUpdated)
	}

	// composeWithFooter
	if s := composeWithFooter(0, []string{"body"}, "footer"); s != "" {
		t.Fatalf("expected empty for height 0, got %q", s)
	}
	if s := composeWithFooter(1, []string{"body"}, "footer"); s == "" {
		t.Fatal("expected footer for height 1")
	}
	if s := composeWithFooter(5, []string{"line1", "line2", "line3", "line4", "line5"}, "footer"); s == "" {
		t.Fatal("expected composed text")
	}

	// setSearch & jumpMatch
	a.setSearch("test")
	a.jumpMatch(1)
	a.jumpMatch(-1)
	a.setSearch("")
}

func TestBrowserOpenCmdForOS(t *testing.T) {
	cmdDarwin := browserOpenCmdForOS("darwin", "https://example.com")
	if cmdDarwin.Path != "open" && !strings.HasSuffix(cmdDarwin.Path, "/open") {
		t.Fatalf("expected open command for darwin, got: %s", cmdDarwin.Path)
	}

	cmdLinux := browserOpenCmdForOS("linux", "https://example.com")
	if cmdLinux.Path != "xdg-open" && !strings.HasSuffix(cmdLinux.Path, "/xdg-open") {
		t.Fatalf("expected xdg-open command for linux, got: %s", cmdLinux.Path)
	}

	cmdWindows := browserOpenCmdForOS("windows", "https://example.com")
	if cmdWindows.Path != "cmd" && !strings.HasSuffix(cmdWindows.Path, "\\cmd.exe") && !strings.HasSuffix(cmdWindows.Path, "/cmd") {
		t.Fatalf("expected cmd command for windows, got: %s", cmdWindows.Path)
	}

	cmdCurrent := browserOpenCmd("https://example.com")
	if cmdCurrent == nil {
		t.Fatal("expected non-nil cmd for current OS")
	}

	origOpen := openBrowser
	defer func() { openBrowser = origOpen }()
	called := false
	openBrowser = func(url string) error {
		called = true
		return nil
	}
	_ = openBrowser("https://example.com")
	if !called {
		t.Fatal("expected openBrowser to be called")
	}

	_ = origOpen("about:blank")
}

func TestTickAndConfigTickFunctions(t *testing.T) {
	now := time.Now()
	msg1 := tickMsgFunc(now)
	if _, ok := msg1.(tickMsg); !ok {
		t.Fatalf("expected tickMsg, got %T", msg1)
	}

	cmd1 := tickCmd(1)
	if cmd1 == nil {
		t.Fatal("expected non-nil tickCmd")
	}

	msg2 := configWatchTickMsgFunc(now)
	if _, ok := msg2.(configWatchTickMsg); !ok {
		t.Fatalf("expected configWatchTickMsg, got %T", msg2)
	}

	cmd2 := configWatchTickCmd()
	if cmd2 == nil {
		t.Fatal("expected non-nil configWatchTickCmd")
	}
}

func TestRunLazygitCmdMocked(t *testing.T) {
	a := newTestApp()

	origLookPath := lookPath
	defer func() { lookPath = origLookPath }()

	lookPath = func(file string) (string, error) {
		return "", errors.New("not found")
	}

	cmd := a.runLazygitCmd("/tmp")
	msg := cmd()
	lzMsg, ok := msg.(lazygitDoneMsg)
	if !ok || lzMsg.err == nil || !strings.Contains(lzMsg.err.Error(), "lazygit 未安装") {
		t.Fatalf("expected lazygit not installed error, got %v", msg)
	}

	lookPath = func(file string) (string, error) {
		return "/bin/echo", nil
	}
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()
	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "test")
	}

	cmdProcess := a.runLazygitCmd("/tmp")
	if cmdProcess == nil {
		t.Fatal("expected non-nil cmdProcess")
	}
}

func TestRebuildReposFromDirsMatchExisting(t *testing.T) {
	a := newTestApp()
	a.repos = []model.RepoStatus{
		{Name: "repo-old", Path: "/tmp/repo-a", Branch: "main", Sync: model.SyncSynced},
	}
	dirs := []workspace.RepoDir{
		{Name: "repo-renamed", Path: "/tmp/repo-a"},
		{Name: "repo-new", Path: "/tmp/repo-new"},
	}
	a.rebuildReposFromDirs(dirs)
	if len(a.repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(a.repos))
	}
	foundRenamed := false
	for _, r := range a.repos {
		if r.Path == "/tmp/repo-a" && r.Name == "repo-renamed" && r.Sync == model.SyncSynced {
			foundRenamed = true
		}
	}
	if !foundRenamed {
		t.Fatal("expected existing repo to be updated with preserved state")
	}
}

func TestHandleRepoRefreshDoneMismatchedSeq(t *testing.T) {
	a := newTestApp()
	a.refreshSeq = 10
	cmd := a.handleRepoRefreshDone(repoRefreshDoneMsg{seq: 9})
	if cmd != nil {
		t.Fatalf("expected nil cmd for stale seq, got %v", cmd)
	}
}

func TestHandleRemoteLoadedMismatchedRepo(t *testing.T) {
	a := newTestApp()
	a.selectedIndex = 0
	cmd := a.handleRemoteLoaded(remoteLoadedMsg{repoPath: "/different/repo/path"})
	if cmd != nil {
		t.Fatalf("expected nil cmd for mismatched repo, got %v", cmd)
	}
}

func TestHandleDiffLoadedWithoutFiles(t *testing.T) {
	a := newTestApp()
	rawDiff := "diff --git a/foo.txt b/foo.txt\n--- a/foo.txt\n+++ b/foo.txt\n@@ -1 +1 @@\n-old\n+new\n"
	cmd := a.handleDiffLoaded(diffLoadedMsg{content: rawDiff})
	if cmd != nil {
		t.Fatalf("expected nil cmd, got %v", cmd)
	}
	if a.diffTree == nil || len(a.diffTree.Files) != 1 {
		t.Fatalf("expected 1 file parsed from diff, got %v", a.diffTree)
	}
}

func TestSetDiffViewportInitialContentBranches(t *testing.T) {
	a := newTestApp()
	a.diffViewport.Width = 80
	a.diffViewport.Height = 20

	a.diffTree = &DiffTree{
		Files: []FileDiff{
			{Path: "large.txt", Content: ""},
		},
		FileList: []string{"large.txt"},
	}
	a.setDiffViewportInitialContent("fallback")
	if !strings.Contains(a.diffViewport.View(), "Diff 内容由于 PR 过大") {
		t.Fatalf("expected PR too large message in viewport, got: %s", a.diffViewport.View())
	}

	a.diffTree = &DiffTree{Files: nil}
	a.setDiffViewportInitialContent("fallback diff text")
	if !strings.Contains(a.diffViewport.View(), "fallback diff text") {
		t.Fatalf("expected fallback text in viewport, got: %s", a.diffViewport.View())
	}
}

func TestHandlePullAllDoneWithActiveRefreshes(t *testing.T) {
	a := newTestApp()
	a.repoRefreshing["/tmp/repo-a"] = true
	a.repoRefreshing["/tmp/repo-b"] = true
	a.handlePullAllDone(pullAllDoneMsg{completed: 2, failed: 0})
	if a.repoRefreshing["/tmp/repo-a"] || a.repoRefreshing["/tmp/repo-b"] {
		t.Fatal("expected repoRefreshing flags to be cleared")
	}
}

func TestHandleKeyMsgUnhandledScreen(t *testing.T) {
	a := newTestApp()
	a.screen = screen(999)
	m, cmd := a.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if m != a || cmd != nil {
		t.Fatalf("expected unchanged app and nil cmd, got %v, %v", m, cmd)
	}
}

func TestUpdateWorkspacesKeyHandling(t *testing.T) {
	a := newTestApp()
	a.screen = screenWorkspaces
	a.startTab = startTabWorkspace

	cmd, ok := a.handleWorkspaceActionKey("d")
	if cmd != nil || !ok {
		t.Fatalf("expected nil cmd and ok=true for 'd' in workspace tab, got %v, %v", cmd, ok)
	}

	cmd, ok = a.handleWorkspaceActionKey("unknown_key")
	if cmd != nil || ok {
		t.Fatalf("expected nil cmd and ok=false for unknown key, got %v, %v", cmd, ok)
	}

	a.workspaces = nil
	cmd, ok = a.handleWorkspaceActionKey("enter")
	if cmd != nil || !ok {
		t.Fatalf("expected nil cmd and ok=true for enter with no workspaces, got %v, %v", cmd, ok)
	}

	m, cmd := a.updateWorkspaces(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	if m != a || cmd != nil {
		t.Fatalf("expected unchanged app for unhandled key in updateWorkspaces")
	}
}

func TestUpdateSearchInputEsc(t *testing.T) {
	a := newTestApp()
	a.searchMode = true
	a.searchInput = "search text"
	a.updateSearchInput(tea.KeyMsg{Type: tea.KeyEsc})
	if a.searchMode || a.searchInput != "" {
		t.Fatalf("expected searchMode=false and empty searchInput, got %v, %q", a.searchMode, a.searchInput)
	}
}

func TestOpenSelectedRepoDetailEmpty(t *testing.T) {
	a := newTestApp()
	cmd, ok := a.openSelectedRepoDetail(nil)
	if cmd != nil || !ok {
		t.Fatalf("expected nil cmd and ok=true when visible repos is empty, got %v, %v", cmd, ok)
	}
}

func TestHandleHomeGitActionBranches(t *testing.T) {
	a := newTestApp()

	cmd, ok := a.handleHomeGitAction("f", nil)
	if cmd != nil || !ok {
		t.Fatalf("expected nil cmd, ok=true for 'f' on empty visible, got %v, %v", cmd, ok)
	}

	cmd, ok = a.handleHomeGitAction("f", a.repos)
	if cmd == nil || !ok {
		t.Fatalf("expected cmd, ok=true for 'f' with repos")
	}

	a.repos = nil
	cmd, ok = a.handleHomeGitAction("F", nil)
	if cmd != nil || !ok {
		t.Fatalf("expected nil cmd, ok=true for 'F' on empty repos, got %v, %v", cmd, ok)
	}

	a.repos = []model.RepoStatus{{Path: "/tmp/repo-a"}}
	cmd, ok = a.handleHomeGitAction("F", nil)
	if cmd == nil || !ok {
		t.Fatalf("expected cmd, ok=true for 'F' with repos")
	}

	cmd, ok = a.handleHomeGitAction("g", nil)
	if cmd != nil || !ok {
		t.Fatalf("expected nil cmd, ok=true for 'g' on empty visible, got %v, %v", cmd, ok)
	}

	cmd, ok = a.handleHomeGitAction("g", a.repos)
	if cmd == nil || !ok {
		t.Fatalf("expected cmd, ok=true for 'g' with repos")
	}

	cmd, ok = a.handleHomeActionKey("unknown", a.repos)
	if cmd != nil || ok {
		t.Fatalf("expected nil cmd, ok=false for unknown action")
	}

	m, cmdQuit := a.updateHome(tea.KeyMsg{Type: tea.KeyCtrlC})
	if m != a || cmdQuit == nil {
		t.Fatalf("expected tea.Quit for ctrl+c in updateHome")
	}

	m, cmdNil := a.updateHome(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	if m != a || cmdNil != nil {
		t.Fatalf("expected nil cmd for unhandled key in updateHome")
	}
}

func TestHandleDetailActionKeyBranches(t *testing.T) {
	a := newTestApp()

	cmd, ok := a.handleDetailActionKey("r", model.RepoStatus{})
	if cmd != nil || !ok {
		t.Fatalf("expected nil cmd, ok=true for 'r' with empty repo name")
	}

	a.detailTab = tabIssue
	cmd, ok = a.handleDetailActionKey("d", a.repos[0])
	if cmd != nil || !ok {
		t.Fatalf("expected nil cmd, ok=true for 'd' on tabIssue")
	}

	a.detailTab = tabPR
	a.prList = nil
	cmd, ok = a.handleDetailActionKey("d", a.repos[0])
	if cmd != nil || !ok {
		t.Fatalf("expected nil cmd, ok=true for 'd' on empty prList")
	}
}

func TestUpdateDiffContentBranches(t *testing.T) {
	a := newTestApp()
	a.diffViewport.Width = 80
	a.diffViewport.Height = 20

	a.diffTree = nil
	a.updateDiffContent()

	a.diffTree = &DiffTree{Files: nil}
	a.updateDiffContent()

	a.diffTree = &DiffTree{
		Files:    []FileDiff{{Path: "f.txt", Content: ""}},
		FileList: []string{"f.txt"},
	}
	a.diffFileIdx = 0
	a.updateDiffContent()
	if !strings.Contains(a.diffViewport.View(), "Diff 内容由于 PR 过大") {
		t.Fatalf("expected fallback text for empty content")
	}

	a.diffFileIdx = 999
	a.diffContent = "diffContent-fallback"
	a.updateDiffContent()
	if !strings.Contains(a.diffViewport.View(), "diffContent-fallback") {
		t.Fatalf("expected diffContent fallback")
	}
}

func TestViewBranches(t *testing.T) {
	a := newTestApp()

	a.width = 0
	a.height = 0
	if v := a.View(); v != "初始化中..." {
		t.Fatalf("expected '初始化中...', got %q", v)
	}

	a.width = 100
	a.height = 30
	a.screen = screen(999)
	if v := a.View(); v != "" {
		t.Fatalf("expected empty string for unknown screen, got %q", v)
	}

	a.screen = screenWorkspaces
	a.workspaces = nil
	if v := a.View(); !strings.Contains(v, "无工作区配置") {
		t.Fatalf("expected '无工作区配置', got %q", v)
	}

	if rows := a.calculateVisibleWorkspaceRows(nil, 2, 2); len(rows) != 0 {
		t.Fatalf("expected empty visible workspace rows")
	}

	if rows := a.calculateVisibleRepoRows(nil, 2, 2); len(rows) != 0 {
		t.Fatalf("expected empty visible repo rows")
	}

	a.screen = screenHome
	a.loading = true
	a.repos = nil
	if v := a.View(); !strings.Contains(v, "刷新中...") {
		t.Fatalf("expected '刷新中...', got %q", v)
	}
}

func TestRenderWorkspaceCardIndicators(t *testing.T) {
	a := newTestApp()
	a.workspaceChecking["ws1"] = true
	c1 := a.renderWorkspaceCard("ws1", false)
	if c1 == "" {
		t.Fatal("empty card for checking")
	}

	a.workspaceChecking["ws1"] = false
	a.workspaceHasUpdate["ws1"] = true
	c2 := a.renderWorkspaceCard("ws1", false)
	if !strings.Contains(c2, "↓") {
		t.Fatalf("expected update indicator in card, got %q", c2)
	}
}

func TestRenderCardVariants(t *testing.T) {
	a := newTestApp()
	val := 3
	repo := model.RepoStatus{
		Name:      "test-repo",
		Path:      "/tmp/test-repo",
		Dirty:     true,
		PROpen:    &val,
		IssueOpen: &val,
		Error:     model.RepoError("fatal: error"),
	}
	a.repoRefreshing["/tmp/test-repo"] = true
	card := a.renderCard(repo, true)
	if !strings.Contains(card, "✎") || !strings.Contains(card, "!fatal: error") {
		t.Fatalf("expected dirty and error markers in card, got: %s", card)
	}
}

func TestRenderStartPRAndIssueLinesAuthAndErrors(t *testing.T) {
	a := newTestApp()

	a.gh = ghprovider.NewWithClient(nil)
	linesPR := a.renderStartPRLines([]string{"header"})
	if !strings.Contains(strings.Join(linesPR, "\n"), "当前未登录 GitHub") {
		t.Fatalf("expected unauth message for PRs, got %v", linesPR)
	}
	linesIS := a.renderStartIssueLines([]string{"header"})
	if !strings.Contains(strings.Join(linesIS, "\n"), "当前未登录 GitHub") {
		t.Fatalf("expected unauth message for issues, got %v", linesIS)
	}

	ghc := gh.NewClient(nil)
	a.gh = ghprovider.NewWithClient(ghc)
	a.startPRErr = "pr error text"
	linesPRErr := a.renderStartPRLines([]string{"header"})
	if !strings.Contains(strings.Join(linesPRErr, "\n"), "pr error text") {
		t.Fatalf("expected pr error text banner")
	}
	a.startIssueErr = "issue error text"
	linesISErr := a.renderStartIssueLines([]string{"header"})
	if !strings.Contains(strings.Join(linesISErr, "\n"), "issue error text") {
		t.Fatalf("expected issue error text banner")
	}

	a.startPRErr = ""
	a.startIssueErr = ""
	a.startLoading = true
	a.startPRs = nil
	a.startIssues = nil
	linesPRLoading := a.renderStartPRLines([]string{"header"})
	if !strings.Contains(strings.Join(linesPRLoading, "\n"), "加载当前账号 PR 中...") {
		t.Fatalf("expected PR loading text, got %v", linesPRLoading)
	}
	linesISLoading := a.renderStartIssueLines([]string{"header"})
	if !strings.Contains(strings.Join(linesISLoading, "\n"), "加载当前账号 Issues 中...") {
		t.Fatalf("expected issue loading text, got %v", linesISLoading)
	}

	a.startLoading = false
	linesPREmpty := a.renderStartPRLines([]string{"header"})
	if !strings.Contains(strings.Join(linesPREmpty, "\n"), "当前账号下暂无 PR") {
		t.Fatalf("expected no PRs text, got %v", linesPREmpty)
	}
	linesISEmpty := a.renderStartIssueLines([]string{"header"})
	if !strings.Contains(strings.Join(linesISEmpty, "\n"), "当前账号下暂无 Issues") {
		t.Fatalf("expected no issues text, got %v", linesISEmpty)
	}
}

func TestSwitchStartTabBounds(t *testing.T) {
	a := newTestApp()
	a.switchStartTab(startTabWorkspace - 1)
	if a.startTab != startTabIssue {
		t.Fatalf("expected startTabIssue, got %v", a.startTab)
	}
	a.switchStartTab(startTabIssue + 1)
	if a.startTab != startTabWorkspace {
		t.Fatalf("expected startTabWorkspace, got %v", a.startTab)
	}
}

func TestOpenWorkspaceTabCurrentURLEmpty(t *testing.T) {
	a := newTestApp()
	a.startPRs = nil
	a.startIssues = nil
	cmd := a.openWorkspaceTabCurrentURLCmd()
	msg := cmd()
	if msg != nil {
		t.Fatalf("expected nil msg from openWorkspaceTabCurrentURLCmd when url is empty, got %v", msg)
	}
}

func TestBuildHomeHeaderLinesAuth(t *testing.T) {
	a := newTestApp()
	ghc := gh.NewClient(nil)
	a.gh = ghprovider.NewWithClient(ghc)
	lines := a.buildHomeHeaderLines()
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "token: github ✓") {
		t.Fatalf("expected token OK in home header, got: %s", joined)
	}
}

func TestBuildDetailHeaderLinesLoadingAndErr(t *testing.T) {
	a := newTestApp()
	a.loading = true
	a.errText = "custom error text"
	lines := a.buildDetailHeaderLines(a.repos[0])
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "刷新中...") || !strings.Contains(joined, "custom error text") {
		t.Fatalf("expected loading and errText in detail header, got: %s", joined)
	}

	a.issues = nil
	linesIS := a.buildDetailIssueListLines(10)
	if len(linesIS) == 0 || linesIS[0] != "暂无 Issues" {
		t.Fatalf("expected '暂无 Issues', got %v", linesIS)
	}
}

func TestDiffPanelDimensionsAndRightPanelEllipsis(t *testing.T) {
	a := newTestApp()
	a.width = 40
	a.height = 10
	leftW, rightW, contentH := a.calculateDiffPanelDimensions()
	if leftW <= 0 || rightW <= 0 || contentH <= 0 {
		t.Fatalf("invalid dimensions: %d, %d, %d", leftW, rightW, contentH)
	}

	panel := a.renderDiffRightPanel(50, 10, "some/super/long/directory/path/that/definitely/exceeds/the/panel/width/file.txt")
	if !strings.Contains(panel, "...") {
		t.Fatalf("expected truncated title with ellipsis, got: %s", panel)
	}
}

func TestViewDiffLoading(t *testing.T) {
	a := newTestApp()
	a.diffLoading = true
	v := a.viewDiff()
	if !strings.Contains(v, "加载 diff 中...") {
		t.Fatalf("expected loading diff text, got %q", v)
	}
}

func TestIssueTableColumnWidthsDeficit(t *testing.T) {
	tw, _, _ := issueTableColumnWidths(30, 5)
	if tw < 10 {
		t.Fatalf("expected min title width 10, got %d", tw)
	}

	tw2, lw2, uw2 := issueTableColumnWidths(45, 5)
	if tw2 < 10 || lw2 < 8 || uw2 < 16 {
		t.Fatalf("unexpected widths: %d, %d, %d", tw2, lw2, uw2)
	}

	tw3, _, _ := issueTableColumnWidths(40, 30)
	if tw3 != 10 {
		t.Fatalf("expected title width 10, got %d", tw3)
	}

	if s := formatIssueCell("test", 0); s != "" {
		t.Fatalf("expected empty for width 0, got %q", s)
	}
}

func TestViewDiffSimpleSmallHeight(t *testing.T) {
	a := newTestApp()
	a.height = 1
	v := a.viewDiffSimple()
	if v == "" {
		t.Fatal("expected non-empty simple diff view")
	}
}

func TestFindSelectedFileLineNotFound(t *testing.T) {
	lines := []treeLine{
		{isDir: true},
		{isDir: false, fileIndex: 1},
	}
	if idx := findSelectedFileLine(lines, 99); idx != 0 {
		t.Fatalf("expected 0 for not found file line, got %d", idx)
	}
}

func TestCalculateFileTreeOffsetBranches(t *testing.T) {
	off1 := calculateFileTreeOffset(0, 10, 2, 5)
	if off1 <= 0 {
		t.Fatalf("expected positive offset, got %d", off1)
	}

	off2 := calculateFileTreeOffset(10, 10, 5, 9)
	if off2 != 5 {
		t.Fatalf("expected offset 5, got %d", off2)
	}
}

func TestBuildTreeLinesNilNode(t *testing.T) {
	a := newTestApp()
	var lines []treeLine
	cnt := 0
	a.buildTreeLines(nil, 0, &lines, &cnt)
	if len(lines) != 0 {
		t.Fatalf("expected 0 lines for nil node")
	}
}

func TestRefreshAllCmdEmptyWorkspaces(t *testing.T) {
	a := newTestApp()
	a.workspaces = nil
	cmd := a.refreshAllCmd()
	if cmd != nil {
		t.Fatalf("expected nil cmd for empty workspaces, got %v", cmd)
	}
}

func TestRefreshAllCmdScanError(t *testing.T) {
	orig := scanWorkspaceReposFn
	defer func() { scanWorkspaceReposFn = orig }()
	scanWorkspaceReposFn = func(cfg config.Config, paths []string) ([]workspace.RepoDir, error) {
		return nil, errors.New("scan error")
	}

	a := newTestApp()
	a.workspaces = []string{"ws1"}
	cmd := a.refreshAllCmd()
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	rfMsg, ok := msg.(refreshDoneMsg)
	if !ok || rfMsg.err == nil {
		t.Fatalf("expected error from scanWorkspaceRepos, got %v", msg)
	}
}

func TestFetchDiffOrFilesViaServer(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/owner/repo/pulls/1", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") == "application/vnd.github.v3.diff" {
			_, _ = w.Write([]byte("diff --git a/a.txt b/a.txt\n"))
			return
		}
	})
	mux.HandleFunc("/api/v3/repos/owner/repo/pulls/2", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotAcceptable)
	})
	mux.HandleFunc("/api/v3/repos/owner/repo/pulls/2/files", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"filename":"new.txt","patch":"+added","additions":1,"deletions":0,"status":"added"},
			{"filename":"del.txt","patch":"-removed","additions":0,"deletions":1,"status":"removed"},
			{"filename":"mod.txt","patch":"+-mod","additions":1,"deletions":1,"status":"modified"}
		]`))
	})
	mux.HandleFunc("/api/v3/repos/owner/repo/pulls/3", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotAcceptable)
	})
	mux.HandleFunc("/api/v3/repos/owner/repo/pulls/3/files", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("/api/v3/repos/owner/repo/pulls/4", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ghc, _ := gh.NewClient(nil).WithEnterpriseURLs(srv.URL+"/", srv.URL+"/")
	a := newTestApp()
	a.gh = ghprovider.NewWithClient(ghc)

	ctx := context.Background()

	msg1 := a.fetchDiffOrFiles(ctx, "owner", "repo", 1).(diffLoadedMsg)
	if msg1.err != nil || !strings.Contains(msg1.content, "diff --git") {
		t.Fatalf("expected diff content, got %v", msg1)
	}

	msg2 := a.fetchDiffOrFiles(ctx, "owner", "repo", 2).(diffLoadedMsg)
	if msg2.err != nil || len(msg2.files) != 3 {
		t.Fatalf("expected 3 files, got %v", msg2)
	}
	if !msg2.files[0].IsNew || !msg2.files[1].IsDelete {
		t.Fatalf("expected IsNew and IsDelete flags set correctly")
	}

	msg3 := a.fetchDiffOrFiles(ctx, "owner", "repo", 3).(diffLoadedMsg)
	if msg3.err == nil {
		t.Fatalf("expected error when listing PR files fails")
	}

	msg4 := a.fetchDiffOrFiles(ctx, "owner", "repo", 4).(diffLoadedMsg)
	if msg4.err == nil {
		t.Fatalf("expected error for general failure")
	}
}

func TestLoadRemoteCmdViaServer(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/myowner/myrepo/pulls", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"number":10,"title":"PR 10","user":{"login":"alice"}}]`))
	})
	mux.HandleFunc("/api/v3/repos/myowner/myrepo/issues", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"number":20,"title":"Issue 20","user":{"login":"bob"}}]`))
	})
	mux.HandleFunc("/api/v3/repos/erruser/errrepo/pulls", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("/api/v3/repos/erruser/errrepo/issues", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ghc, _ := gh.NewClient(nil).WithEnterpriseURLs(srv.URL+"/", srv.URL+"/")
	a := newTestApp()
	a.gh = ghprovider.NewWithClient(ghc)

	tmpEmpty := t.TempDir()
	cmd1 := a.loadRemoteCmd(model.RepoStatus{Path: tmpEmpty})
	msg1 := cmd1().(remoteLoadedMsg)
	if msg1.remoteErr != "no-remote" {
		t.Fatalf("expected no-remote, got %s", msg1.remoteErr)
	}

	tmpGit := t.TempDir()
	_ = exec.Command("git", "init", tmpGit).Run()
	_ = exec.Command("git", "-C", tmpGit, "remote", "add", "origin", "https://github.com/myowner/myrepo.git").Run()

	cmd2 := a.loadRemoteCmd(model.RepoStatus{Path: tmpGit})
	msg2 := cmd2().(remoteLoadedMsg)
	if msg2.remoteErr != "" || len(msg2.prs) != 1 || len(msg2.issues) != 1 {
		t.Fatalf("expected loaded PRs and issues, got %v", msg2)
	}
	if msg2.prOpen == nil || *msg2.prOpen != 1 || msg2.issueOpen == nil || *msg2.issueOpen != 1 {
		t.Fatalf("expected counts 1, got %v, %v", msg2.prOpen, msg2.issueOpen)
	}

	tmpErr := t.TempDir()
	_ = exec.Command("git", "init", tmpErr).Run()
	_ = exec.Command("git", "-C", tmpErr, "remote", "add", "origin", "https://github.com/erruser/errrepo.git").Run()

	cmd3 := a.loadRemoteCmd(model.RepoStatus{Path: tmpErr})
	msg3 := cmd3().(remoteLoadedMsg)
	if msg3.remoteErr != "fetch" {
		t.Fatalf("expected fetch error, got %s", msg3.remoteErr)
	}

	a.gh = ghprovider.NewWithClient(nil)
	cmd4 := a.loadRemoteCmd(model.RepoStatus{Path: tmpGit})
	msg4 := cmd4().(remoteLoadedMsg)
	if msg4.remoteErr != "unauth" {
		t.Fatalf("expected unauth, got %s", msg4.remoteErr)
	}
}

func TestRefreshRepoCmdViaServer(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/o1/r1/pulls", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"number":1}]`))
	})
	mux.HandleFunc("/api/v3/repos/o1/r1/issues", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"number":2}]`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ghc, _ := gh.NewClient(nil).WithEnterpriseURLs(srv.URL+"/", srv.URL+"/")
	a := newTestApp()
	a.gh = ghprovider.NewWithClient(ghc)

	tmpGit := t.TempDir()
	_ = exec.Command("git", "init", tmpGit).Run()
	_ = exec.Command("git", "-C", tmpGit, "remote", "add", "origin", "https://github.com/o1/r1.git").Run()

	cmd := a.refreshRepoCmd(1, "r1", tmpGit)
	msg := cmd().(repoRefreshDoneMsg)
	if msg.status.PROpen == nil || *msg.status.PROpen != 1 {
		t.Fatalf("expected PROpen=1, got %v", msg.status.PROpen)
	}
	if msg.status.IssueOpen == nil || *msg.status.IssueOpen != 1 {
		t.Fatalf("expected IssueOpen=1, got %v", msg.status.IssueOpen)
	}
}

func TestLoadAccountRemoteCmdErrors(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/search/issues", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ghc, _ := gh.NewClient(nil).WithEnterpriseURLs(srv.URL+"/", srv.URL+"/")
	a := newTestApp()
	a.gh = ghprovider.NewWithClient(ghc)

	cmd := a.loadAccountRemoteCmd()
	msg := cmd().(accountRemoteLoadedMsg)
	if msg.prErr == "" || msg.issueErr == "" {
		t.Fatalf("expected error text for PR and issues, got %q, %q", msg.prErr, msg.issueErr)
	}
}

func TestWorkspaceCheckOneCmdBranches(t *testing.T) {
	a := newTestApp()

	cmd1 := a.workspaceCheckOneCmd("ws1", []string{"/invalid/nonexistent/path"})
	msg1 := cmd1().(workspaceCheckDoneMsg)
	if msg1.hasUpdate {
		t.Fatalf("expected hasUpdate=false for invalid path")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo1")
	_ = os.MkdirAll(repoDir, 0755)
	_ = exec.Command("git", "init", repoDir).Run()

	cmd2 := a.workspaceCheckOneCmd("ws1", []string{tmpDir})
	msg2 := cmd2().(workspaceCheckDoneMsg)
	if msg2.hasUpdate {
		t.Fatalf("expected hasUpdate=false")
	}
}

func TestReconcileWorkspaceRuntimeStateBranches(t *testing.T) {
	a := newTestApp()

	a.workspaces = nil
	a.reconcileWorkspaceRuntimeState("ws1")
	if a.selectedWsIndex != 0 || a.screen != screenWorkspaces {
		t.Fatalf("expected reset to 0 and screenWorkspaces")
	}

	a.workspaces = []string{"ws1", "ws2", "ws3"}
	a.reconcileWorkspaceRuntimeState("ws3")
	if a.selectedWsIndex != 2 {
		t.Fatalf("expected selectedWsIndex=2, got %d", a.selectedWsIndex)
	}

	a.selectedWsIndex = 10
	a.reconcileWorkspaceRuntimeState("wsNotFound")
	if a.selectedWsIndex != 2 {
		t.Fatalf("expected selectedWsIndex=2, got %d", a.selectedWsIndex)
	}

	a.selectedWsIndex = -5
	a.reconcileWorkspaceRuntimeState("")
	if a.selectedWsIndex != 0 {
		t.Fatalf("expected selectedWsIndex=0, got %d", a.selectedWsIndex)
	}
}

func TestCalcWorkspaceCountsError(t *testing.T) {
	ws := config.WorkspaceMap{
		"invalid": []string{"/non/existent/path/12345"},
	}
	counts := calcWorkspaceCounts(ws, []string{"invalid"})
	if counts["invalid"] != 0 {
		t.Fatalf("expected 0 for invalid workspace path, got %d", counts["invalid"])
	}
}

func TestRecomputeGridBranches(t *testing.T) {
	a := newTestApp()

	a.screen = screenWorkspaces
	a.workspaces = nil
	a.recomputeGrid()
	if a.selectedWsIndex != 0 {
		t.Fatalf("expected selectedWsIndex=0")
	}

	a.screen = screenHome
	a.repos = nil
	a.recomputeGrid()
	if a.selectedIndex != 0 {
		t.Fatalf("expected selectedIndex=0")
	}
}

func TestSetSearchDiffTreeFiles(t *testing.T) {
	a := newTestApp()
	a.diffContent = ""
	a.diffTree = &DiffTree{
		Files: []FileDiff{
			{Path: "f1.go", Content: "package main\n\nfunc SearchKeyword() {}"},
		},
	}
	a.setSearch("SearchKeyword")
	if len(a.matches) != 1 {
		t.Fatalf("expected 1 match in diffTree files, got %d", len(a.matches))
	}

	a.matches = nil
	a.jumpMatch(1)
	if a.matchIdx != -1 && len(a.matches) != 0 {
		t.Fatalf("unexpected jump on empty matches")
	}
}

func TestPullCurrentCmdNoRepo(t *testing.T) {
	a := newTestApp()
	a.repos = nil
	cmd := a.pullCurrentCmd()
	msg := cmd().(pullDoneMsg)
	if msg.err == nil || !strings.Contains(msg.err.Error(), "no repo selected") {
		t.Fatalf("expected 'no repo selected' error, got %v", msg.err)
	}
}

func TestPullAllCmdFailures(t *testing.T) {
	a := newTestApp()
	a.repos = []model.RepoStatus{
		{Path: "/nonexistent/repo1"},
		{Path: "/nonexistent/repo2"},
	}
	cmd := a.pullAllCmd()
	msg := cmd().(pullAllDoneMsg)
	if msg.failed != 2 || msg.lastErr == nil {
		t.Fatalf("expected 2 failed pulls, got failed=%d, err=%v", msg.failed, msg.lastErr)
	}
}

func TestNewWorkspaceModeScanError(t *testing.T) {
	cfg := config.Config{
		WorkspaceMode: true,
		WorkspaceRoot: filepath.Join(t.TempDir(), "nonexistent_dir"),
		Global: config.GlobalConfig{
			Workspaces: map[string][]string{"default": {"/tmp"}},
		},
	}
	app := New(cfg)
	if app.workspaceCounts["default"] != 0 {
		t.Fatalf("expected count 0 on scan error, got %d", app.workspaceCounts["default"])
	}
}

func TestInitVariants(t *testing.T) {
	cfg1 := config.Config{
		WorkspaceMode: false,
		NoGitHub:      true,
	}
	app1 := New(cfg1)
	app1.workspaces = nil
	cmd1 := app1.Init()
	if cmd1 == nil {
		t.Fatal("expected non-nil cmd1")
	}

	cfg2 := config.Config{
		WorkspaceMode: true,
		WorkspaceRoot: t.TempDir(),
		Global: config.GlobalConfig{
			Workspaces: map[string][]string{"default": {"/tmp"}},
		},
	}
	ghc := gh.NewClient(nil)
	app2 := New(cfg2)
	app2.gh = ghprovider.NewWithClient(ghc)
	cmd2 := app2.Init()
	if cmd2 == nil {
		t.Fatal("expected non-nil cmd2")
	}
}

func TestUpdateRepoStatusAndOpenCounts(t *testing.T) {
	a := newTestApp()
	a.repos = []model.RepoStatus{
		{Name: "r1", Path: "/p1"},
		{Name: "r2", Path: "/p2"},
	}

	a.updateRepoStatus(model.RepoStatus{Name: "r2-updated", Path: "/p2"})
	if a.repos[1].Name != "r2-updated" {
		t.Fatalf("expected r2-updated, got %s", a.repos[1].Name)
	}

	a.updateRepoStatus(model.RepoStatus{Name: "r3", Path: "/p3"})

	cnt := 5
	a.updateRepoOpenCounts("/p2", &cnt, &cnt)
	if a.repos[1].PROpen == nil || *a.repos[1].PROpen != 5 {
		t.Fatalf("expected PROpen=5, got %v", a.repos[1].PROpen)
	}

	a.updateRepoOpenCounts("/pNotFound", &cnt, &cnt)
}

type mockGitExecForTest struct {
	fn func(args ...string) (string, error)
}

func (m mockGitExecForTest) Run(ctx context.Context, dir string, args ...string) (string, error) {
	if m.fn != nil {
		return m.fn(args...)
	}
	return "", nil
}

func TestRemainingEdgeCases(t *testing.T) {
	a := newTestApp()

	// 1. a.Update with unhandled msg
	m, cmd := a.Update(struct{}{})
	if m != a || cmd != nil {
		t.Fatalf("expected unchanged app for unhandled msg")
	}

	// 2. configWatchTickMsg in WorkspaceMode
	a.cfg.WorkspaceMode = true
	cmd, ok := a.handleLifecycleMsg(configWatchTickMsg(time.Now()))
	if cmd != nil || !ok {
		t.Fatalf("expected nil cmd and ok=true in WorkspaceMode")
	}

	// 3. handleTick in screenHome and not loading
	a.screen = screenHome
	a.loading = false
	cmdTick := a.handleTick()
	if cmdTick == nil {
		t.Fatal("expected non-nil cmdTick")
	}

	// 4. handleConfigReloaded with error
	cmdCfgErr := a.handleConfigReloaded(configReloadedMsg{err: errors.New("cfg err")})
	if cmdCfgErr != nil || a.errText != "配置文件错误: cfg err" {
		t.Fatalf("expected error text set, got: %s", a.errText)
	}

	// 5. handleConfigReloaded with screen != screenWorkspaces
	a.screen = screenHome
	a.workspaces = []string{"ws1"}
	cmdCfgHome := a.handleConfigReloaded(configReloadedMsg{global: config.GlobalConfig{Workspaces: map[string][]string{"ws1": {"/tmp"}}}})
	if cmdCfgHome == nil {
		t.Fatal("expected batch cmd in screenHome")
	}

	// 6. handleConfigReloaded when len(cmds) == 0 (screenWorkspaces and 0 workspaces)
	a.screen = screenWorkspaces
	a.workspaces = nil
	a.cfg.Global = config.GlobalConfig{Workspaces: map[string][]string{"old": {"/tmp"}}}
	cmdEmpty := a.handleConfigReloaded(configReloadedMsg{global: config.GlobalConfig{}})
	if cmdEmpty != nil {
		t.Fatalf("expected nil cmd when len(cmds) == 0, got %v", cmdEmpty)
	}

	// 7. handleRefreshDone with 0 repos
	a.refreshSeq = 1
	cmdNoRepos := a.handleRefreshDone(refreshDoneMsg{seq: 1, repos: nil})
	if cmdNoRepos != nil {
		t.Fatalf("expected nil cmd for 0 repos refresh")
	}

	// 8. refreshAllCmd returning msg
	a.workspaces = []string{"ws1"}
	a.selectedWsIndex = 0
	a.cfg.WorkspaceMode = false
	a.cfg.Global = config.GlobalConfig{Workspaces: map[string][]string{"ws1": {"/tmp"}}}
	cmdRefresh := a.refreshAllCmd()
	if cmdRefresh != nil {
		msg := cmdRefresh()
		if rfMsg, ok := msg.(refreshDoneMsg); !ok || rfMsg.seq != a.refreshSeq {
			t.Fatalf("expected refreshDoneMsg with seq %d, got %v", a.refreshSeq, msg)
		}
	}

	// 9. loadPRDiffCmd with valid remote repo
	tmpGit := t.TempDir()
	_ = exec.Command("git", "init", tmpGit).Run()
	_ = exec.Command("git", "-C", tmpGit, "remote", "add", "origin", "https://github.com/owner/repo.git").Run()
	cmdPRDiff := a.loadPRDiffCmd(model.RepoStatus{Path: tmpGit}, 1)
	if cmdPRDiff != nil {
		_ = cmdPRDiff()
	}

	// 10. loadAccountPRDiffCmd with invalid repoFull (no slash)
	cmdAccDiff := a.loadAccountPRDiffCmd("invalid-repo-format", 1)
	if cmdAccDiff != nil {
		msg := cmdAccDiff().(diffLoadedMsg)
		if msg.err == nil || !strings.Contains(msg.err.Error(), "invalid repo name") {
			t.Fatalf("expected invalid repo name error, got %v", msg.err)
		}
	}

	// 11. openCurrentURLCmd when url is empty
	a.prList = nil
	a.issues = nil
	cmdURL := a.openCurrentURLCmd()
	if msg := cmdURL(); msg != nil {
		t.Fatalf("expected nil msg from openCurrentURLCmd, got %v", msg)
	}

	// 12. recomputeGrid bounds
	a.screen = screenWorkspaces
	a.workspaces = []string{"ws1"}
	a.selectedWsIndex = 10
	a.recomputeGrid()
	if a.selectedWsIndex != 0 {
		t.Fatalf("expected selectedWsIndex bounded to 0, got %d", a.selectedWsIndex)
	}

	a.screen = screenHome
	a.repos = []model.RepoStatus{{Name: "r1"}}
	a.selectedIndex = 10
	a.recomputeGrid()
	if a.selectedIndex != 0 {
		t.Fatalf("expected selectedIndex bounded to 0, got %d", a.selectedIndex)
	}

	// 13. pullAllCmd success (completed++)
	a.repos = []model.RepoStatus{{Path: "/p1"}}
	a.git = gitcli.NewWithExecutor(mockGitExecForTest{
		fn: func(args ...string) (string, error) {
			return "", nil
		},
	})
	cmdPullAll := a.pullAllCmd()
	msgPullAll := cmdPullAll().(pullAllDoneMsg)
	if msgPullAll.completed != 1 || msgPullAll.failed != 0 {
		t.Fatalf("expected completed=1, got %v", msgPullAll)
	}

	// 14. lazygitDoneFunc
	msgLzOk := lazygitDoneFunc(nil).(lazygitDoneMsg)
	if msgLzOk.err != nil {
		t.Fatalf("expected nil err, got %v", msgLzOk.err)
	}
	msgLzErr := lazygitDoneFunc(errors.New("err")).(lazygitDoneMsg)
	if msgLzErr.err == nil {
		t.Fatal("expected err")
	}
}

func TestWorkspaceCheckOneCmdWithUpdates(t *testing.T) {
	a := newTestApp()

	a.git = gitcli.NewWithExecutor(mockGitExecForTest{
		fn: func(args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.Contains(joined, "symbolic-ref") {
				return "main\n", nil
			}
			if strings.Contains(joined, "@{upstream}") {
				return "origin/main\n", nil
			}
			if strings.Contains(joined, "fetch") {
				return "", nil
			}
			if strings.Contains(joined, "rev-list") {
				return "0\t1\n", nil
			}
			return "", nil
		},
	})

	origScan := scanWorkspaceReposFn
	defer func() { scanWorkspaceReposFn = origScan }()

	// Case A: 2 repos (< 3), updateFound = true
	scanWorkspaceReposFn = func(cfg config.Config, paths []string) ([]workspace.RepoDir, error) {
		return []workspace.RepoDir{
			{Name: "r1", Path: "/p1"},
			{Name: "r2", Path: "/p2"},
		}, nil
	}
	cmdA := a.workspaceCheckOneCmd("ws1", []string{"/tmp"})
	msgA := cmdA().(workspaceCheckDoneMsg)
	if !msgA.hasUpdate {
		t.Fatalf("expected hasUpdate=true")
	}

	// Case B: 4 repos (> 3), updateFound = false
	a.git = gitcli.NewWithExecutor(mockGitExecForTest{
		fn: func(args ...string) (string, error) {
			return "", errors.New("no upstream")
		},
	})
	scanWorkspaceReposFn = func(cfg config.Config, paths []string) ([]workspace.RepoDir, error) {
		return []workspace.RepoDir{
			{Name: "r1", Path: "/p1"},
			{Name: "r2", Path: "/p2"},
			{Name: "r3", Path: "/p3"},
			{Name: "r4", Path: "/p4"},
		}, nil
	}
	cmdB := a.workspaceCheckOneCmd("ws1", []string{"/tmp"})
	msgB := cmdB().(workspaceCheckDoneMsg)
	if msgB.hasUpdate {
		t.Fatalf("expected hasUpdate=false")
	}
}
