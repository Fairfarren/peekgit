package tui

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Fairfarren/peekgit/internal/config"
	"github.com/Fairfarren/peekgit/internal/gitcli"
	"github.com/Fairfarren/peekgit/internal/model"
	ghprovider "github.com/Fairfarren/peekgit/internal/provider/github"
	"github.com/Fairfarren/peekgit/internal/workspace"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	cardMinWidth = 44
	cardGap      = 2
)

type screen int

const (
	screenWorkspaces screen = iota
	screenHome
	screenDetail
	screenDiff
)

type tab int

const (
	tabPR tab = iota
	tabIssue
)

type refreshDoneMsg struct {
	repos []model.RepoStatus
	err   error
}

type remoteLoadedMsg struct {
	repoPath  string
	prs       []model.PullRequestItem
	issues    []model.IssueItem
	remoteErr string
	prOpen    *int
	issueOpen *int
}

type diffLoadedMsg struct {
	content string
	err     error
}

type pullDoneMsg struct {
	repoPath string
	err      error
}

type pullAllDoneMsg struct {
	completed int
	failed    int
	lastErr   error
}

type tickMsg time.Time

type App struct {
	cfg    config.Config
	git    *gitcli.CLI
	gh     *ghprovider.Client
	width  int
	height int

	repos         []model.RepoStatus
	selectedIndex int
	columns       int
	cardWidth     int

	workspaces      []string
	workspaceCounts   map[string]int
	workspaceHasUpdate map[string]bool
	workspaceChecking bool
	selectedWsIndex int

	filterMode bool
	filterText string

	screen      screen
	detailTab   tab
	detailPRIdx int
	detailISIdx int
	prList      []model.PullRequestItem
	issues      []model.IssueItem
	remoteErr   string


	diffViewport  viewport.Model
	diffContent   string
	diffLoading   bool
	diffSearch    string
	searchMode    bool
	searchInput   string
	matches       []int
	matchIdx      int

	// Split diff view state
	diffTree        *DiffTree
	diffFileIdx     int
	diffFocusLeft   bool
	diffLeftOffset  int
	loading bool
	errText string
}

func New(cfg config.Config) *App {
	wsKeys := make([]string, 0, len(cfg.Global.Workspaces))
	for k := range cfg.Global.Workspaces {
		wsKeys = append(wsKeys, k)
	}
	sort.Strings(wsKeys)

	// Pre-calculate workspace counts (for display on cards)
	wsCounts := make(map[string]int)
	for _, k := range wsKeys {
		paths := cfg.Global.Workspaces[k]
		repos, err := workspace.ScanRepos(paths)
		if err != nil {
			wsCounts[k] = 0
			continue
		}
		wsCounts[k] = len(repos)
	}

	app := &App{
		cfg:                cfg,
		git:                gitcli.New(),
		gh:                 ghprovider.New(context.Background(), cfg.NoGitHub),
		screen:             screenWorkspaces,
		detailTab:          tabPR,
		columns:            1,
		cardWidth:          cardMinWidth,
		loading:            false, // Not loading initially, wait for enter
		matchIdx:           -1,
		remoteErr:          "",
		searchMode:         false,
		workspaces:         wsKeys,
		workspaceCounts:    wsCounts,
		workspaceHasUpdate: make(map[string]bool),
		diffTree:           &DiffTree{},
		diffFileIdx:        0,
		diffFocusLeft:      true,
		diffLeftOffset:     0,
	}

	return app
}

func (a *App) Init() tea.Cmd {
	// Run initial workspace update check in background
	go a.checkWorkspaceUpdates()
	return tickCmd(a.cfg.IntervalSec)
}

func tickCmd(intervalSec int) tea.Cmd {
	return tea.Tick(time.Duration(intervalSec)*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = m.Width
		a.height = m.Height
		a.recomputeGrid()
		a.diffViewport.Width = max(20, a.width-2)
		a.diffViewport.Height = max(5, a.height-4)
		return a, nil

	case refreshDoneMsg:
		a.loading = false
		if m.err != nil {
			a.errText = m.err.Error()
			return a, nil
		}
		a.errText = ""
		a.repos = m.repos
		if len(a.repos) == 0 {
			a.selectedIndex = 0
		} else if a.selectedIndex >= len(a.repos) {
			a.selectedIndex = len(a.repos) - 1
		}
		a.recomputeGrid()
		return a, nil

	case remoteLoadedMsg:
		if a.currentRepoPath() != m.repoPath {
			return a, nil
		}
		a.prList = m.prs
		a.issues = m.issues
		a.remoteErr = m.remoteErr
		a.updateRepoOpenCounts(m.repoPath, m.prOpen, m.issueOpen)
		return a, nil

	case diffLoadedMsg:
		a.diffLoading = false
		if m.err != nil {
			a.errText = m.err.Error()
			return a, nil
		}
		a.errText = ""
		a.diffContent = m.content
		a.diffTree = ParseDiff(m.content)
		a.diffFileIdx = 0
		a.diffLeftOffset = 0
		a.diffFocusLeft = true
		// Set content for right panel (first file or empty)
		if len(a.diffTree.Files) > 0 {
			a.diffViewport.SetContent(colorizeDiff(a.diffTree.Files[0].Content))
		} else {
			a.diffViewport.SetContent(colorizeDiff(m.content))
		}
		a.setSearch(a.diffSearch)
		return a, nil

	case pullDoneMsg:
		if m.err != nil {
			a.errText = "pull 失败: " + m.err.Error()
		} else {
			a.errText = ""
		}
		return a, a.refreshAllCmd()

	case pullAllDoneMsg:
		if m.failed > 0 && m.lastErr != nil {
			a.errText = fmt.Sprintf("pull 完成: %d 成功, %d 失败 (%s)", m.completed, m.failed, m.lastErr.Error())
		} else {
			a.errText = ""
		}
		return a, a.refreshAllCmd()

	case lazygitDoneMsg:
		if m.err != nil {
			a.errText = fmt.Sprintf("lazygit 执行失败: %v", m.err)
		} else {
			a.errText = ""
		}
		return a, a.refreshAllCmd()

	case tickMsg:
		// Check workspace updates in background on every tick
		go a.checkWorkspaceUpdates()
		if a.screen == screenHome {
			return a, tea.Batch(a.refreshAllCmd(), tickCmd(a.cfg.IntervalSec))
		}
		return a, tickCmd(a.cfg.IntervalSec)

	case tea.KeyMsg:
		if a.searchMode {
			return a.updateSearchInput(m)
		}
		if a.filterMode {
			return a.updateFilterInput(m)
		}
		switch a.screen {
		case screenWorkspaces:
			return a.updateWorkspaces(m)
		case screenHome:
			return a.updateHome(m)
		case screenDetail:
			return a.updateDetail(m)
		case screenDiff:
			return a.updateDiff(m)
		}
	}
	return a, nil
}

func (a *App) updateWorkspaces(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(a.workspaces) == 0 {
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return a, tea.Quit
		}
		return a, nil
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return a, tea.Quit
	case "up", "k":
		a.selectedWsIndex = moveIndex(a.selectedWsIndex, len(a.workspaces), a.columns, "up")
	case "down", "j":
		a.selectedWsIndex = moveIndex(a.selectedWsIndex, len(a.workspaces), a.columns, "down")
	case "left", "h":
		a.selectedWsIndex = moveIndex(a.selectedWsIndex, len(a.workspaces), a.columns, "left")
	case "right", "l":
		a.selectedWsIndex = moveIndex(a.selectedWsIndex, len(a.workspaces), a.columns, "right")
	case " ", "enter":
		a.screen = screenHome
		a.loading = true
		a.selectedIndex = 0
		return a, a.refreshAllCmd()
	}
	return a, nil
}

func (a *App) updateFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.filterMode = false
	case "enter":
		a.filterMode = false
	case "backspace":
		if len(a.filterText) > 0 {
			a.filterText = a.filterText[:len(a.filterText)-1]
		}
	default:
		if msg.Type == tea.KeyRunes {
			a.filterText += string(msg.Runes)
		}
	}
	a.recomputeGrid()
	return a, nil
}

func (a *App) updateSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.searchMode = false
		a.searchInput = ""
	case "enter":
		a.searchMode = false
		a.diffSearch = a.searchInput
		a.setSearch(a.diffSearch)
	case "backspace":
		if len(a.searchInput) > 0 {
			a.searchInput = a.searchInput[:len(a.searchInput)-1]
		}
	default:
		if msg.Type == tea.KeyRunes {
			a.searchInput += string(msg.Runes)
		}
	}
	return a, nil
}

func (a *App) updateHome(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	visible := a.filteredRepos()
	if len(visible) == 0 {
		switch msg.String() {
		case "q", "esc":
			a.screen = screenWorkspaces
			a.filterMode = false
			a.filterText = ""
			return a, nil
		case "ctrl+c":
			return a, tea.Quit
		case "r":
			a.loading = true
			return a, a.refreshAllCmd()
		}
		return a, nil
	}

	switch msg.String() {
	case "q", "esc":
		a.screen = screenWorkspaces
		a.filterMode = false
		a.filterText = ""
		return a, nil
	case "ctrl+c":
		return a, tea.Quit
	case "r":
		a.loading = true
		return a, a.refreshAllCmd()
	case "/":
		a.filterMode = true
		return a, nil
	case "up", "k":
		a.selectedIndex = moveIndex(a.selectedIndex, len(visible), a.columns, "up")
	case "down", "j":
		a.selectedIndex = moveIndex(a.selectedIndex, len(visible), a.columns, "down")
	case "left", "h":
		a.selectedIndex = moveIndex(a.selectedIndex, len(visible), a.columns, "left")
	case "right", "l":
		a.selectedIndex = moveIndex(a.selectedIndex, len(visible), a.columns, "right")
	case " ", "enter":
		a.screen = screenDetail
		a.detailTab = tabPR
		a.detailPRIdx = 0
		a.detailISIdx = 0
		a.remoteErr = ""
		return a, a.loadRemoteCmd(visible[a.selectedIndex])
	case "f":
		if len(visible) > 0 {
			a.loading = true
			return a, a.pullCurrentCmd()
		}
	case "F":
		if len(a.repos) > 0 {
			a.loading = true
			return a, a.pullAllCmd()
		}
	case "g":
		if len(visible) > 0 {
			repo := visible[a.selectedIndex]
			return a, a.runLazygitCmd(repo.Path)
		}
	}
	return a, nil
}

func (a *App) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	current := a.currentRepo()
	switch msg.String() {
	case "q", "backspace":
		a.screen = screenHome
		return a, nil
	case "r":
		if current.Name == "" {
			return a, nil
		}
		return a, a.loadRemoteCmd(current)
	case "tab", "right":
		a.detailTab = tab((int(a.detailTab) + 1) % 2)
	case "left", "shift+tab":
		a.detailTab = tab((int(a.detailTab) + 1) % 2)
	case "1":
		a.detailTab = tabPR
	case "2":
		a.detailTab = tabIssue

	case "o":
		return a, a.openCurrentURLCmd()
	case "d":
		if a.detailTab == tabPR && len(a.prList) > 0 {
			a.screen = screenDiff
			a.diffLoading = true
			a.diffViewport = viewport.New(max(20, a.width-2), max(5, a.height-4))
			return a, a.loadPRDiffCmd(current, a.prList[a.detailPRIdx].Number)
		}
	case "up", "k":
		if a.detailTab == tabPR && a.detailPRIdx > 0 {
			a.detailPRIdx--
		}
		if a.detailTab == tabIssue && a.detailISIdx > 0 {
			a.detailISIdx--
		}
	case "down", "j":
		if a.detailTab == tabPR && a.detailPRIdx < len(a.prList)-1 {
			a.detailPRIdx++
		}
		if a.detailTab == tabIssue && a.detailISIdx < len(a.issues)-1 {
			a.detailISIdx++
		}
	}
	return a, nil
}

func (a *App) updateDiff(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global keys
	switch key {
	case "q":
		a.screen = screenDetail
		return a, nil
	case "tab":
		// Toggle between panels
		a.diffFocusLeft = !a.diffFocusLeft
		return a, nil
	case "right":
		// Switch to right panel
		a.diffFocusLeft = false
		return a, nil
	case "left":
		// Switch to left panel
		a.diffFocusLeft = true
		return a, nil
	}

	// Handle panel-specific keys
	if a.diffFocusLeft {
		// Left panel: file list navigation
		fileCount := len(a.diffTree.Files)
		switch key {
		case "up", "k":
			if a.diffFileIdx > 0 {
				a.diffFileIdx--
				a.updateDiffContent()
			}
		case "down", "j":
			if a.diffFileIdx < fileCount-1 {
				a.diffFileIdx++
				a.updateDiffContent()
			}
		}
	} else {
		// Right panel: diff content scrolling
		var cmd tea.Cmd
		a.diffViewport, cmd = a.diffViewport.Update(msg)
		return a, cmd
	}

	return a, nil
}

// updateDiffContent updates the right panel with the selected file's diff
func (a *App) updateDiffContent() {
	if a.diffTree == nil || len(a.diffTree.Files) == 0 {
		return
	}

	file := a.diffTree.GetFileByIndex(a.diffFileIdx)
	if file != nil {
		a.diffViewport.SetContent(colorizeDiff(file.Content))
	} else {
		// Fallback to full diff
		a.diffViewport.SetContent(colorizeDiff(a.diffContent))
	}
}

func (a *App) View() string {
	if a.width == 0 || a.height == 0 {
		return "初始化中..."
	}
	switch a.screen {
	case screenWorkspaces:
		return a.viewWorkspaces()
	case screenHome:
		return a.viewHome()
	case screenDetail:
		return a.viewDetail()
	case screenDiff:
		return a.viewDiff()
	default:
		return ""
	}
}

func (a *App) viewWorkspaces() string {
	header := titleStyle.Render("Repo Monitor - Workspaces")
	help := helpStyle.Render("↑↓←→/h j k l 选择  Space/Enter 进入  q 退出")
	columns := max(1, a.columns)

	headerLines := []string{header, ""}

	if len(a.workspaces) == 0 {
		bodyLines := append(headerLines, "无工作区配置，请编辑 ~/.config/peekgit/config.json")
		return composeWithFooter(a.height, bodyLines, help)
	}

	rows := make([]string, 0)
	for i := 0; i < len(a.workspaces); i += columns {
		end := i + columns
		if end > len(a.workspaces) {
			end = len(a.workspaces)
		}
		cards := make([]string, 0, end-i)
		for j := i; j < end; j++ {
			selected := (j == a.selectedWsIndex)
			cards = append(cards, a.renderWorkspaceCard(a.workspaces[j], selected))
		}
		if len(cards) == 1 {
			rows = append(rows, cards[0])
			continue
		}
		segments := make([]string, 0, len(cards)*2-1)
		for idx, card := range cards {
			if idx > 0 {
				segments = append(segments, strings.Repeat(" ", cardGap))
			}
			segments = append(segments, card)
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, segments...))
	}

	// Calculate how many rows we can display
	rowHeight := 1
	if len(rows) > 0 {
		rowHeight = lipgloss.Height(rows[0])
		if rowHeight < 1 {
			rowHeight = 1
		}
	}
	availableHeight := a.height - len(headerLines) - 2 // -2 for footer help text and spacing

	displayRows := availableHeight / rowHeight
	if displayRows < 0 {
		displayRows = 0
	}

	visibleRows := []string{}
	if displayRows > 0 {
		selectedRow := a.selectedWsIndex / columns
		startRow, endRow := calculateScrollWindow(len(rows), selectedRow, displayRows)
		visibleRows = rows[startRow:endRow]
	}

	bodyLines := append(headerLines, visibleRows...)
	bodyLines = append(bodyLines, "")
	return composeWithFooter(a.height, bodyLines, help)
}

func (a *App) renderWorkspaceCard(name string, selected bool) string {
	borderColor := lipgloss.AdaptiveColor{Light: "#C0C0C0", Dark: "#444444"}
	if selected {
		borderColor = lipgloss.AdaptiveColor{Light: "#2B6FE8", Dark: "#6EA8FF"}
	}
	s := lipgloss.NewStyle().
		Width(a.cardWidth).
		Padding(0, 1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor)

	nameStr := cardNameStyle.Render(name)
	countStr := labelDimStyle.Render(fmt.Sprintf("%d repos", a.workspaceCounts[name]))

	// Show checking status or update indicator
	var indicator string
	if a.workspaceChecking {
		indicator = " " + loadingStyle.Render("↻")
	} else if a.workspaceHasUpdate[name] {
		indicator = " " + dirtyStyle.Render("↓")
	}

	return s.Render(nameStr + indicator + "\n" + countStr)
}

func (a *App) viewHome() string {
	wsName := ""
	columns := max(1, a.columns)
	if len(a.workspaces) > 0 && a.selectedWsIndex < len(a.workspaces) {
		wsName = a.workspaces[a.selectedWsIndex]
	}
	header := titleStyle.Render("Repo Monitor")
	if wsName != "" {
		header += "  " + wsPathStyle.Render(fmt.Sprintf("[%s]", wsName))
	}
	tokenState := tokenBadStyle.Render("token: unauth")
	if a.gh.Authenticated() {
		tokenState = tokenOKStyle.Render("token: github ✓")
	}
	help := helpStyle.Render("↑↓←→/h j k l 选择  Space 进入  / 过滤  r 刷新  f pull  F pull全部  g lazygit  q/ESC 返回")
	if a.filterMode {
		help = searchInfoStyle.Render("过滤中: ") + a.filterText + helpStyle.Render("  (Enter/ESC 结束)")
	}

	headerLines := []string{header, tokenState}
	if a.errText != "" {
		headerLines = append(headerLines, errStyle.Render("错误: "+a.errText))
	}
	if a.loading {
		bodyLines := append(headerLines, loadingStyle.Render("刷新中..."), "")
		return composeWithFooter(a.height, bodyLines, help)
	}

	repos := a.filteredRepos()
	if len(repos) == 0 {
		bodyLines := append(headerLines, "没有仓库（可调整过滤条件）")
		return composeWithFooter(a.height, bodyLines, help)
	}

	rows := make([]string, 0)
	for i := 0; i < len(repos); i += columns {
		end := i + columns
		if end > len(repos) {
			end = len(repos)
		}
		cards := make([]string, 0, end-i)
		for j := i; j < end; j++ {
			selected := (j == a.selectedIndex)
			cards = append(cards, a.renderCard(repos[j], selected))
		}
		if len(cards) == 1 {
			rows = append(rows, cards[0])
			continue
		}
		segments := make([]string, 0, len(cards)*2-1)
		for idx, card := range cards {
			if idx > 0 {
				segments = append(segments, strings.Repeat(" ", cardGap))
			}
			segments = append(segments, card)
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, segments...))
	}

	rowHeight := 1
	if len(rows) > 0 {
		rowHeight = lipgloss.Height(rows[0])
		if rowHeight < 1 {
			rowHeight = 1
		}
	}
	availableHeight := a.height - len(headerLines) - 2 // -2 for footer help text and spacing

	displayRows := availableHeight / rowHeight
	if displayRows < 0 {
		displayRows = 0
	}

	visibleRows := []string{}
	if displayRows > 0 {
		selectedRow := a.selectedIndex / columns
		startRow, endRow := calculateScrollWindow(len(rows), selectedRow, displayRows)
		visibleRows = rows[startRow:endRow]
	}

	bodyLines := append(headerLines, visibleRows...)
	return composeWithFooter(a.height, bodyLines, help)
}

func (a *App) renderCard(repo model.RepoStatus, selected bool) string {
	borderColor := lipgloss.AdaptiveColor{Light: "#C0C0C0", Dark: "#444444"}
	if selected {
		borderColor = lipgloss.AdaptiveColor{Light: "#2B6FE8", Dark: "#6EA8FF"}
	}
	s := lipgloss.NewStyle().
		Width(a.cardWidth).
		Padding(0, 1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor)

	nameStr := cardNameStyle.Render(repo.Name)
	syncStr := renderSyncColored(repo.Sync, repo.Ahead, repo.Behind)
	dirtyStr := ""
	if repo.Dirty {
		dirtyStr = dirtyStyle.Render(" ✎")
	}

	pr := "-"
	if repo.PROpen != nil {
		pr = fmt.Sprintf("%d", *repo.PROpen)
	}
	issue := "-"
	if repo.IssueOpen != nil {
		issue = fmt.Sprintf("%d", *repo.IssueOpen)
	}
	errMark := ""
	if repo.Error != "" {
		errMark = errStyle.Render(" !" + string(repo.Error))
	}

	line1 := nameStr + dirtyStr
	line2 := labelDimStyle.Render("branch: ") + repo.Branch + "  " + syncStr
	line3 := prLabelStyle.Render("PR ") + pr + "  " +
		issueLabelStyle.Render("Issues ") + issue + errMark

	return s.Render(line1 + "\n" + line2 + "\n" + line3)
}

func (a *App) viewDetail() string {
	repo := a.currentRepo()
	header := titleStyle.Render(repo.Name) + " " +
		labelDimStyle.Render("(branch: ") + repo.Branch +
		labelDimStyle.Render("  status: ") + renderSyncColored(repo.Sync, repo.Ahead, repo.Behind) +
		labelDimStyle.Render(")") + "  " + helpStyle.Render("[q] back")

	tabLabels := []string{"PRs", "Issues"}
	tabStrs := make([]string, len(tabLabels))
	for i, label := range tabLabels {
		if tab(i) == a.detailTab {
			tabStrs[i] = tabActiveStyle.Render("[" + label + "]")
		} else {
			tabStrs[i] = tabInactiveStyle.Render(" " + label + " ")
		}
	}
	refreshText := helpStyle.Render("[r] refresh")
	if a.loading {
		refreshText = loadingStyle.Render("刷新中...")
	}
	headerLines := []string{header, strings.Join(tabStrs, "  ") + "   " + refreshText}
	if a.errText != "" {
		headerLines = append(headerLines, errStyle.Render("错误: "+a.errText))
	}
	if a.remoteErr != "" {
		headerLines = append(headerLines, errStyle.Render("远端: "+a.remoteErr))
	}

	var subHelp string
	switch a.detailTab {
	case tabPR:
		subHelp = helpStyle.Render("↑↓: select  d: diff  o: open")
	case tabIssue:
		subHelp = helpStyle.Render("↑↓: select  o: open")
	}

	// Calculate available height for the list
	// -1 for subHelp at bottom
	listHeight := a.height - len(headerLines) - 1
	if listHeight < 0 {
		listHeight = 0
	}

	var listLines []string
	if a.detailTab == tabPR {
		if len(a.prList) == 0 {
			listLines = append(listLines, "暂无 PR")
		} else {
			start, end := calculateScrollWindow(len(a.prList), a.detailPRIdx, listHeight)
			for i := start; i < end; i++ {
				pr := a.prList[i]
				numStr := numberStyle.Render(fmt.Sprintf("#%d", pr.Number))
				authStr := authorStyle.Render(pr.Author)
				dateStr := dateStyle.Render(pr.UpdatedAt.Format("2006-01-02"))
				branchStr := fmt.Sprintf("[%s -> %s]", emptyDash(pr.HeadBranch), emptyDash(pr.BaseBranch))
				if i == a.detailPRIdx {
					listLines = append(listLines, selectedMarkerStyle.Render(">")+" "+numStr+" "+pr.Title+" "+branchStr+"  "+authStr+"  "+dateStr)
				} else {
					listLines = append(listLines, "  "+numStr+" "+pr.Title+" "+branchStr+"  "+authStr+"  "+dateStr)
				}
			}
		}
	} else {
		if len(a.issues) == 0 {
			listLines = append(listLines, "暂无 Issues")
		} else {
			start, end := calculateScrollWindow(len(a.issues), a.detailISIdx, listHeight)
			for i := start; i < end; i++ {
				is := a.issues[i]
				numStr := numberStyle.Render(fmt.Sprintf("#%d", is.Number))
				dateStr := dateStyle.Render(is.UpdatedAt.Format("2006-01-02"))
				if i == a.detailISIdx {
					listLines = append(listLines, selectedMarkerStyle.Render(">")+" "+numStr+" "+is.Title+"  "+dateStr)
				} else {
					listLines = append(listLines, "  "+numStr+" "+is.Title+"  "+dateStr)
				}
			}
		}
	}

	res := append(headerLines, listLines...)
	return composeWithFooter(a.height, res, subHelp)
}

func calculateScrollWindow(itemCount, selectedIdx, height int) (int, int) {
	if itemCount <= height {
		return 0, itemCount
	}
	start := selectedIdx - height/2
	if start < 0 {
		start = 0
	}
	if start+height > itemCount {
		start = itemCount - height
	}
	return start, start + height
}

func (a *App) viewDiff() string {
	if a.diffLoading {
		return loadingStyle.Render(" 加载 diff 中...")
	}

	// Handle tiny height - just show help
	if a.height <= 1 {
		return helpStyle.Render("[q] back  [/] search  [n/p] next/prev")
	}

	// For small screens, use single panel layout (no file tree)
	if a.height < 10 || a.width < 60 {
		return a.viewDiffSimple()
	}

	// Calculate panel dimensions (3:7 ratio)
	leftWidth := a.width * 3 / 10
	if leftWidth < 25 {
		leftWidth = 25
	}
	rightWidth := a.width - leftWidth - 4 // -4 for borders and gap
	if rightWidth < 40 {
		rightWidth = 40
		leftWidth = a.width - rightWidth - 4
		if leftWidth < 25 {
			leftWidth = 25
		}
	}

	// Calculate content height (account for header, borders, help)
	contentHeight := a.height - 4 // header(1) + panel borders(2) + help(1)
	if contentHeight < 3 {
		contentHeight = 3
	}

	// Update viewport dimensions
	a.diffViewport.Width = max(20, rightWidth-2)
	a.diffViewport.Height = contentHeight

	// Get current file info for right panel title
	currentFile := ""
	if a.diffTree != nil && a.diffFileIdx >= 0 && a.diffFileIdx < len(a.diffTree.Files) {
		currentFile = a.diffTree.Files[a.diffFileIdx].Path
	}

	// Build left panel (file tree)
	leftTitle := dirStyle.Render(" Files ")
	leftContent := a.renderFileTree(leftWidth-2, contentHeight-1) // -1 for title
	leftBorder := panelBorderBlur
	if a.diffFocusLeft {
		leftBorder = panelBorderFocus
	}
	leftPanel := lipgloss.NewStyle().
		Width(leftWidth).
		Height(contentHeight).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(leftBorder).
		Render(leftTitle + "\n" + leftContent)

	// Build right panel (diff content)
	rightTitle := " Diff "
	if currentFile != "" {
		// Truncate filename if too long
		displayName := currentFile
		if len(displayName) > rightWidth-10 {
			displayName = "..." + displayName[len(displayName)-(rightWidth-13):]
		}
		rightTitle = diffMetaStyle.Render(" " + displayName + " ")
	}
	rightBorder := panelBorderBlur
	if !a.diffFocusLeft {
		rightBorder = panelBorderFocus
	}
	rightPanel := lipgloss.NewStyle().
		Width(rightWidth).
		Height(contentHeight).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(rightBorder).
		Render(rightTitle + "\n" + a.diffViewport.View())

	// Combine panels with gap
	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	// Build help text with panel indicator
	panelHint := "[Files]"
	if !a.diffFocusLeft {
		panelHint = "[Diff]"
	}
	fileCount := 0
	if a.diffTree != nil {
		fileCount = len(a.diffTree.Files)
	}
	help := helpStyle.Render(fmt.Sprintf("%s %s  ↑↓选择  ←→切换面板  [q]退出  [%d files]",
		panelHint, cursorStyle.Render("●"), fileCount))
	lines := []string{"", panels, help}

	return strings.Join(lines, "\n")
}

// viewDiffSimple renders a simple single-panel diff view for small screens
func (a *App) viewDiffSimple() string {
	header := diffHeaderStyle.Render("Diff")
	help := helpStyle.Render("[q] back")

	headerLines := []string{header}

	// 动态调整 viewport 高度
	vHeight := a.height - len(headerLines) - 1
	if vHeight < 0 {
		vHeight = 0
	}
	if a.diffViewport.Height != vHeight {
		a.diffViewport.Height = vHeight
	}

	bodyLines := append([]string{}, headerLines...)
	if vHeight > 0 {
		content := a.diffViewport.View()
		if content != "" {
			bodyLines = append(bodyLines, strings.Split(content, "\n")...)
		}
	}
	return composeWithFooter(a.height, bodyLines, help)
}

// renderFileTree renders the file tree for the left panel with tree structure
func (a *App) renderFileTree(width, height int) string {
	if a.diffTree == nil || a.diffTree.Tree == nil || len(a.diffTree.Files) == 0 {
		return labelDimStyle.Render(" 无文件变更")
	}

	// Build tree lines with file index tracking
	var treeLines []treeLine
	fileCounter := 0
	a.buildTreeLines(a.diffTree.Tree, 0, &treeLines, &fileCounter)
	totalLines := len(treeLines)

	// Find the actual line index of the selected file in tree
	selectedLineIdx := 0
	for i, tl := range treeLines {
		if !tl.isDir && tl.fileIndex == a.diffFileIdx {
			selectedLineIdx = i
			break
		}
	}

	// Calculate vertical scroll based on selected line position
	if totalLines <= height {
		a.diffLeftOffset = 0
	} else {
		// Keep selected line visible
		if selectedLineIdx < a.diffLeftOffset {
			a.diffLeftOffset = selectedLineIdx
		} else if selectedLineIdx >= a.diffLeftOffset+height {
			a.diffLeftOffset = selectedLineIdx - height + 1
		}
		// Ensure we don't scroll past the end
		if a.diffLeftOffset+height > totalLines {
			a.diffLeftOffset = totalLines - height
		}
		if a.diffLeftOffset < 0 {
			a.diffLeftOffset = 0
		}
	}

	// Get visible range
	end := a.diffLeftOffset + height
	if end > totalLines {
		end = totalLines
	}

	var lines []string
	for i := a.diffLeftOffset; i < end; i++ {
		tl := treeLines[i]
		line := a.renderTreeLine(tl, width)
		lines = append(lines, line)
	}

	// Pad with empty lines if needed
	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// treeLine represents a line in the file tree
type treeLine struct {
	indent    int
	name      string
	isDir     bool
	file      *FileDiff
	fileIndex int // -1 for directories
}

// buildTreeLines recursively builds flat list of tree lines
func (a *App) buildTreeLines(node *DiffNode, indent int, lines *[]treeLine, fileCounter *int) {
	if node == nil {
		return
	}

	if node.IsDir {
		*lines = append(*lines, treeLine{
			indent: indent,
			name:   node.Name,
			isDir:  true,
		})
		for _, child := range node.Children {
			a.buildTreeLines(child, indent+1, lines, fileCounter)
		}
	} else if node.File != nil {
		idx := *fileCounter
		*lines = append(*lines, treeLine{
			indent:    indent,
			name:      node.Name,
			isDir:     false,
			file:      node.File,
			fileIndex: idx,
		})
		*fileCounter++
	}
}

// renderTreeLine renders a single tree line
func (a *App) renderTreeLine(tl treeLine, width int) string {
	indentStr := strings.Repeat("  ", tl.indent)

	if tl.isDir {
		// Directory line - build plain text first
		plainText := indentStr + "📂 " + tl.name + "/"
		// Truncate plain text if too long
		if lipgloss.Width(plainText) > width {
			// Truncate name to fit
			maxNameLen := width - len(indentStr) - 4 // 4 for "📂 " and "/"
			if maxNameLen > 0 && len(tl.name) > maxNameLen {
				plainText = indentStr + "📂 " + tl.name[:maxNameLen-1] + "…/"
			}
		}
		return dirStyle.Render(plainText)
	}

	// File line
	isSelected := (tl.fileIndex == a.diffFileIdx)
	f := tl.file

	// Status indicator
	var statusSquare string
	switch {
	case f.IsNew:
		statusSquare = lipgloss.NewStyle().Foreground(lipgloss.Color("#5FD97F")).Render("■")
	case f.IsDelete:
		statusSquare = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B")).Render("■")
	case f.OldPath != "" && f.OldPath != f.Path:
		statusSquare = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700")).Render("■")
	default:
		statusSquare = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700")).Render("■")
	}

	// Format stats
	statsStr := ""
	if f.AddLines > 0 || f.DelLines > 0 {
		statsStr = fmt.Sprintf(" (+%d/-%d)", f.AddLines, f.DelLines)
	}

	// Build plain text content for width calculation
	plainText := indentStr + "  ■ " + tl.name + statsStr
	
	// Truncate filename if plain text is too long
	if lipgloss.Width(plainText) > width {
		availableForName := width - len(indentStr) - 4 - lipgloss.Width(statsStr)
		if availableForName > 3 && len(tl.name) > availableForName {
			// Truncate name and add ellipsis
			truncatedName := tl.name[:availableForName-1] + "…"
			plainText = indentStr + "  ■ " + truncatedName + statsStr
		} else if availableForName > 0 {
			plainText = plainText[:width]
		}
	}

	// Build styled line
	line := indentStr + "  " + statusSquare + " " + tl.name
	if statsStr != "" {
		line += labelDimStyle.Render(statsStr)
	}
	
	// Re-apply truncation to styled line if needed
	if lipgloss.Width(line) > width {
		availableForName := width - lipgloss.Width(indentStr+"  ") - lipgloss.Width(statsStr) - lipgloss.Width("■ ")
		if availableForName > 3 && len(tl.name) > availableForName {
			truncatedName := tl.name[:availableForName-1] + "…"
			line = indentStr + "  " + statusSquare + " " + truncatedName + labelDimStyle.Render(statsStr)
		}
	}

	// Apply selection style
	if isSelected {
		line = lipgloss.NewStyle().
			Background(lipgloss.AdaptiveColor{Light: "#1A5CCC", Dark: "#1A5CCC"}).
			Foreground(lipgloss.Color("#FFFFFF")).
			Render(line)
	}

	return line
}

func composeWithFooter(height int, bodyLines []string, footer string) string {
	if height <= 0 {
		return ""
	}
	if height == 1 {
		return footer
	}

	maxBodyLines := height - 1
	if len(bodyLines) > maxBodyLines {
		bodyLines = bodyLines[:maxBodyLines]
	}

	lines := append([]string{}, bodyLines...)
	lines = append(lines, footer)
	return strings.Join(lines, "\n")
}

func (a *App) refreshAllCmd() tea.Cmd {
	return func() tea.Msg {
		if len(a.workspaces) == 0 || a.selectedWsIndex >= len(a.workspaces) {
			return refreshDoneMsg{repos: []model.RepoStatus{}}
		}

		wsName := a.workspaces[a.selectedWsIndex]
		paths := a.cfg.Global.Workspaces[wsName]

		repos, err := workspace.ScanRepos(paths)
		if err != nil {
			return refreshDoneMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		sem := make(chan struct{}, a.cfg.Concurrency)
		ch := make(chan model.RepoStatus, len(repos))
		for _, r := range repos {
			r := r
			sem <- struct{}{}
			go func() {
				defer func() { <-sem }()
				status := a.git.RefreshRepo(ctx, r.Name, r.Path)
				if a.gh.Authenticated() && status.Error != model.RepoErrNoRemote && status.Error != model.RepoErrNotARepo {
					owner, rname, err := a.git.ParseOwnerRepoFromRemote(ctx, r.Path)
					if err == nil {
						prs, errPR := a.gh.ListPRs(ctx, owner, rname)
						issues, errIssue := a.gh.ListIssues(ctx, owner, rname)
						if errPR == nil && errIssue == nil {
							prValue := len(prs)
							issueValue := len(issues)
							status.PROpen = &prValue
							status.IssueOpen = &issueValue
						}
					}
				}
				ch <- status
			}()
		}
		for i := 0; i < cap(sem); i++ {
			sem <- struct{}{}
		}

		statuses := make([]model.RepoStatus, 0, len(repos))
		for i := 0; i < len(repos); i++ {
			statuses = append(statuses, <-ch)
		}
		sort.Slice(statuses, func(i int, j int) bool { return statuses[i].Name < statuses[j].Name })
		return refreshDoneMsg{repos: statuses}
	}
}

func (a *App) loadRemoteCmd(repo model.RepoStatus) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		owner, rname, err := a.git.ParseOwnerRepoFromRemote(ctx, repo.Path)
		if err != nil {
			return remoteLoadedMsg{repoPath: repo.Path, remoteErr: "no-remote"}
		}
		prs, errPR := a.gh.ListPRs(ctx, owner, rname)
		issues, errIssue := a.gh.ListIssues(ctx, owner, rname)
		remoteErr := ""
		var prOpen *int
		var issueOpen *int
		if errPR == ghprovider.ErrUnauthenticated || errIssue == ghprovider.ErrUnauthenticated {
			remoteErr = "unauth"
			prs = []model.PullRequestItem{}
			issues = []model.IssueItem{}
		} else if errPR != nil || errIssue != nil {
			remoteErr = "fetch"
		} else {
			prValue := len(prs)
			issueValue := len(issues)
			prOpen = &prValue
			issueOpen = &issueValue
		}
		return remoteLoadedMsg{
			repoPath:  repo.Path,
			prs:       prs,
			issues:    issues,
			remoteErr: remoteErr,
			prOpen:    prOpen,
			issueOpen: issueOpen,
		}
	}
}

func (a *App) updateRepoOpenCounts(repoPath string, prOpen *int, issueOpen *int) {
	for i := range a.repos {
		if a.repos[i].Path != repoPath {
			continue
		}
		a.repos[i].PROpen = prOpen
		a.repos[i].IssueOpen = issueOpen
		return
	}
}

func (a *App) loadPRDiffCmd(repo model.RepoStatus, number int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		owner, rname, err := a.git.ParseOwnerRepoFromRemote(ctx, repo.Path)
		if err != nil {
			return diffLoadedMsg{err: err}
		}
		diff, err := a.gh.PullRequestDiff(ctx, owner, rname, number)
		if err != nil {
			return diffLoadedMsg{err: err}
		}
		return diffLoadedMsg{content: diff}
	}
}

func (a *App) openCurrentURLCmd() tea.Cmd {
	return func() tea.Msg {
		url := ""
		if a.detailTab == tabPR && len(a.prList) > 0 {
			url = a.prList[a.detailPRIdx].HTMLURL
		}
		if a.detailTab == tabIssue && len(a.issues) > 0 {
			url = a.issues[a.detailISIdx].HTMLURL
		}
		if url == "" {
			return nil
		}
		cmd := browserOpenCmd(url)
		_ = cmd.Run()
		return nil
	}
}

func browserOpenCmd(url string) *exec.Cmd {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", url)
	}
	if runtime.GOOS == "linux" {
		return exec.Command("xdg-open", url)
	}
	return exec.Command("cmd", "/c", "start", url)
}

func (a *App) currentRepo() model.RepoStatus {
	filtered := a.filteredRepos()
	if len(filtered) == 0 || a.selectedIndex >= len(filtered) {
		return model.RepoStatus{}
	}
	return filtered[a.selectedIndex]
}

func (a *App) currentRepoPath() string {
	return a.currentRepo().Path
}

func (a *App) filteredRepos() []model.RepoStatus {
	if strings.TrimSpace(a.filterText) == "" {
		return a.repos
	}
	needle := strings.ToLower(strings.TrimSpace(a.filterText))
	out := make([]model.RepoStatus, 0, len(a.repos))
	for _, r := range a.repos {
		if strings.Contains(strings.ToLower(r.Name), needle) {
			out = append(out, r)
		}
	}
	if a.selectedIndex >= len(out) {
		a.selectedIndex = max(0, len(out)-1)
	}
	return out
}

// checkWorkspaceUpdates checks if any workspace has repos that need pull from remote.
// This is a lightweight background check that runs without blocking the UI.
func (a *App) checkWorkspaceUpdates() {
	// Prevent overlapping goroutines
	if a.workspaceChecking {
		return
	}
	a.workspaceChecking = true
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, wsName := range a.workspaces {
		paths := a.cfg.Global.Workspaces[wsName]
		repos, err := workspace.ScanRepos(paths)
		if err != nil || len(repos) == 0 {
			a.workspaceHasUpdate[wsName] = false
			continue
		}

		// Check first repo only for quick check
		// For performance, we'll check up to 3 repos
		maxCheck := 3
		if len(repos) < maxCheck {
			maxCheck = len(repos)
		}

		hasUpdate := false
		for i := 0; i < maxCheck; i++ {
			repoPath := repos[i].Path
			if a.git.HasRemoteUpdate(ctx, repoPath) {
				hasUpdate = true
				break
			}
		}
		a.workspaceHasUpdate[wsName] = hasUpdate
	}
	a.workspaceChecking = false
}

func (a *App) recomputeGrid() {
	availableWidth := max(20, a.width-4)
	a.columns = computeColumns(availableWidth, cardMinWidth, cardGap)
	a.cardWidth = computeCardWidth(availableWidth, a.columns, cardGap)

	if a.screen == screenWorkspaces {
		if len(a.workspaces) == 0 {
			a.selectedWsIndex = 0
			return
		}
		if a.selectedWsIndex >= len(a.workspaces) {
			a.selectedWsIndex = len(a.workspaces) - 1
		}
		return
	}

	filtered := a.filteredRepos()
	if len(filtered) == 0 {
		a.selectedIndex = 0
		return
	}
	if a.selectedIndex >= len(filtered) {
		a.selectedIndex = len(filtered) - 1
	}
}

func (a *App) setSearch(term string) {
	a.matches = a.matches[:0]
	a.matchIdx = -1
	if strings.TrimSpace(term) == "" {
		return
	}
	lines := strings.Split(a.diffContent, "\n")
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), strings.ToLower(term)) {
			a.matches = append(a.matches, i)
		}
	}
	a.jumpMatch(1)
}

func (a *App) jumpMatch(delta int) {
	if len(a.matches) == 0 {
		return
	}
	a.matchIdx = (a.matchIdx + delta + len(a.matches)) % len(a.matches)
	a.diffViewport.SetYOffset(a.matches[a.matchIdx])
}

func emptyDash(v string) string {
	if strings.TrimSpace(v) == "" {
		return "—"
	}
	return v
}

func (a *App) pullCurrentCmd() tea.Cmd {
	return func() tea.Msg {
		repo := a.currentRepo()
		if repo.Path == "" {
			return pullDoneMsg{err: errors.New("no repo selected")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := a.git.Pull(ctx, repo.Path)
		return pullDoneMsg{repoPath: repo.Path, err: err}
	}
}

func (a *App) pullAllCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		completed := 0
		failed := 0
		var lastErr error

		for _, repo := range a.repos {
			if err := a.git.Pull(ctx, repo.Path); err != nil {
				failed++
				lastErr = err
			} else {
				completed++
			}
		}
		return pullAllDoneMsg{completed: completed, failed: failed, lastErr: lastErr}
	}
}

func (a *App) runLazygitCmd(repoPath string) tea.Cmd {
	// 检查 lazygit 是否可用
	if _, err := exec.LookPath("lazygit"); err != nil {
		return func() tea.Msg {
			return lazygitDoneMsg{err: fmt.Errorf("lazygit 未安装，请先安装 lazygit: https://github.com/jesseduffield/lazygit")}
		}
	}

	c := exec.Command("lazygit")
	c.Dir = repoPath
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return lazygitDoneMsg{err: err}
	})
}

type lazygitDoneMsg struct {
	err error
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
