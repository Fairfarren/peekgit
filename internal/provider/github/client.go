package github

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Fairfarren/peekgit/internal/cache"
	"github.com/Fairfarren/peekgit/internal/model"
	gh "github.com/google/go-github/v57/github"
	"golang.org/x/oauth2"
)

var ErrUnauthenticated = errors.New("unauthenticated")

var runGhAuthToken = func(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", "auth", "token")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

type Client struct {
	client       *gh.Client
	auth         bool
	prCache      *cache.TTLCache[[]model.PullRequestItem]
	issueCache   *cache.TTLCache[[]model.IssueItem]
	diffCache    *cache.TTLCache[string]
	myPRCache    *cache.TTLCache[[]model.AccountPullRequestItem]
	myIssueCache *cache.TTLCache[[]model.AccountIssueItem]
	viewerCache  *cache.TTLCache[string]
}

func New(ctx context.Context, noGitHub bool) *Client {
	if noGitHub {
		return &Client{auth: false}
	}
	token := ResolveToken(ctx)
	if token == "" {
		return &Client{auth: false}
	}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	return &Client{
		client:       gh.NewClient(tc),
		auth:         true,
		prCache:      cache.NewTTLCache[[]model.PullRequestItem](60 * time.Second),
		issueCache:   cache.NewTTLCache[[]model.IssueItem](60 * time.Second),
		diffCache:    cache.NewTTLCache[string](60 * time.Second),
		myPRCache:    cache.NewTTLCache[[]model.AccountPullRequestItem](60 * time.Second),
		myIssueCache: cache.NewTTLCache[[]model.AccountIssueItem](60 * time.Second),
		viewerCache:  cache.NewTTLCache[string](60 * time.Second),
	}
}

func ResolveToken(ctx context.Context) string {
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		return token
	}
	token, err := runGhAuthToken(ctx)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(token)
}

func (c *Client) Authenticated() bool { return c.auth && c.client != nil }

func (c *Client) ListPRs(ctx context.Context, owner string, repo string) ([]model.PullRequestItem, error) {
	if !c.Authenticated() {
		return nil, ErrUnauthenticated
	}
	cacheKey := owner + "/" + repo
	if v, ok := c.prCache.Get(cacheKey, time.Now()); ok {
		return v, nil
	}
	opt := &gh.PullRequestListOptions{State: "open", ListOptions: gh.ListOptions{PerPage: 50}}
	prs, _, err := c.client.PullRequests.List(ctx, owner, repo, opt)
	if err != nil {
		return nil, err
	}
	out := make([]model.PullRequestItem, 0, len(prs))
	for _, pr := range prs {
		out = append(out, model.PullRequestItem{
			Number:     pr.GetNumber(),
			Title:      pr.GetTitle(),
			Author:     pr.GetUser().GetLogin(),
			UpdatedAt:  pr.GetUpdatedAt().Time,
			Draft:      pr.GetDraft(),
			HTMLURL:    pr.GetHTMLURL(),
			HeadBranch: pr.GetHead().GetRef(),
			BaseBranch: pr.GetBase().GetRef(),
		})
	}
	c.prCache.Set(cacheKey, out, time.Now())
	return out, nil
}

func (c *Client) ListIssues(ctx context.Context, owner string, repo string) ([]model.IssueItem, error) {
	if !c.Authenticated() {
		return nil, ErrUnauthenticated
	}
	cacheKey := owner + "/" + repo
	if v, ok := c.issueCache.Get(cacheKey, time.Now()); ok {
		return v, nil
	}
	opt := &gh.IssueListByRepoOptions{State: "open", ListOptions: gh.ListOptions{PerPage: 50}}
	items, _, err := c.client.Issues.ListByRepo(ctx, owner, repo, opt)
	if err != nil {
		return nil, err
	}
	out := make([]model.IssueItem, 0, len(items))
	for _, it := range items {
		if it.IsPullRequest() {
			continue
		}
		labels := make([]string, 0, len(it.Labels))
		for _, l := range it.Labels {
			labels = append(labels, l.GetName())
		}
		out = append(out, model.IssueItem{
			Number:    it.GetNumber(),
			Title:     it.GetTitle(),
			Labels:    labels,
			UpdatedAt: it.GetUpdatedAt().Time,
			HTMLURL:   it.GetHTMLURL(),
			Body:      it.GetBody(),
		})
	}
	c.issueCache.Set(cacheKey, out, time.Now())
	return out, nil
}

func (c *Client) PullRequestDiff(ctx context.Context, owner string, repo string, number int) (string, error) {
	if !c.Authenticated() {
		return "", ErrUnauthenticated
	}
	cacheKey := owner + "/" + repo + "/" + strconv.Itoa(number)
	if v, ok := c.diffCache.Get(cacheKey, time.Now()); ok {
		return v, nil
	}
	raw, resp, err := c.client.PullRequests.GetRaw(ctx, owner, repo, number, gh.RawOptions{Type: gh.Diff})
	if err != nil {
		return "", err
	}
	if resp.StatusCode == 406 {
		return "", errors.New("diff-too-large")
	}
	c.diffCache.Set(cacheKey, raw, time.Now())
	return raw, nil
}

func (c *Client) ListPRFiles(ctx context.Context, owner string, repo string, number int) ([]*gh.CommitFile, error) {
	if !c.Authenticated() {
		return nil, ErrUnauthenticated
	}
	opt := &gh.ListOptions{PerPage: 100}
	var allFiles []*gh.CommitFile
	for {
		files, resp, err := c.client.PullRequests.ListFiles(ctx, owner, repo, number, opt)
		if err != nil {
			return nil, err
		}
		allFiles = append(allFiles, files...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return allFiles, nil
}

func (c *Client) ListMyPullRequests(ctx context.Context) ([]model.AccountPullRequestItem, error) {
	if !c.Authenticated() {
		return nil, ErrUnauthenticated
	}
	login, err := c.currentViewerLogin(ctx)
	if err != nil {
		return nil, err
	}
	cacheKey := "my-prs:" + login
	if v, ok := c.myPRCache.Get(cacheKey, time.Now()); ok {
		return v, nil
	}

	query := "is:pr is:open author:" + login
	opt := &gh.SearchOptions{Sort: "updated", Order: "desc", ListOptions: gh.ListOptions{PerPage: 50}}
	result, _, err := c.client.Search.Issues(ctx, query, opt)
	if err != nil {
		return nil, err
	}

	out := make([]model.AccountPullRequestItem, 0, len(result.Issues))
	for _, it := range result.Issues {
		if it.GetPullRequestLinks() == nil {
			continue
		}
		if !strings.EqualFold(it.GetState(), "open") {
			continue
		}
		repoFull := repositoryFullNameFromURL(it.GetRepositoryURL())
		ciStatus := c.pullRequestCIState(ctx, repoFull, it.GetNumber())
		out = append(out, model.AccountPullRequestItem{
			Number:     it.GetNumber(),
			Title:      it.GetTitle(),
			RepoFull:   repoFull,
			UpdatedAt:  it.GetUpdatedAt().Time,
			HTMLURL:    it.GetHTMLURL(),
			StateLabel: strings.ToUpper(it.GetState()),
			CIStatus:   ciStatus,
		})
	}

	c.myPRCache.Set(cacheKey, out, time.Now())
	return out, nil
}

func (c *Client) ListMyIssues(ctx context.Context) ([]model.AccountIssueItem, error) {
	if !c.Authenticated() {
		return nil, ErrUnauthenticated
	}
	login, err := c.currentViewerLogin(ctx)
	if err != nil {
		return nil, err
	}
	cacheKey := "my-issues:" + login
	if v, ok := c.myIssueCache.Get(cacheKey, time.Now()); ok {
		return v, nil
	}

	queryAuthor := "is:issue is:open author:" + login
	queryAssignee := "is:issue is:open assignee:" + login
	opt := &gh.SearchOptions{Sort: "updated", Order: "desc", ListOptions: gh.ListOptions{PerPage: 50}}
	authorResult, _, err := c.client.Search.Issues(ctx, queryAuthor, opt)
	if err != nil {
		return nil, err
	}
	assigneeResult, _, err := c.client.Search.Issues(ctx, queryAssignee, opt)
	if err != nil {
		return nil, err
	}

	merged := make(map[string]model.AccountIssueItem, len(authorResult.Issues)+len(assigneeResult.Issues))
	collect := func(items []*gh.Issue) {
		for _, it := range items {
			if it.GetPullRequestLinks() != nil {
				continue
			}
			if !strings.EqualFold(it.GetState(), "open") {
				continue
			}

			repoFull := repositoryFullNameFromURL(it.GetRepositoryURL())
			key := repoFull + "#" + strconv.Itoa(it.GetNumber())

			assignedToMe := false
			for _, assignee := range it.Assignees {
				if strings.EqualFold(assignee.GetLogin(), login) {
					assignedToMe = true
					break
				}
			}
			createdByMe := strings.EqualFold(it.GetUser().GetLogin(), login)
			state := strings.ToUpper(it.GetState())

			if existing, ok := merged[key]; ok {
				existing.CreatedByMe = existing.CreatedByMe || createdByMe
				existing.AssignedToMe = existing.AssignedToMe || assignedToMe
				if it.GetUpdatedAt().After(existing.UpdatedAt) {
					existing.UpdatedAt = it.GetUpdatedAt().Time
					existing.Title = it.GetTitle()
					existing.HTMLURL = it.GetHTMLURL()
					existing.StateLabel = state
				}
				existing.StateLabel = buildIssueStateLabel(state, existing.CreatedByMe, existing.AssignedToMe)
				merged[key] = existing
				continue
			}

			merged[key] = model.AccountIssueItem{
				Number:       it.GetNumber(),
				Title:        it.GetTitle(),
				RepoFull:     repoFull,
				UpdatedAt:    it.GetUpdatedAt().Time,
				HTMLURL:      it.GetHTMLURL(),
				StateLabel:   buildIssueStateLabel(state, createdByMe, assignedToMe),
				CreatedByMe:  createdByMe,
				AssignedToMe: assignedToMe,
			}
		}
	}

	collect(authorResult.Issues)
	collect(assigneeResult.Issues)

	out := make([]model.AccountIssueItem, 0, len(merged))
	for _, it := range merged {
		out = append(out, it)
	}
	sort.Slice(out, func(i int, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})

	c.myIssueCache.Set(cacheKey, out, time.Now())
	return out, nil
}

func buildIssueStateLabel(state string, createdByMe bool, assignedToMe bool) string {
	stateLabel := strings.TrimSpace(state)
	if stateLabel == "" {
		stateLabel = "OPEN"
	}
	switch {
	case createdByMe && assignedToMe:
		return stateLabel + " | 我创建+指派我"
	case createdByMe:
		return stateLabel + " | 我创建"
	case assignedToMe:
		return stateLabel + " | 指派我"
	default:
		return stateLabel
	}
}

func (c *Client) pullRequestCIState(ctx context.Context, repoFull string, number int) string {
	owner, repo, ok := splitRepoFull(repoFull)
	if !ok {
		return "UNKNOWN"
	}
	pr, _, err := c.client.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return "UNKNOWN"
	}
	sha := strings.TrimSpace(pr.GetHead().GetSHA())
	if sha == "" {
		return "UNKNOWN"
	}
	status, _, err := c.client.Repositories.GetCombinedStatus(ctx, owner, repo, sha, nil)
	if err != nil {
		return "UNKNOWN"
	}
	state := strings.ToUpper(strings.TrimSpace(status.GetState()))
	if state == "" {
		return "UNKNOWN"
	}
	return state
}

func (c *Client) currentViewerLogin(ctx context.Context) (string, error) {
	if v, ok := c.viewerCache.Get("viewer", time.Now()); ok && strings.TrimSpace(v) != "" {
		return v, nil
	}
	user, _, err := c.client.Users.Get(ctx, "")
	if err != nil {
		return "", err
	}
	login := strings.TrimSpace(user.GetLogin())
	if login == "" {
		return "", errors.New("empty viewer login")
	}
	c.viewerCache.Set("viewer", login, time.Now())
	return login, nil
}

func repositoryFullNameFromURL(repoURL string) string {
	trimmed := strings.TrimSpace(repoURL)
	if trimmed == "" {
		return "-"
	}
	idx := strings.Index(trimmed, "/repos/")
	if idx < 0 {
		return "-"
	}
	tail := strings.Trim(trimmed[idx+len("/repos/"):], "/")
	parts := strings.Split(tail, "/")
	if len(parts) < 2 {
		return "-"
	}
	return parts[0] + "/" + parts[1]
}

func splitRepoFull(repoFull string) (string, string, bool) {
	parts := strings.Split(strings.TrimSpace(repoFull), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
