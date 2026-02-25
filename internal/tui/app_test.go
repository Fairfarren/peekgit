package tui

import (
	"testing"

	"github.com/Fairfarren/peekgit/internal/config"
	"github.com/Fairfarren/peekgit/internal/model"
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

func TestEmptyDash(t *testing.T) {
	if emptyDash("") != "—" {
		t.Fatalf("expected dash")
	}
	if emptyDash("origin/main") != "origin/main" {
		t.Fatalf("expected original value")
	}
}
