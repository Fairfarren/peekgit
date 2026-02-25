package github

import (
	"context"
	"errors"
	"os"
	"os/exec"
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
	client     *gh.Client
	auth       bool
	prCache    *cache.TTLCache[[]model.PullRequestItem]
	issueCache *cache.TTLCache[[]model.IssueItem]
	diffCache  *cache.TTLCache[string]
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
		client:     gh.NewClient(tc),
		auth:       true,
		prCache:    cache.NewTTLCache[[]model.PullRequestItem](60 * time.Second),
		issueCache: cache.NewTTLCache[[]model.IssueItem](60 * time.Second),
		diffCache:  cache.NewTTLCache[string](60 * time.Second),
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
			Number:    pr.GetNumber(),
			Title:     pr.GetTitle(),
			Author:    pr.GetUser().GetLogin(),
			UpdatedAt: pr.GetUpdatedAt().Time,
			Draft:     pr.GetDraft(),
			HTMLURL:   pr.GetHTMLURL(),
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
	raw, _, err := c.client.PullRequests.GetRaw(ctx, owner, repo, number, gh.RawOptions{Type: gh.Diff})
	if err != nil {
		return "", err
	}
	c.diffCache.Set(cacheKey, raw, time.Now())
	return raw, nil
}
