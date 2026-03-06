package tui

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Fairfarren/peekgit/internal/config"
	"github.com/Fairfarren/peekgit/internal/gitcli"
	"github.com/Fairfarren/peekgit/internal/model"
	ghprovider "github.com/Fairfarren/peekgit/internal/provider/github"
	"github.com/Fairfarren/peekgit/internal/workspace"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	cardMinWidth           = 44
	cardGap                = 2
	configWatchIntervalSec = 2
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

type startTab int

const (
	startTabWorkspace startTab = iota
	startTabPR
	startTabIssue
)

type refreshDoneMsg struct {
	seq   int
	repos []workspace.RepoDir
	err   error
}

type repoRefreshDoneMsg struct {
	seq    int
	status model.RepoStatus
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

type workspaceCheckDoneMsg struct {
	workspace string
	hasUpdate bool
}

type configWatchTickMsg time.Time

type configReloadedMsg struct {
	global config.GlobalConfig
	err    error
}

type accountRemoteLoadedMsg struct {
	prs      []model.AccountPullRequestItem
	items    []model.AccountIssueItem
	prErr    string
	issueErr string
}

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
	refreshSeq    int

	workspaces         []string
	reposWorkspace     string
	workspaceCounts    map[string]int
	workspaceHasUpdate map[string]bool
	workspaceChecking  map[string]bool
	selectedWsIndex    int
	repoRefreshing     map[string]bool
	repoRefreshPending int

	startTab                startTab
	startPRs                []model.AccountPullRequestItem
	startIssues             []model.AccountIssueItem
	startPRIdx              int
	startIssueIdx           int
	startLoading            bool
	startPRErr              string
	startIssueErr           string
	startRefreshNoticeUntil time.Time

	filterMode bool
	filterText string

	screen           screen
	diffSourceScreen screen
	detailTab        tab
	detailPRIdx      int
	detailISIdx      int
	prList           []model.PullRequestItem
	issues           []model.IssueItem
	remoteErr        string

	diffViewport viewport.Model
	diffContent  string
	diffLoading  bool
	diffSearch   string
	searchMode   bool
	searchInput  string
	matches      []int
	matchIdx     int

	// Split diff view state
	diffTree       *DiffTree
	diffFileIdx    int
	diffFocusLeft  bool
	diffLeftOffset int
	loading        bool
	errText        string
	refreshLimiter chan struct{}
	spinner        spinner.Model
}

func New(cfg config.Config) *App {
	wsKeys := sortedWorkspaceKeys(cfg.Global.Workspaces)
	wsCounts := calcWorkspaceCounts(cfg.Global.Workspaces, wsKeys)

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
		workspaceChecking:  make(map[string]bool),
		repoRefreshing:     make(map[string]bool),
		startTab:           startTabWorkspace,
		diffTree:           &DiffTree{},
		diffFileIdx:        0,
		diffFocusLeft:      true,
		diffLeftOffset:     0,
	}

	limiterSize := cfg.Concurrency
	if limiterSize < 1 {
		limiterSize = 1
	}
	app.refreshLimiter = make(chan struct{}, limiterSize)

	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = loadingStyle
	app.spinner = sp

	return app
}

func (a *App) Init() tea.Cmd {
	cmds := []tea.Cmd{tickCmd(a.cfg.IntervalSec), configWatchTickCmd()}
	if len(a.workspaces) > 0 {
		cmds = append(cmds, a.workspaceCheckCmd())
	}
	if a.gh.Authenticated() {
		a.startLoading = true
		cmds = append(cmds, a.loadAccountRemoteCmd())
	}
	return tea.Batch(cmds...)
}

func tickCmd(intervalSec int) tea.Cmd {
	return tea.Tick(time.Duration(intervalSec)*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func configWatchTickCmd() tea.Cmd {
	return tea.Tick(time.Duration(configWatchIntervalSec)*time.Second, func(t time.Time) tea.Msg {
		return configWatchTickMsg(t)
	})
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case spinner.TickMsg:
		if a.loading || a.startLoading || a.diffLoading {
			var cmd tea.Cmd
			a.spinner, cmd = a.spinner.Update(msg)
			return a, cmd
		}
		return a, nil

	case tea.WindowSizeMsg:
		a.width = m.Width
		a.height = m.Height
		a.recomputeGrid()
		a.diffViewport.Width = max(20, a.width-2)
		a.diffViewport.Height = max(5, a.height-4)
		return a, nil

	case refreshDoneMsg:
		if m.err != nil {
			if m.seq != a.refreshSeq {
				return a, nil
			}
			a.loading = false
			a.repoRefreshPending = 0
			a.repoRefreshing = make(map[string]bool)
			a.errText = m.err.Error()
			return a, nil
		}
		if m.seq != a.refreshSeq {
			return a, nil
		}

		a.errText = ""
		repoDirs := dedupeRepoDirsByPath(m.repos)
		existing := make(map[string]model.RepoStatus, len(a.repos))
		for _, repo := range a.repos {
			existing[repo.Path] = repo
		}

		nextRepos := make([]model.RepoStatus, 0, len(repoDirs))
		for _, repo := range repoDirs {
			if prev, ok := existing[repo.Path]; ok {
				prev.Name = repo.Name
				prev.Path = repo.Path
				nextRepos = append(nextRepos, prev)
				continue
			}
			nextRepos = append(nextRepos, model.RepoStatus{Name: repo.Name, Path: repo.Path, Sync: model.SyncUnknown})
		}
		sort.Slice(nextRepos, func(i int, j int) bool { return nextRepos[i].Name < nextRepos[j].Name })
		a.repos = nextRepos

		a.repoRefreshing = make(map[string]bool, len(a.repos))
		a.repoRefreshPending = 0
		cmds := make([]tea.Cmd, 0, len(a.repos))
		for _, repo := range a.repos {
			a.repoRefreshing[repo.Path] = true
			a.repoRefreshPending++
			cmds = append(cmds, a.refreshRepoCmd(m.seq, repo.Name, repo.Path))
		}
		a.loading = a.repoRefreshPending > 0

		if len(a.repos) == 0 {
			a.selectedIndex = 0
		} else if a.selectedIndex >= len(a.repos) {
			a.selectedIndex = len(a.repos) - 1
		}
		a.recomputeGrid()
		if len(cmds) == 0 {
			return a, nil
		}
		return a, tea.Batch(cmds...)

	case repoRefreshDoneMsg:
		if m.seq != a.refreshSeq {
			return a, nil
		}
		a.updateRepoStatus(m.status)
		if a.repoRefreshing[m.status.Path] {
			a.repoRefreshing[m.status.Path] = false
			if a.repoRefreshPending > 0 {
				a.repoRefreshPending--
			}
		}
		if a.repoRefreshPending == 0 {
			a.loading = false
		}
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
		cmds := []tea.Cmd{tickCmd(a.cfg.IntervalSec)}
		if len(a.workspaces) > 0 {
			cmds = append(cmds, a.workspaceCheckCmd())
		}
		if a.screen == screenHome && !a.loading {
			cmds = append(cmds, a.refreshAllCmd())
		}
		return a, tea.Batch(cmds...)

	case configWatchTickMsg:
		return a, tea.Batch(configWatchTickCmd(), a.reloadGlobalConfigCmd())

	case configReloadedMsg:
		if m.err != nil {
			a.errText = "配置文件错误: " + m.err.Error()
			return a, nil
		}
		if reflect.DeepEqual(a.cfg.Global, m.global) {
			return a, nil
		}
		a.applyGlobalConfig(m.global)
		a.errText = ""

		cmds := make([]tea.Cmd, 0, 2)
		if len(a.workspaces) > 0 {
			cmds = append(cmds, a.workspaceCheckCmd())
		}
		if a.screen != screenWorkspaces {
			cmds = append(cmds, a.refreshAllCmd())
		}
		if len(cmds) == 0 {
			return a, nil
		}
		return a, tea.Batch(cmds...)

	case workspaceCheckDoneMsg:
		a.workspaceHasUpdate[m.workspace] = m.hasUpdate
		a.workspaceChecking[m.workspace] = false
		return a, nil

	case accountRemoteLoadedMsg:
		a.startLoading = false
		a.startPRs = m.prs
		a.startIssues = m.items
		a.startPRErr = m.prErr
		a.startIssueErr = m.issueErr
		if a.startPRIdx >= len(a.startPRs) {
			a.startPRIdx = max(0, len(a.startPRs)-1)
		}
		if a.startIssueIdx >= len(a.startIssues) {
			a.startIssueIdx = max(0, len(a.startIssues)-1)
		}
		return a, nil

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
	switch msg.String() {
	case "q", "ctrl+c":
		return a, tea.Quit
	case "tab":
		return a, a.switchStartTab(a.startTab + 1)
	case "1":
		return a, a.switchStartTab(startTabWorkspace)
	case "2":
		return a, a.switchStartTab(startTabPR)
	case "3":
		return a, a.switchStartTab(startTabIssue)
	case "r":
		if (a.startTab == startTabPR || a.startTab == startTabIssue) && a.gh.Authenticated() {
			a.startLoading = true
			a.startPRErr = ""
			a.startIssueErr = ""
			a.startRefreshNoticeUntil = time.Now().Add(2 * time.Second)
			return a, a.loadAccountRemoteCmd()
		}
		return a, nil
	case "left", "h":
		if a.startTab == startTabWorkspace {
			a.selectedWsIndex = moveIndex(a.selectedWsIndex, len(a.workspaces), a.columns, "left")
			return a, nil
		}
		return a, a.switchStartTab(a.startTab - 1)
	case "right", "l":
		if a.startTab == startTabWorkspace {
			a.selectedWsIndex = moveIndex(a.selectedWsIndex, len(a.workspaces), a.columns, "right")
			return a, nil
		}
		return a, a.switchStartTab(a.startTab + 1)
	case "up", "k":
		switch a.startTab {
		case startTabWorkspace:
			a.selectedWsIndex = moveIndex(a.selectedWsIndex, len(a.workspaces), a.columns, "up")
		case startTabPR:
			if a.startPRIdx > 0 {
				a.startPRIdx--
			}
		case startTabIssue:
			if a.startIssueIdx > 0 {
				a.startIssueIdx--
			}
		}
	case "down", "j":
		switch a.startTab {
		case startTabWorkspace:
			a.selectedWsIndex = moveIndex(a.selectedWsIndex, len(a.workspaces), a.columns, "down")
		case startTabPR:
			if a.startPRIdx < len(a.startPRs)-1 {
				a.startPRIdx++
			}
		case startTabIssue:
			if a.startIssueIdx < len(a.startIssues)-1 {
				a.startIssueIdx++
			}
		}
	case "o":
		return a, a.openWorkspaceTabCurrentURLCmd()
	case "d":
		if a.startTab == startTabPR && len(a.startPRs) > 0 {
			a.screen = screenDiff
			a.diffSourceScreen = screenWorkspaces
			a.diffLoading = true
			a.diffViewport = viewport.New(max(20, a.width-2), max(5, a.height-4))
			pr := a.startPRs[a.startPRIdx]
			return a, a.loadAccountPRDiffCmd(pr.RepoFull, pr.Number)
		}
	case " ", "enter":
		if a.startTab != startTabWorkspace {
			return a, nil
		}
		if len(a.workspaces) == 0 {
			return a, nil
		}
		a.screen = screenHome
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
			return a, a.pullCurrentCmd()
		}
	case "F":
		if len(a.repos) > 0 {
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
			a.diffSourceScreen = screenDetail
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

	// Check if we're in simple mode (small screen)
	isSimpleMode := a.height < 10 || a.width < 69

	// Global keys
	switch key {
	case "q":
		a.screen = a.diffSourceScreen
		return a, nil
	case "tab":
		// Toggle between panels (only in split mode)
		if !isSimpleMode {
			a.diffFocusLeft = !a.diffFocusLeft
		}
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

	// In simple mode, always scroll the diff content
	if isSimpleMode {
		var cmd tea.Cmd
		a.diffViewport, cmd = a.diffViewport.Update(msg)
		return a, cmd
	}

	// Handle panel-specific keys (split mode)
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
	tabLabels := []string{"workspace", "pr", "issues"}
	tabStrs := make([]string, len(tabLabels))
	for i, label := range tabLabels {
		if startTab(i) == a.startTab {
			tabStrs[i] = tabActiveStyle.Render("[" + label + "]")
		} else {
			tabStrs[i] = tabInactiveStyle.Render(" " + label + " ")
		}
	}
	tabLine := strings.Join(tabStrs, "  ")

	helpText := HelpKeyStyle.Render("Tab/←→") + HelpDescStyle.Render(" 切换页签  ") +
		HelpKeyStyle.Render("1/2/3") + HelpDescStyle.Render(" 快速切换  ") +
		HelpKeyStyle.Render("↑↓") + HelpDescStyle.Render(" 选择  ") +
		HelpKeyStyle.Render("Enter") + HelpDescStyle.Render(" 进入  ") +
		HelpKeyStyle.Render("q") + HelpDescStyle.Render(" 退出")
	if a.startTab == startTabPR || a.startTab == startTabIssue {
		helpText = HelpKeyStyle.Render("Tab/←→") + HelpDescStyle.Render(" 切换页签  ") +
			HelpKeyStyle.Render("1/2/3") + HelpDescStyle.Render(" 快速切换  ") +
			HelpKeyStyle.Render("↑↓") + HelpDescStyle.Render(" 选择  ") +
			HelpKeyStyle.Render("r") + HelpDescStyle.Render(" 刷新  ")
		if a.startTab == startTabPR {
			helpText += HelpKeyStyle.Render("d") + HelpDescStyle.Render(" diff  ")
		}
		helpText += HelpKeyStyle.Render("o") + HelpDescStyle.Render(" 打开链接  ") +
			HelpKeyStyle.Render("q") + HelpDescStyle.Render(" 退出")
	}
	help := helpText
	columns := max(1, a.columns)

	headerLines := []string{header, tabLine, ""}

	if a.startTab == startTabPR {
		bodyLines := a.renderStartPRLines(headerLines)
		return composeWithFooter(a.height, bodyLines, help)
	}
	if a.startTab == startTabIssue {
		bodyLines := a.renderStartIssueLines(headerLines)
		return composeWithFooter(a.height, bodyLines, help)
	}

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
	style := CardStyle
	if selected {
		style = CardSelectedStyle
	}
	s := style.Width(a.cardWidth)

	nameStr := CardHeaderStyle.Render(name)
	countStr := labelDimStyle.Render(fmt.Sprintf("%d repos", a.workspaceCounts[name]))

	var indicator string
	if a.workspaceChecking[name] {
		indicator = " " + loadingStyle.Render("↜")
	} else if a.workspaceHasUpdate[name] {
		indicator = " " + dirtyStyle.Render("↓")
	}

	return s.Render(nameStr + indicator + "\n" + countStr)
}

func (a *App) renderStartPRLines(headerLines []string) []string {
	if !a.gh.Authenticated() {
		lines := append([]string{}, headerLines...)
		lines = a.appendStartRefreshHint(lines)
		return append(lines, "当前未登录 GitHub，无法显示 PR 列表")
	}
	lines := append([]string{}, headerLines...)
	lines = a.appendStartRefreshHint(lines)
	if a.startPRErr != "" {
		return append(lines, ErrorBannerStyle.Render("⚠ "+a.startPRErr))
	}
	if len(a.startPRs) == 0 {
		if a.startLoading {
			return append(lines, loadingStyle.Render("加载当前账号 PR 中..."))
		}
		return append(lines, "当前账号下暂无 PR")
	}

	listHeight := a.height - len(lines) - 1
	if listHeight < 0 {
		listHeight = 0
	}
	start, end := calculateScrollWindow(len(a.startPRs), a.startPRIdx, listHeight)
	listLines := append([]string{}, lines...)
	for i := start; i < end; i++ {
		pr := a.startPRs[i]
		numStr := numberStyle.Render(fmt.Sprintf("#%d", pr.Number))
		repoStr := wsPathStyle.Render(pr.RepoFull)
		stateStr := labelDimStyle.Render(pr.StateLabel)
		ciStatus := strings.TrimSpace(pr.CIStatus)
		if ciStatus == "" {
			ciStatus = "UNKNOWN"
		}
		ciStr := labelDimStyle.Render("CI:" + ciStatus)
		dateStr := dateStyle.Render(pr.UpdatedAt.Format("2006-01-02"))
		line := numStr + " " + pr.Title + "  [" + repoStr + "]  " + stateStr + "  " + ciStr + "  " + dateStr
		if i == a.startPRIdx {
			line = selectedMarkerStyle.Render(">") + " " + line
		} else {
			line = "  " + line
		}
		listLines = append(listLines, line)
	}
	return listLines
}

func (a *App) renderStartIssueLines(headerLines []string) []string {
	if !a.gh.Authenticated() {
		lines := append([]string{}, headerLines...)
		lines = a.appendStartRefreshHint(lines)
		return append(lines, "当前未登录 GitHub，无法显示 Issues 列表")
	}
	lines := append([]string{}, headerLines...)
	lines = a.appendStartRefreshHint(lines)
	if a.startIssueErr != "" {
		return append(lines, ErrorBannerStyle.Render("⚠ "+a.startIssueErr))
	}
	if len(a.startIssues) == 0 {
		if a.startLoading {
			return append(lines, loadingStyle.Render("加载当前账号 Issues 中..."))
		}
		return append(lines, "当前账号下暂无 Issues（我创建或指派给我）")
	}

	listHeight := a.height - len(lines) - 1
	if listHeight < 0 {
		listHeight = 0
	}
	start, end := calculateScrollWindow(len(a.startIssues), a.startIssueIdx, listHeight)
	listLines := append([]string{}, lines...)
	for i := start; i < end; i++ {
		is := a.startIssues[i]
		numStr := numberStyle.Render(fmt.Sprintf("#%d", is.Number))
		repoStr := wsPathStyle.Render(is.RepoFull)
		stateStr := labelDimStyle.Render(is.StateLabel)
		dateStr := dateStyle.Render(is.UpdatedAt.Format("2006-01-02"))
		line := numStr + " " + is.Title + "  [" + repoStr + "]  " + stateStr + "  " + dateStr
		if i == a.startIssueIdx {
			line = selectedMarkerStyle.Render(">") + " " + line
		} else {
			line = "  " + line
		}
		listLines = append(listLines, line)
	}
	return listLines
}

func (a *App) appendStartRefreshHint(lines []string) []string {
	if a.startLoading {
		return append(lines, a.spinner.View()+" 刷新中...")
	}
	if time.Now().Before(a.startRefreshNoticeUntil) {
		return append(lines, loadingStyle.Render("已触发刷新"))
	}
	return lines
}

func (a *App) switchStartTab(next startTab) tea.Cmd {
	if next < startTabWorkspace {
		next = startTabIssue
	}
	if next > startTabIssue {
		next = startTabWorkspace
	}
	a.startTab = next
	if (next == startTabPR || next == startTabIssue) && a.gh.Authenticated() && len(a.startPRs) == 0 && len(a.startIssues) == 0 && !a.startLoading {
		a.startLoading = true
		return a.loadAccountRemoteCmd()
	}
	return nil
}

func (a *App) loadAccountRemoteCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		prs, errPR := a.gh.ListMyPullRequests(ctx)
		issues, errIssue := a.gh.ListMyIssues(ctx)
		prErrText := ""
		issueErrText := ""
		if errPR == ghprovider.ErrUnauthenticated || errIssue == ghprovider.ErrUnauthenticated {
			prErrText = "unauth"
			issueErrText = "unauth"
			prs = []model.AccountPullRequestItem{}
			issues = []model.AccountIssueItem{}
		} else {
			if errPR != nil {
				prErrText = errPR.Error()
			}
			if errIssue != nil {
				issueErrText = errIssue.Error()
			}
		}
		return accountRemoteLoadedMsg{prs: prs, items: issues, prErr: prErrText, issueErr: issueErrText}
	}
}

func (a *App) openWorkspaceTabCurrentURLCmd() tea.Cmd {
	return func() tea.Msg {
		url := ""
		if a.startTab == startTabPR && len(a.startPRs) > 0 {
			url = a.startPRs[a.startPRIdx].HTMLURL
		}
		if a.startTab == startTabIssue && len(a.startIssues) > 0 {
			url = a.startIssues[a.startIssueIdx].HTMLURL
		}
		if url == "" {
			return nil
		}
		cmd := browserOpenCmd(url)
		_ = cmd.Run()
		return nil
	}
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
	help := HelpKeyStyle.Render("↑↓←→/hjkl") + HelpDescStyle.Render(" 选择  ") +
		HelpKeyStyle.Render("Space") + HelpDescStyle.Render(" 进入  ") +
		HelpKeyStyle.Render("/") + HelpDescStyle.Render(" 过滤  ") +
		HelpKeyStyle.Render("r") + HelpDescStyle.Render(" 刷新  ") +
		HelpKeyStyle.Render("f") + HelpDescStyle.Render(" pull  ") +
		HelpKeyStyle.Render("F") + HelpDescStyle.Render(" pull全部  ") +
		HelpKeyStyle.Render("g") + HelpDescStyle.Render(" lazygit  ") +
		HelpKeyStyle.Render("q/ESC") + HelpDescStyle.Render(" 返回")
	if a.filterMode {
		help = searchInfoStyle.Render("过滤中: ") + a.filterText +
			HelpDescStyle.Render("  (") + HelpKeyStyle.Render("Enter/ESC") + HelpDescStyle.Render(" 结束)")
	}

	headerLines := []string{header, tokenState}
	if a.errText != "" {
		headerLines = append(headerLines, ErrorBannerStyle.Render("⚠ "+a.errText))
	}
	repos := a.filteredRepos()
	if a.loading {
		headerLines = append(headerLines, a.spinner.View()+" 刷新中...")
	}
	if a.loading && len(repos) == 0 {
		bodyLines := append(headerLines, a.spinner.View()+" 刷新中...", "")
		return composeWithFooter(a.height, bodyLines, help)
	}

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
	style := CardStyle
	if selected {
		style = CardSelectedStyle
	}
	s := style.Width(a.cardWidth)

	nameStr := CardHeaderStyle.Render(repo.Name)
	dirtyStr := ""
	if repo.Dirty {
		dirtyStr = TagDirtyStyle.Render(" ✎")
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
	if a.repoRefreshing[repo.Path] {
		line1 += " " + loadingStyle.Render("↻")
	}
	line2 := labelDimStyle.Render("branch: ") + repo.Branch + "  " + renderSyncColored(repo.Sync, repo.Ahead, repo.Behind)
	line3 := TagStyle.Render("PR ") + pr + "  " +
		TagStyle.Render("Issues ") + issue + errMark

	return s.Render(line1 + "\n" + line2 + "\n" + line3)
}

func (a *App) viewDetail() string {
	repo := a.currentRepo()
	header := titleStyle.Render(repo.Name) + " " +
		labelDimStyle.Render("(branch: ") + repo.Branch +
		labelDimStyle.Render("  status: ") + renderSyncColored(repo.Sync, repo.Ahead, repo.Behind) +
		labelDimStyle.Render(")") + "  " + HelpKeyStyle.Render("[q]") + HelpDescStyle.Render(" 返回")

	tabLabels := []string{"PRs", "Issues"}
	tabStrs := make([]string, len(tabLabels))
	for i, label := range tabLabels {
		if tab(i) == a.detailTab {
			tabStrs[i] = tabActiveStyle.Render("[" + label + "]")
		} else {
			tabStrs[i] = tabInactiveStyle.Render(" " + label + " ")
		}
	}
	refreshText := HelpKeyStyle.Render("[r]") + HelpDescStyle.Render(" 刷新")
	if a.loading {
		refreshText = a.spinner.View() + " 刷新中..."
	}
	headerLines := []string{header, strings.Join(tabStrs, "  ") + "   " + refreshText}
	if a.errText != "" {
		headerLines = append(headerLines, ErrorBannerStyle.Render("⚠ "+a.errText))
	}
	if a.remoteErr != "" {
		headerLines = append(headerLines, ErrorBannerStyle.Render("⚠ 远端: "+a.remoteErr))
	}

	var subHelp string
	switch a.detailTab {
	case tabPR:
		subHelp = HelpKeyStyle.Render("↑↓") + HelpDescStyle.Render(" 选择  ") +
			HelpKeyStyle.Render("d") + HelpDescStyle.Render(" diff  ") +
			HelpKeyStyle.Render("o") + HelpDescStyle.Render(" 打开")
	case tabIssue:
		subHelp = HelpKeyStyle.Render("↑↓") + HelpDescStyle.Render(" 选择  ") +
			HelpKeyStyle.Render("o") + HelpDescStyle.Render(" 打开")
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
		return a.spinner.View() + " 加载 diff 中..."
	}

	// Handle tiny height - just show help
	if a.height <= 1 {
		return HelpKeyStyle.Render("[q]") + HelpDescStyle.Render(" 返回")
	}

	// For small screens, use single panel layout (no file tree)
	// Minimum width needed: leftWidth(25) + rightWidth(40) + borders/gap(4) = 69
	if a.height < 10 || a.width < 69 {
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
	leftContent := a.renderFileTree(leftWidth-4, contentHeight-1) // -4 for borders(2) + padding(2), -1 for title
	leftBorder := panelBorderBlur
	if a.diffFocusLeft {
		leftBorder = panelBorderFocus
	}
	leftPanel := lipgloss.NewStyle().
		Width(leftWidth).
		Height(contentHeight).
		Padding(0, 1).
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
		Padding(0, 1).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(rightBorder).
		Render(rightTitle + "\n" + a.diffViewport.View())

	// Combine panels with gap
	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	// Build help text with panel indicator
	panelHint := HelpDescStyle.Render("[")
	if a.diffFocusLeft {
		panelHint += HelpKeyStyle.Render("Files")
	} else {
		panelHint += HelpKeyStyle.Render("Diff")
	}
	panelHint += HelpDescStyle.Render("]")
	fileCount := 0
	if a.diffTree != nil {
		fileCount = len(a.diffTree.Files)
	}
	help := panelHint + HelpDescStyle.Render("  ") +
		HelpKeyStyle.Render("↑↓") + HelpDescStyle.Render("选择  ") +
		HelpKeyStyle.Render("←→") + HelpDescStyle.Render("切换面板  ") +
		HelpDescStyle.Render("[") + HelpKeyStyle.Render("q") + HelpDescStyle.Render("]退出  ") +
		cursorStyle.Render("●") + HelpDescStyle.Render(fmt.Sprintf(" %d files", fileCount))
	lines := []string{"", panels, help}

	return strings.Join(lines, "\n")
}

// viewDiffSimple renders a simple single-panel diff view for small screens
func (a *App) viewDiffSimple() string {
	header := diffHeaderStyle.Render("Diff")
	help := HelpKeyStyle.Render("[q]") + HelpDescStyle.Render(" 返回")

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
	// Scroll when selection reaches 1/3 of visible height from top
	if totalLines <= height {
		a.diffLeftOffset = 0
	} else {
		// Calculate threshold at 1/3 of height
		threshold := height / 3
		if threshold < 1 {
			threshold = 1
		}

		// Scroll up when selection moves above threshold from top
		if selectedLineIdx < a.diffLeftOffset+threshold {
			a.diffLeftOffset = selectedLineIdx - threshold
			if a.diffLeftOffset < 0 {
				a.diffLeftOffset = 0
			}
		}

		// Scroll down when selection moves below threshold from bottom
		if selectedLineIdx >= a.diffLeftOffset+height-threshold {
			a.diffLeftOffset = selectedLineIdx - height + threshold + 1
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
		lines = append(lines, strings.Repeat(" ", width))
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
		plainText := indentStr + "📂 " + tl.name + "/"
		if lipgloss.Width(plainText) > width {
			maxNameLen := width - len(indentStr) - 4
			if maxNameLen > 0 && len(tl.name) > maxNameLen {
				plainText = indentStr + "📂 " + tl.name[:maxNameLen-1] + "…/"
			}
		}
		return TreeDirStyle.Render(plainText)
	}

	isSelected := (tl.fileIndex == a.diffFileIdx)
	f := tl.file

	var statusIcon string
	switch {
	case f.IsNew:
		statusIcon = FileStatusNewStyle.Render("+")
	case f.IsDelete:
		statusIcon = FileStatusDelStyle.Render("-")
	default:
		statusIcon = FileStatusModStyle.Render("~")
	}

	statsStr := ""
	if f.AddLines > 0 || f.DelLines > 0 {
		statsStr = fmt.Sprintf(" (+%d/-%d)", f.AddLines, f.DelLines)
	}

	// Build base line
	line := indentStr + " " + statusIcon + " " + tl.name
	if statsStr != "" {
		line += labelDimStyle.Render(statsStr)
	}

	// Apply truncation if needed
	if lipgloss.Width(line) > width {
		availableForName := width - lipgloss.Width(indentStr+" ") - lipgloss.Width(statsStr) - lipgloss.Width("+ ")
		if availableForName > 3 && len(tl.name) > availableForName {
			truncatedName := tl.name[:availableForName-1] + "…"
			line = indentStr + " " + statusIcon + " " + truncatedName
			if statsStr != "" {
				line += labelDimStyle.Render(statsStr)
			}
		}
	}

	// Apply selection style
	if isSelected {
		line = TreeSelectedStyle.Render(line)
	}

	return line
}

func composeWithFooter(height int, bodyLines []string, footer string) string {
	if height <= 0 {
		return ""
	}

	styledFooter := HelpBarStyle.Render(footer)

	if height == 1 {
		return styledFooter
	}

	maxBodyLines := height - 1
	if len(bodyLines) > maxBodyLines {
		bodyLines = bodyLines[:maxBodyLines]
	}

	lines := append([]string{}, bodyLines...)
	lines = append(lines, styledFooter)
	return strings.Join(lines, "\n")
}

func (a *App) refreshAllCmd() tea.Cmd {
	a.refreshSeq++
	seq := a.refreshSeq
	a.loading = true
	a.repoRefreshing = make(map[string]bool)
	a.repoRefreshPending = 0

	if len(a.workspaces) == 0 || a.selectedWsIndex >= len(a.workspaces) {
		a.repos = []model.RepoStatus{}
		a.reposWorkspace = ""
		a.loading = false
		return nil
	}

	wsName := a.workspaces[a.selectedWsIndex]
	if wsName != a.reposWorkspace {
		a.repos = []model.RepoStatus{}
		a.selectedIndex = 0
	}
	a.reposWorkspace = wsName
	paths := append([]string(nil), a.cfg.Global.Workspaces[wsName]...)

	return func() tea.Msg {
		repos, err := workspace.ScanRepos(paths)
		if err != nil {
			return refreshDoneMsg{seq: seq, err: err}
		}
		return refreshDoneMsg{seq: seq, repos: repos}
	}
}

func (a *App) refreshRepoCmd(seq int, name string, path string) tea.Cmd {
	return func() tea.Msg {
		a.refreshLimiter <- struct{}{}
		defer func() { <-a.refreshLimiter }()

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		status := a.git.RefreshRepo(ctx, name, path)
		if a.gh.Authenticated() && status.Error != model.RepoErrNoRemote && status.Error != model.RepoErrNotARepo {
			owner, rname, err := a.git.ParseOwnerRepoFromRemote(ctx, path)
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

		return repoRefreshDoneMsg{seq: seq, status: status}
	}
}

func (a *App) updateRepoStatus(status model.RepoStatus) {
	for i := range a.repos {
		if a.repos[i].Path != status.Path {
			continue
		}
		a.repos[i] = status
		return
	}
}

func dedupeRepoDirsByPath(repos []workspace.RepoDir) []workspace.RepoDir {
	seen := make(map[string]struct{}, len(repos))
	out := make([]workspace.RepoDir, 0, len(repos))
	for _, repo := range repos {
		if _, ok := seen[repo.Path]; ok {
			continue
		}
		seen[repo.Path] = struct{}{}
		out = append(out, repo)
	}
	return out
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

func (a *App) loadAccountPRDiffCmd(repoFull string, number int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		parts := strings.Split(repoFull, "/")
		if len(parts) != 2 {
			return diffLoadedMsg{err: fmt.Errorf("invalid repo name: %s", repoFull)}
		}
		owner, rname := parts[0], parts[1]
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

func (a *App) workspaceCheckCmd() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(a.workspaces))
	for _, wsName := range a.workspaces {
		if a.workspaceChecking[wsName] {
			continue
		}
		a.workspaceChecking[wsName] = true
		paths := append([]string(nil), a.cfg.Global.Workspaces[wsName]...)
		cmds = append(cmds, a.workspaceCheckOneCmd(wsName, paths))
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (a *App) workspaceCheckOneCmd(wsName string, paths []string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		repos, err := workspace.ScanRepos(paths)
		if err != nil || len(repos) == 0 {
			return workspaceCheckDoneMsg{workspace: wsName, hasUpdate: false}
		}

		// For performance, only sample up to 3 repos per workspace.
		maxCheck := 3
		if len(repos) < maxCheck {
			maxCheck = len(repos)
		}

		updateFound := false
		for i := 0; i < maxCheck; i++ {
			if a.git.HasRemoteUpdate(ctx, repos[i].Path) {
				updateFound = true
				break
			}
		}

		return workspaceCheckDoneMsg{workspace: wsName, hasUpdate: updateFound}
	}
}

func (a *App) reloadGlobalConfigCmd() tea.Cmd {
	return func() tea.Msg {
		global, err := config.LoadGlobalConfig()
		return configReloadedMsg{global: global, err: err}
	}
}

func (a *App) applyGlobalConfig(global config.GlobalConfig) {
	selectedName := ""
	if a.selectedWsIndex >= 0 && a.selectedWsIndex < len(a.workspaces) {
		selectedName = a.workspaces[a.selectedWsIndex]
	}

	a.cfg.Global = global
	a.workspaces = sortedWorkspaceKeys(global.Workspaces)
	a.workspaceCounts = calcWorkspaceCounts(global.Workspaces, a.workspaces)
	a.reconcileWorkspaceRuntimeState(selectedName)
	a.recomputeGrid()
}

func (a *App) reconcileWorkspaceRuntimeState(selectedName string) {
	nextHasUpdate := make(map[string]bool, len(a.workspaces))
	nextChecking := make(map[string]bool, len(a.workspaces))
	for _, ws := range a.workspaces {
		nextHasUpdate[ws] = a.workspaceHasUpdate[ws]
		nextChecking[ws] = false
	}
	a.workspaceHasUpdate = nextHasUpdate
	a.workspaceChecking = nextChecking

	if len(a.workspaces) == 0 {
		a.selectedWsIndex = 0
		a.reposWorkspace = ""
		a.repos = []model.RepoStatus{}
		a.selectedIndex = 0
		a.screen = screenWorkspaces
		return
	}

	if selectedName != "" {
		for i, ws := range a.workspaces {
			if ws == selectedName {
				a.selectedWsIndex = i
				return
			}
		}
	}
	if a.selectedWsIndex >= len(a.workspaces) {
		a.selectedWsIndex = len(a.workspaces) - 1
	}
	if a.selectedWsIndex < 0 {
		a.selectedWsIndex = 0
	}
}

func sortedWorkspaceKeys(ws config.WorkspaceMap) []string {
	keys := make([]string, 0, len(ws))
	for k := range ws {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func calcWorkspaceCounts(ws config.WorkspaceMap, keys []string) map[string]int {
	counts := make(map[string]int, len(keys))
	for _, k := range keys {
		repos, err := workspace.ScanRepos(ws[k])
		if err != nil {
			counts[k] = 0
			continue
		}
		counts[k] = len(repos)
	}
	return counts
}

func (a *App) recomputeGrid() {
	availableWidth := max(20, a.width-4)
	minWidth := getResponsiveCardMinWidth(a.width)
	a.columns = computeColumns(availableWidth, minWidth, cardGap)
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
