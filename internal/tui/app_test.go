package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Fairfarren/peekgit/internal/config"
	"github.com/Fairfarren/peekgit/internal/model"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func TestFilteredRepos(t *testing.T) {
	a := New(config.Config{Global: config.GlobalConfig{Workspaces: map[string][]string{"default": {"/tmp"}}}, IntervalSec: 300, Concurrency: 1, NoGitHub: true})
	a.repos = []model.RepoStatus{{Name: "repo-a"}, {Name: "demo"}}
	a.filterText = "repo"
	got := a.filteredRepos()
	if len(got) != 1 || got[0].Name != "repo-a" {
		t.Fatalf("unexpected filtered repos: %+v", got)
	}
}

func TestRecomputeGrid(t *testing.T) {
	a := New(config.Config{Global: config.GlobalConfig{Workspaces: map[string][]string{"default": {"/tmp"}}}, IntervalSec: 300, Concurrency: 1, NoGitHub: true})
	a.width = 120
	a.repos = []model.RepoStatus{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	a.recomputeGrid()
	if a.columns < 2 {
		t.Fatalf("columns=%d", a.columns)
	}
	if a.cardWidth <= 0 {
		t.Fatalf("card width=%d", a.cardWidth)
	}
}

func TestUpdateSpinnerTick(t *testing.T) {
	a := New(config.Config{Global: config.GlobalConfig{Workspaces: map[string][]string{"default": {"/tmp"}}}, IntervalSec: 300, Concurrency: 1, NoGitHub: true})

	_, cmd := a.Update(spinner.TickMsg{})
	if cmd == nil {
		t.Fatalf("expected next tick command")
	}
	if a.spinner.View() == "" {
		t.Fatalf("spinner view should not be empty")
	}
}

func TestUpdateSearchInput(t *testing.T) {
	a := New(config.Config{Global: config.GlobalConfig{Workspaces: map[string][]string{"default": {"/tmp"}}}, IntervalSec: 300, Concurrency: 1, NoGitHub: true})
	a.searchMode = true

	_, _ = a.updateSearchInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	_, _ = a.updateSearchInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	_, _ = a.updateSearchInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	_, _ = a.updateSearchInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})

	if a.searchInput != "test" {
		t.Fatalf("expected searchInput 'test', got '%s'", a.searchInput)
	}

	_, _ = a.updateSearchInput(tea.KeyMsg{Type: tea.KeyBackspace})
	if a.searchInput != "tes" {
		t.Fatalf("expected searchInput 'tes', got '%s'", a.searchInput)
	}

	_, _ = a.updateSearchInput(tea.KeyMsg{Type: tea.KeyEnter})
	if a.searchMode {
		t.Fatalf("expected searchMode to be false after enter")
	}
	if a.diffSearch != "tes" {
		t.Fatalf("expected diffSearch 'tes', got '%s'", a.diffSearch)
	}
}

func TestEmptyDash(t *testing.T) {
	if emptyDash("") != "—" {
		t.Fatalf("expected dash")
	}
	if emptyDash("origin/main") != "origin/main" {
		t.Fatalf("expected original value")
	}
}

func TestWorkspaceModeStartsOnHomeAndInitRefreshes(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	cfg := config.Config{
		Global:         config.GlobalConfig{Workspaces: map[string][]string{root: []string{root}}},
		IntervalSec:    300,
		Concurrency:    1,
		NoGitHub:       true,
		WorkspaceMode:  true,
		WorkspaceDepth: 1,
		WorkspaceRoot:  root,
	}
	a := New(cfg)
	if a.screen != screenHome {
		t.Fatalf("expected home screen in workspace mode")
	}
	cmd := a.Init()
	if cmd == nil {
		t.Fatalf("expected init command")
	}
	if !a.loading {
		t.Fatalf("expected loading to be true after startup refresh command setup")
	}
}
