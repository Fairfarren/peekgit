package tui

import (
	"context"
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
	screenHome screen = iota
	screenDetail
	screenDiff
)

type tab int

const (
	tabPR tab = iota
	tabIssue
	tabBranch
)

type refreshDoneMsg struct {
	repos []model.RepoStatus
	err   error
}

type remoteLoadedMsg struct {
	repoPath  string
	prs       []model.PullRequestItem
	issues    []model.IssueItem
	branches  []model.BranchInfo
	remoteErr string
}

type diffLoadedMsg struct {
	content string
	err     error
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

	filterMode bool
	filterText string

	screen      screen
	detailTab   tab
	detailPRIdx int
	detailISIdx int
	prList      []model.PullRequestItem
	issues      []model.IssueItem
	branches    []model.BranchInfo
	remoteErr   string

	diffViewport viewport.Model
	diffContent  string
	diffLoading  bool
	diffSearch   string
	searchMode   bool
	searchInput  string
	matches      []int
	matchIdx     int

	loading bool
	errText string
}

func New(cfg config.Config) *App {
	return &App{
		cfg:        cfg,
		git:        gitcli.New(),
		gh:         ghprovider.New(context.Background(), cfg.NoGitHub),
		screen:     screenHome,
		detailTab:  tabPR,
		columns:    1,
		cardWidth:  cardMinWidth,
		loading:    true,
		matchIdx:   -1,
		remoteErr:  "",
		searchMode: false,
	}
}

func (a *App) Init() tea.Cmd {
	return tea.Batch(a.refreshAllCmd(), tickCmd(a.cfg.IntervalSec))
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
		a.diffViewport.Width = max(20, a.width-4)
		a.diffViewport.Height = max(5, a.height-7)
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
		a.branches = m.branches
		a.remoteErr = m.remoteErr
		return a, nil

	case diffLoadedMsg:
		a.diffLoading = false
		if m.err != nil {
			a.errText = m.err.Error()
			return a, nil
		}
		a.errText = ""
		a.diffContent = m.content
		a.diffViewport.SetContent(m.content)
		a.setSearch(a.diffSearch)
		return a, nil

	case tickMsg:
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
		case "q", "ctrl+c":
			return a, tea.Quit
		case "r":
			a.loading = true
			return a, a.refreshAllCmd()
		}
		return a, nil
	}

	switch msg.String() {
	case "q", "ctrl+c":
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
	case "enter":
		a.screen = screenDetail
		a.detailTab = tabPR
		a.detailPRIdx = 0
		a.detailISIdx = 0
		a.remoteErr = ""
		return a, a.loadRemoteCmd(visible[a.selectedIndex])
	}
	return a, nil
}

func (a *App) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	current := a.currentRepo()
	switch msg.String() {
	case "esc", "backspace":
		a.screen = screenHome
		return a, nil
	case "r":
		if current.Name == "" {
			return a, nil
		}
		return a, a.loadRemoteCmd(current)
	case "tab", "right":
		a.detailTab = tab((int(a.detailTab) + 1) % 3)
	case "left", "shift+tab":
		a.detailTab = tab((int(a.detailTab) + 2) % 3)
	case "1":
		a.detailTab = tabPR
	case "2":
		a.detailTab = tabIssue
	case "3":
		a.detailTab = tabBranch
	case "o":
		return a, a.openCurrentURLCmd()
	case "d":
		if a.detailTab == tabPR && len(a.prList) > 0 {
			a.screen = screenDiff
			a.diffLoading = true
			a.diffViewport = viewport.New(max(20, a.width-4), max(5, a.height-7))
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
	if msg.String() == "esc" {
		a.screen = screenDetail
		return a, nil
	}
	if msg.String() == "/" {
		a.searchMode = true
		a.searchInput = ""
		return a, nil
	}
	if msg.String() == "n" {
		a.jumpMatch(1)
		return a, nil
	}
	if msg.String() == "p" {
		a.jumpMatch(-1)
		return a, nil
	}
	var cmd tea.Cmd
	a.diffViewport, cmd = a.diffViewport.Update(msg)
	return a, cmd
}

func (a *App) View() string {
	if a.width == 0 || a.height == 0 {
		return "初始化中..."
	}
	switch a.screen {
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

func (a *App) viewHome() string {
	header := lipgloss.NewStyle().Bold(true).Render("Repo Monitor") + "  (ws: " + a.cfg.Workspace + ")"
	tokenState := "token: unauth"
	if a.gh.Authenticated() {
		tokenState = "token: github ✓"
	}
	help := "↑↓←→/h j k l 选择  Enter 进入  / 过滤  r 刷新  q 退出"
	if a.filterMode {
		help = "过滤中: " + a.filterText + "  (Enter/ESC 结束)"
	}

	lines := []string{header, tokenState, help}
	if a.errText != "" {
		lines = append(lines, "错误: "+a.errText)
	}
	if a.loading {
		lines = append(lines, "刷新中...")
	}

	repos := a.filteredRepos()
	if len(repos) == 0 {
		lines = append(lines, "没有仓库（可调整过滤条件）")
		return strings.Join(lines, "\n")
	}

	rows := make([]string, 0)
	for i := 0; i < len(repos); i += a.columns {
		end := i + a.columns
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

	lines = append(lines, rows...)
	lines = append(lines, "Legend: ✓ synced  ↑ ahead(push)  ↓ behind(pull)  ✎ dirty  ! error")
	return strings.Join(lines, "\n")
}

func (a *App) renderCard(repo model.RepoStatus, selected bool) string {
	borderColor := lipgloss.AdaptiveColor{Light: "#8A8A8A", Dark: "#5F5F5F"}
	cardBg := lipgloss.AdaptiveColor{Light: "#F6F4EE", Dark: "#232327"}
	cardFg := lipgloss.AdaptiveColor{Light: "#2C3743", Dark: "#E5E7EB"}
	if selected {
		borderColor = lipgloss.AdaptiveColor{Light: "#2B6FE8", Dark: "#6EA8FF"}
		cardBg = lipgloss.AdaptiveColor{Light: "#EEF4FF", Dark: "#1D2636"}
	}
	s := lipgloss.NewStyle().
		Width(a.cardWidth).
		Padding(0, 1).
		Foreground(cardFg).
		Background(cardBg).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(borderColor)

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
		errMark = " !" + string(repo.Error)
	}
	dirty := ""
	if repo.Dirty {
		dirty = " ✎"
	}
	content := fmt.Sprintf("%s  %s%s\nbranch: %s\nstatus: %s  PR %s  Issues %s%s",
		repo.Name,
		model.SyncSymbol(repo.Sync, repo.Ahead, repo.Behind),
		dirty,
		repo.Branch,
		model.SyncSymbol(repo.Sync, repo.Ahead, repo.Behind),
		pr,
		issue,
		errMark,
	)
	return s.Render(content)
}

func (a *App) viewDetail() string {
	repo := a.currentRepo()
	header := fmt.Sprintf("%s (branch: %s  status: %s)   [Esc] back", repo.Name, repo.Branch, model.SyncSymbol(repo.Sync, repo.Ahead, repo.Behind))
	tabs := []string{"PRs", "Issues", "Branches"}
	for i := range tabs {
		if tab(i) == a.detailTab {
			tabs[i] = "[" + tabs[i] + "]"
		}
	}
	lines := []string{header, "Tabs: " + strings.Join(tabs, "  ") + "   [r] refresh"}
	if a.remoteErr != "" {
		lines = append(lines, "远端: "+a.remoteErr)
	}

	switch a.detailTab {
	case tabPR:
		lines = append(lines, "↑↓: select  d: diff  o: open")
		if len(a.prList) == 0 {
			lines = append(lines, "暂无 PR")
		} else {
			for i, pr := range a.prList {
				mark := " "
				if i == a.detailPRIdx {
					mark = ">"
				}
				lines = append(lines, fmt.Sprintf("%s #%d %s  %s  updated %s", mark, pr.Number, pr.Title, pr.Author, pr.UpdatedAt.Format("2006-01-02")))
			}
		}
	case tabIssue:
		lines = append(lines, "↑↓: select  o: open")
		if len(a.issues) == 0 {
			lines = append(lines, "暂无 Issues")
		} else {
			for i, is := range a.issues {
				mark := " "
				if i == a.detailISIdx {
					mark = ">"
				}
				lines = append(lines, fmt.Sprintf("%s #%d %s  updated %s", mark, is.Number, is.Title, is.UpdatedAt.Format("2006-01-02")))
			}
		}
	case tabBranch:
		if len(a.branches) == 0 {
			lines = append(lines, "暂无分支")
		} else {
			for _, b := range a.branches {
				cur := " "
				if b.Current {
					cur = "*"
				}
				lines = append(lines, fmt.Sprintf("%s %s  upstream:%s  %s", cur, b.Name, emptyDash(b.Upstream), b.SyncSymbol))
			}
		}
	}
	return strings.Join(lines, "\n")
}

func (a *App) viewDiff() string {
	if a.diffLoading {
		return "加载 diff 中..."
	}
	header := "Diff  [Esc] back  / search  n/p next/prev"
	if a.searchMode {
		header += "\n搜索: " + a.searchInput
	} else if a.diffSearch != "" {
		header += fmt.Sprintf("\n搜索词: %s  命中: %d", a.diffSearch, len(a.matches))
	}
	return header + "\n" + a.diffViewport.View()
}

func (a *App) refreshAllCmd() tea.Cmd {
	return func() tea.Msg {
		repos, err := workspace.ScanRepos(a.cfg.Workspace)
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
				ch <- a.git.RefreshRepo(ctx, r.Name, r.Path)
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
		branches, _ := a.git.ListBranches(ctx, repo.Path, repo.Dirty)

		owner, rname, err := a.git.ParseOwnerRepoFromRemote(ctx, repo.Path)
		if err != nil {
			return remoteLoadedMsg{repoPath: repo.Path, branches: branches, remoteErr: "no-remote"}
		}
		prs, errPR := a.gh.ListPRs(ctx, owner, rname)
		issues, errIssue := a.gh.ListIssues(ctx, owner, rname)
		remoteErr := ""
		if errPR == ghprovider.ErrUnauthenticated || errIssue == ghprovider.ErrUnauthenticated {
			remoteErr = "unauth"
			prs = []model.PullRequestItem{}
			issues = []model.IssueItem{}
		} else if errPR != nil || errIssue != nil {
			remoteErr = "fetch"
		}
		return remoteLoadedMsg{
			repoPath:  repo.Path,
			prs:       prs,
			issues:    issues,
			branches:  branches,
			remoteErr: remoteErr,
		}
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

func (a *App) recomputeGrid() {
	availableWidth := max(20, a.width-4)
	a.columns = computeColumns(availableWidth, cardMinWidth, cardGap)
	a.cardWidth = computeCardWidth(availableWidth, a.columns, cardGap)
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

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
