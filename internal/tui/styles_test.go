package tui

import (
	"strings"
	"testing"

	"github.com/Fairfarren/peekgit/internal/config"
	"github.com/Fairfarren/peekgit/internal/model"
	"github.com/charmbracelet/lipgloss"
)

// TestSpinnerInitialization 测试 spinner 正确初始化
func TestSpinnerInitialization(t *testing.T) {
	a := New(config.Config{Global: config.GlobalConfig{Workspaces: map[string][]string{"default": {"/tmp"}}}, IntervalSec: 300, Concurrency: 1, NoGitHub: true})

	// 验证 spinner 已初始化（通过检查视图是否非空）
	view := a.spinner.View()
	if view == "" {
		t.Fatalf("expected spinner view to be non-empty")
	}
}

// TestGetResponsiveCardMinWidth 测试响应式卡片宽度计算
func TestGetResponsiveCardMinWidth(t *testing.T) {
	tests := []struct {
		width    int
		expected int
	}{
		{250, 50},
		{200, 50},
		{150, 44},
		{120, 44},
		{100, 35},
		{80, 35},
		{79, 25},
		{50, 25},
		{30, 25},
	}

	for _, tc := range tests {
		got := getResponsiveCardMinWidth(tc.width)
		if got != tc.expected {
			t.Fatalf("width=%d: expected %d, got %d", tc.width, tc.expected, got)
		}
	}
}

// TestCardStylesApplied 测试卡片样式正确应用
func TestCardStylesApplied(t *testing.T) {
	a := newTestApp()

	// 测试未选中卡片使用 CardStyle
	unselectedCard := a.renderCard(a.repos[0], false)
	if unselectedCard == "" {
		t.Fatalf("expected non-empty unselected card")
	}

	// 测试选中卡片使用 CardSelectedStyle
	selectedCard := a.renderCard(a.repos[0], true)
	if selectedCard == "" {
		t.Fatalf("expected non-empty selected card")
	}

	// 选中卡片应该有不同的样式表现
	if unselectedCard == selectedCard {
		t.Fatalf("expected selected and unselected cards to differ")
	}
}

// TestWorkspaceCardStyles 测试工作区卡片样式
func TestWorkspaceCardStyles(t *testing.T) {
	a := New(config.Config{
		Global:      config.GlobalConfig{Workspaces: map[string][]string{"ws1": {"/tmp"}, "ws2": {"/tmp"}}},
		IntervalSec: 300,
		Concurrency: 1,
		NoGitHub:    true,
	})
	a.width = 120
	a.height = 40
	a.workspaceCounts = map[string]int{"ws1": 5, "ws2": 3}
	a.recomputeGrid()

	// 测试未选中工作区卡片
	unselectedCard := a.renderWorkspaceCard("ws1", false)
	if unselectedCard == "" {
		t.Fatalf("expected non-empty unselected workspace card")
	}

	// 测试选中工作区卡片
	selectedCard := a.renderWorkspaceCard("ws1", true)
	if selectedCard == "" {
		t.Fatalf("expected non-empty selected workspace card")
	}
}

// TestRenderFileTreeWithSelection 测试文件树渲染和选中
func TestRenderFileTreeWithSelection(t *testing.T) {
	a := newTestApp()
	a.screen = screenDiff

	// 设置 diff 树
	a.diffTree = &DiffTree{
		Files: []FileDiff{
			{Path: "file1.go", IsNew: true, AddLines: 10},
			{Path: "file2.go", IsDelete: true, DelLines: 5},
			{Path: "file3.go", AddLines: 3, DelLines: 2},
		},
	}
	a.diffTree.Tree = &DiffNode{
		Name:  "",
		IsDir: true,
		Children: []*DiffNode{
			{Name: "file1.go", IsDir: false, File: &a.diffTree.Files[0]},
			{Name: "file2.go", IsDir: false, File: &a.diffTree.Files[1]},
			{Name: "file3.go", IsDir: false, File: &a.diffTree.Files[2]},
		},
	}
	a.diffFileIdx = 0

	// 渲染文件树
	treeContent := a.renderFileTree(30, 10)
	if treeContent == "" {
		t.Fatalf("expected non-empty tree content")
	}

	// 检查文件状态图标
	if !strings.Contains(treeContent, "+") {
		t.Fatalf("expected '+' icon for new file")
	}
	if !strings.Contains(treeContent, "-") {
		t.Fatalf("expected '-' icon for deleted file")
	}
	if !strings.Contains(treeContent, "~") {
		t.Fatalf("expected '~' icon for modified file")
	}
}

// TestErrorBannerStyle 测试错误横幅样式应用
func TestErrorBannerStyle(t *testing.T) {
	a := newTestApp()
	a.screen = screenHome
	a.errText = "测试错误"

	view := a.viewHome()
	if !strings.Contains(view, "测试错误") {
		t.Fatalf("expected error text in view")
	}
	if !strings.Contains(view, "⚠") {
		t.Fatalf("expected warning icon in error banner")
	}
}

// TestHelpBarStyles 测试帮助栏样式
func TestHelpBarStyles(t *testing.T) {
	a := newTestApp()
	a.screen = screenHome

	view := a.viewHome()
	// 帮助栏应该包含快捷键提示
	if !strings.Contains(view, "选择") {
		t.Fatalf("expected '选择' in help bar")
	}
}

// TestDetailViewErrorBanner 测试详情页错误横幅
func TestDetailViewErrorBanner(t *testing.T) {
	a := newTestApp()
	a.screen = screenDetail
	a.detailTab = tabPR
	a.prList = []model.PullRequestItem{{Number: 1, Title: "pr-1", Author: "user"}}
	a.remoteErr = "API 错误"

	view := a.viewDetail()
	if !strings.Contains(view, "API 错误") {
		t.Fatalf("expected remote error in view")
	}
}

// TestRenderTreeLineTruncation 测试文件树行截断
func TestRenderTreeLineTruncation(t *testing.T) {
	a := newTestApp()

	// 创建一个很长的文件名
	longName := strings.Repeat("a", 100)
	tl := treeLine{
		indent:    0,
		name:      longName,
		isDir:     false,
		file:      &FileDiff{Path: longName, IsNew: true},
		fileIndex: 0,
	}

	// 使用很窄的宽度渲染
	width := 20
	line := a.renderTreeLine(tl, width)

	// 确保行宽不超过限制
	if lipgloss.Width(line) > width+10 { // 允许一些样式字符的误差
		t.Fatalf("line width %d exceeds limit %d", lipgloss.Width(line), width)
	}
}

// TestRenderTreeLineSelected 测试文件树选中行渲染
func TestRenderTreeLineSelected(t *testing.T) {
	a := newTestApp()
	a.diffFileIdx = 0

	tl := treeLine{
		indent:    0,
		name:      "test.go",
		isDir:     false,
		file:      &FileDiff{Path: "test.go", IsNew: true},
		fileIndex: 0,
	}

	line := a.renderTreeLine(tl, 30)
	if line == "" {
		t.Fatalf("expected non-empty line for selected file")
	}
}

func TestSelectionStyles(t *testing.T) {
	focused := getSelectionStyle(true)
	unfocused := getSelectionStyle(false)

	if focused.GetBold() != unfocused.GetBold() {
		t.Fatalf("expected both to be bold")
	}
}

func TestBorderStyle(t *testing.T) {
	focused := getBorderStyle(true)
	unfocused := getBorderStyle(false)

	if focused.String() == unfocused.String() {
		t.Fatalf("expected focused and unfocused border styles to differ")
	}
}

func TestColorizeDiff(t *testing.T) {
	raw := `diff --git a/file.go b/file.go
--- a/file.go
+++ b/file.go
+added line
-deleted line
@@ -1,1 +1,1 @@
index 123`
	colored := colorizeDiff(raw)
	if colored == "" {
		t.Fatalf("expected non-empty colored diff")
	}
}

// TestComposeWithFooter 测试底部帮助栏组合
func TestComposeWithFooter(t *testing.T) {
	bodyLines := []string{"line1", "line2"}
	footer := "help text"

	result := composeWithFooter(5, bodyLines, footer)
	if result == "" {
		t.Fatalf("expected non-empty result")
	}

	lines := strings.Split(result, "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}

	// 最后一行应该包含帮助文本
	lastLine := lines[len(lines)-1]
	if !strings.Contains(lastLine, "help") {
		t.Fatalf("expected footer in last line")
	}
}

// TestViewDiffWithFileTree 测试 diff 视图带文件树
func TestViewDiffWithFileTree(t *testing.T) {
	a := newTestApp()
	a.screen = screenDiff
	a.width = 100
	a.height = 30

	// 设置 diff 树
	a.diffTree = &DiffTree{
		Files: []FileDiff{
			{Path: "file1.go", IsNew: true, Content: "+line1\n+line2"},
		},
	}
	a.diffTree.Tree = &DiffNode{
		Name:  "",
		IsDir: true,
		Children: []*DiffNode{
			{Name: "file1.go", IsDir: false, File: &a.diffTree.Files[0]},
		},
	}
	a.diffFileIdx = 0
	a.diffContent = "+line1\n+line2"
	a.diffViewport.SetContent(a.diffContent)

	view := a.viewDiff()
	if view == "" {
		t.Fatalf("expected non-empty diff view")
	}

	// 应该包含文件树和 diff 内容
	if !strings.Contains(view, "Files") {
		t.Fatalf("expected 'Files' panel in view")
	}
}
