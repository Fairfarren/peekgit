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
	screenHome screen = iota
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

	filterMode bool
	filterText string

	screen      screen
	detailTab   tab
	detailPRIdx int
	detailISIdx int
	prList      []model.PullRequestItem
	issues      []model.IssueItem
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
		a.diffViewport.SetContent(colorizeDiff(m.content))
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
	if msg.String() == "q" {
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
	header := titleStyle.Render("Repo Monitor") + "  " + wsPathStyle.Render("(ws: "+a.cfg.Workspace+")")
	tokenState := tokenBadStyle.Render("token: unauth")
	if a.gh.Authenticated() {
		tokenState = tokenOKStyle.Render("token: github ✓")
	}
	help := helpStyle.Render("↑↓←→/h j k l 选择  Space 进入  / 过滤  r 刷新  f pull  F pull全部  g lazygit  q 退出")
	if a.filterMode {
		help = searchInfoStyle.Render("过滤中: ") + a.filterText + helpStyle.Render("  (Enter/ESC 结束)")
	}

	lines := []string{header, tokenState}
	if a.errText != "" {
		lines = append(lines, errStyle.Render("错误: "+a.errText))
	}
	if a.loading {
		lines = append(lines, loadingStyle.Render("刷新中..."))
	}

	repos := a.filteredRepos()
	if len(repos) == 0 && !a.loading {
		lines = append(lines, "没有仓库（可调整过滤条件）")
		lines = append(lines, help)
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
	lines = append(lines, help)
	return strings.Join(lines, "\n")
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
	lines := []string{header, strings.Join(tabStrs, "  ") + "   " + refreshText}
	if a.errText != "" {
		lines = append(lines, errStyle.Render("错误: "+a.errText))
	}
	if a.remoteErr != "" {
		lines = append(lines, errStyle.Render("远端: "+a.remoteErr))
	}

	switch a.detailTab {
	case tabPR:
		lines = append(lines, helpStyle.Render("↑↓: select  d: diff  o: open"))
		if len(a.prList) == 0 {
			lines = append(lines, "暂无 PR")
		} else {
			for i, pr := range a.prList {
				numStr := numberStyle.Render(fmt.Sprintf("#%d", pr.Number))
				authStr := authorStyle.Render(pr.Author)
				dateStr := dateStyle.Render("updated " + pr.UpdatedAt.Format("2006-01-02"))
				if i == a.detailPRIdx {
					lines = append(lines, selectedMarkerStyle.Render(">")+" "+numStr+" "+pr.Title+"  "+authStr+"  "+dateStr)
				} else {
					lines = append(lines, "  "+numStr+" "+pr.Title+"  "+authStr+"  "+dateStr)
				}
			}
		}
	case tabIssue:
		lines = append(lines, helpStyle.Render("↑↓: select  o: open"))
		if len(a.issues) == 0 {
			lines = append(lines, "暂无 Issues")
		} else {
			for i, is := range a.issues {
				numStr := numberStyle.Render(fmt.Sprintf("#%d", is.Number))
				dateStr := dateStyle.Render("updated " + is.UpdatedAt.Format("2006-01-02"))
				if i == a.detailISIdx {
					lines = append(lines, selectedMarkerStyle.Render(">")+" "+numStr+" "+is.Title+"  "+dateStr)
				} else {
					lines = append(lines, "  "+numStr+" "+is.Title+"  "+dateStr)
				}
			}
		}
	}
	return strings.Join(lines, "\n")
}

func (a *App) viewDiff() string {
	if a.diffLoading {
		return loadingStyle.Render("加载 diff 中...")
	}
	header := diffHeaderStyle.Render("Diff") + "  " + helpStyle.Render("[q] back  / search  n/p next/prev")
	if a.searchMode {
		header += "\n" + searchInfoStyle.Render("搜索: ") + a.searchInput
	} else if a.diffSearch != "" {
		header += "\n" + searchInfoStyle.Render(fmt.Sprintf("搜索词: %s  命中: %d", a.diffSearch, len(a.matches)))
	}
	return header + "\n" + a.diffViewport.View()
}

func (a *App) refreshAllCmd() tea.Cmd {
	return func() tea.Msg {
		wsCfg, cfgErr := workspace.LoadConfig(a.cfg.Workspace)
		if cfgErr != nil {
			return refreshDoneMsg{err: cfgErr}
		}
		repos, err := workspace.ScanRepos(a.cfg.Workspace, wsCfg.Repos)
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
