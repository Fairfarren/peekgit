package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestFormatRelativeTime(t *testing.T) {
	now := time.Now()
	if got := formatRelativeTime(now.Add(5*time.Second), now); got != "just now" {
		t.Errorf("got %q, want just now", got)
	}
	if got := formatRelativeTime(time.Time{}, now); got != "-" {
		t.Errorf("got %q, want -", got)
	}
	if got := formatRelativeTime(now.Add(-30*time.Second), now); got != "just now" {
		t.Errorf("got %q, want just now", got)
	}
	if got := formatRelativeTime(now.Add(-time.Minute), now); got != "about 1 minute ago" {
		t.Errorf("got %q, want about 1 minute ago", got)
	}
	if got := formatRelativeTime(now.Add(-5*time.Minute), now); got != "about 5 minutes ago" {
		t.Errorf("got %q, want about 5 minutes ago", got)
	}
	if got := formatRelativeTime(now.Add(-time.Hour), now); got != "about 1 hour ago" {
		t.Errorf("got %q, want about 1 hour ago", got)
	}
	if got := formatRelativeTime(now.Add(-3*time.Hour), now); got != "about 3 hours ago" {
		t.Errorf("got %q, want about 3 hours ago", got)
	}
	if got := formatRelativeTime(now.Add(-24*time.Hour), now); got != "about 1 day ago" {
		t.Errorf("got %q, want about 1 day ago", got)
	}
	if got := formatRelativeTime(now.Add(-4*24*time.Hour), now); got != "about 4 days ago" {
		t.Errorf("got %q, want about 4 days ago", got)
	}
	if got := formatRelativeTime(now.Add(-35*24*time.Hour), now); got != "about 1 month ago" {
		t.Errorf("got %q, want about 1 month ago", got)
	}
	if got := formatRelativeTime(now.Add(-70*24*time.Hour), now); got != "about 2 months ago" {
		t.Errorf("got %q, want about 2 months ago", got)
	}
	if got := formatRelativeTime(now.Add(-400*24*time.Hour), now); got != "about 1 year ago" {
		t.Errorf("got %q, want about 1 year ago", got)
	}
	if got := formatRelativeTime(now.Add(-800*24*time.Hour), now); got != "about 2 years ago" {
		t.Errorf("got %q, want about 2 years ago", got)
	}
}

func Test_相对时间_跨单位边界与未来时间(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		offset time.Duration
		want   string
	}{
		{"整月", -30 * 24 * time.Hour, "about 1 month ago"},
		{"整年", -365 * 24 * time.Hour, "about 1 year ago"},
		{"未来五分钟", 5 * time.Minute, "about 5 minutes ago"},
		{"恰好现在", 0, "just now"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := formatRelativeTime(now.Add(tc.offset), now)

			if got != tc.want {
				t.Fatalf("相对时间 = %q，期望 %q", got, tc.want)
			}
		})
	}
}

func Test_文件树滚动_上下边界和缓冲行(t *testing.T) {
	for _, tc := range []struct {
		name                                  string
		offset, total, height, selected, want int
	}{
		{"顶部缓冲边界", 5, 30, 9, 8, 5}, {"顶部缓冲内", 5, 30, 9, 7, 4},
		{"底部缓冲边界", 5, 30, 9, 11, 6}, {"底部缓冲前", 5, 30, 9, 10, 5},
		{"向后滚动", 0, 30, 9, 15, 10}, {"末页", 20, 30, 9, 29, 21},
		{"刚好一页", 5, 9, 9, 4, 0}, {"首行", 5, 30, 9, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := calculateFileTreeOffset(tc.offset, tc.total, tc.height, tc.selected)

			if got != tc.want {
				t.Fatalf("滚动偏移 = %d，期望 %d", got, tc.want)
			}
		})
	}
}

func Test_表格宽度_逐步压缩辅助列(t *testing.T) {
	for _, tc := range []struct {
		name                              string
		width, id, title, labels, updated int
	}{
		{"完整", 62, 4, 20, 12, 20}, {"缩标签", 60, 4, 20, 10, 20}, {"再缩时间", 56, 4, 20, 8, 18},
		{"最窄", 40, 4, 10, 8, 16}, {"大编号", 64, 8, 20, 10, 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			title, labels, updated := issueTableColumnWidths(tc.width, tc.id)

			if title != tc.title || labels != tc.labels || updated != tc.updated {
				t.Fatalf("列宽 = %d/%d/%d", title, labels, updated)
			}
		})
	}
}

func Test_单元格_零宽度不显示(t *testing.T) {
	got := formatIssueCell("文本", 0)

	if got != "" {
		t.Fatalf("零宽度单元格 = %q", got)
	}
}

func Test_单行文件树_选择越过缓冲区时滚动(t *testing.T) {
	got := calculateFileTreeOffset(5, 30, 1, 6)

	if got != 7 {
		t.Fatalf("单行偏移 = %d，期望 7", got)
	}
}

func Test_文件树名称_恰好容纳与最小截断空间(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func() string
		want string
	}{
		{"目录恰好容纳", func() string { return renderDirTreeLine("", "中文", 8) }, "📂 中文/"},
		{"目录无截断空间", func() string { return renderDirTreeLine("", "abcdef", 4) }, "📂 abcdef/"},
		{"文件恰好容纳", func() string { return renderFileDiffTreeLine(treeLine{name: "中文"}, 7, false, false) }, " ~ 中文"},
		{"文件只有三字空间", func() string { return renderFileDiffTreeLine(treeLine{name: "abcdef"}, 6, false, false) }, " ~ abcdef"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.run()

			if got != tc.want {
				t.Fatalf("名称 = %q，期望 %q", got, tc.want)
			}
		})
	}
}
