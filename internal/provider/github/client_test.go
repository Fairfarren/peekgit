package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Fairfarren/peekgit/internal/cache"
	"github.com/Fairfarren/peekgit/internal/model"
	gh "github.com/google/go-github/v57/github"
)

func TestResolveTokenFromEnv(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "abc")
	orig := runGhAuthToken
	runGhAuthToken = func(context.Context) (string, error) {
		return "from-gh", nil
	}
	defer func() { runGhAuthToken = orig }()

	token := ResolveToken(context.Background())
	if token != "abc" {
		t.Fatalf("token = %s", token)
	}
}

func TestResolveTokenFromGh(t *testing.T) {
	_ = os.Unsetenv("GITHUB_TOKEN")
	orig := runGhAuthToken
	runGhAuthToken = func(context.Context) (string, error) {
		return "from-gh", nil
	}
	defer func() { runGhAuthToken = orig }()

	token := ResolveToken(context.Background())
	if token != "from-gh" {
		t.Fatalf("token = %s", token)
	}
}

func TestUnauthenticatedGuards(t *testing.T) {
	c := &Client{}
	if _, err := c.ListPRs(context.Background(), "a", "b"); err != ErrUnauthenticated {
		t.Fatalf("expected unauth, got %v", err)
	}
	if _, err := c.ListIssues(context.Background(), "a", "b"); err != ErrUnauthenticated {
		t.Fatalf("expected unauth, got %v", err)
	}
	if _, err := c.PullRequestDiff(context.Background(), "a", "b", 1); err != ErrUnauthenticated {
		t.Fatalf("expected unauth, got %v", err)
	}
}

func TestListPRsIssuesAndDiff(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/o/r/pulls", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`[{"number":1,"title":"pr1","user":{"login":"u1"},"updated_at":"2026-01-01T00:00:00Z","html_url":"http://x/pr/1"}]`))
			return
		}
	})
	mux.HandleFunc("/api/v3/repos/o/r/pulls/1", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") == "application/vnd.github.v3.diff" {
			_, _ = w.Write([]byte("diff --git a/a b/a\n"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/api/v3/repos/o/r/issues", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[
			{"number":2,"title":"issue1","labels":[{"name":"bug"}],"updated_at":"2026-01-02T00:00:00Z","html_url":"http://x/i/2"},
			{"number":3,"title":"pr-as-issue","pull_request":{"url":"x"}}
		]`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	apiURL := srv.URL + "/"
	ghc, err := gh.NewClient(nil).WithEnterpriseURLs(apiURL, apiURL)
	if err != nil {
		t.Fatalf("gh client: %v", err)
	}

	c := &Client{
		client:     ghc,
		auth:       true,
		prCache:    cache.NewTTLCache[[]model.PullRequestItem](time.Minute),
		issueCache: cache.NewTTLCache[[]model.IssueItem](time.Minute),
		diffCache:  cache.NewTTLCache[string](time.Minute),
	}

	prs, err := c.ListPRs(context.Background(), "o", "r")
	if err != nil || len(prs) != 1 || prs[0].Number != 1 {
		t.Fatalf("prs err=%v len=%d", err, len(prs))
	}
	issues, err := c.ListIssues(context.Background(), "o", "r")
	if err != nil || len(issues) != 1 || issues[0].Number != 2 {
		t.Fatalf("issues err=%v len=%d", err, len(issues))
	}
	diff, err := c.PullRequestDiff(context.Background(), "o", "r", 1)
	if err != nil || diff == "" {
		t.Fatalf("diff err=%v", err)
	}
}

func TestPullRequestDiffTooLarge(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/o/r/pulls/1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotAcceptable) // 406
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	apiURL := srv.URL + "/"
	ghc, _ := gh.NewClient(nil).WithEnterpriseURLs(apiURL, apiURL)
	c := &Client{client: ghc, auth: true, diffCache: cache.NewTTLCache[string](time.Minute)}

	_, err := c.PullRequestDiff(context.Background(), "o", "r", 1)
	if !errors.Is(err, ErrDiffTooLarge) {
		t.Fatalf("expected ErrDiffTooLarge, got %v", err)
	}
}

func TestListPRFilesErrorWrapping(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/o/r/pulls/1/files", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	apiURL := srv.URL + "/"
	ghc, _ := gh.NewClient(nil).WithEnterpriseURLs(apiURL, apiURL)
	c := &Client{client: ghc, auth: true}

	_, err := c.ListPRFiles(context.Background(), "o", "r", 1)
	if err == nil || !strings.Contains(err.Error(), "list PR files:") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestListPRsUsesCache(t *testing.T) {
	c := &Client{
		client:  gh.NewClient(nil),
		auth:    true,
		prCache: cache.NewTTLCache[[]model.PullRequestItem](time.Minute),
	}
	now := time.Now()
	c.prCache.Set("o/r", []model.PullRequestItem{{Number: 7, Title: "cached"}}, now)

	prs, err := c.ListPRs(context.Background(), "o", "r")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(prs) != 1 || prs[0].Number != 7 {
		t.Fatalf("unexpected prs=%+v", prs)
	}
}

func TestResolveTokenEmptyWhenNoEnvAndGhFail(t *testing.T) {
	_ = os.Unsetenv("GITHUB_TOKEN")
	orig := runGhAuthToken
	runGhAuthToken = func(context.Context) (string, error) {
		return "", context.DeadlineExceeded
	}
	defer func() { runGhAuthToken = orig }()

	if tok := ResolveToken(context.Background()); tok != "" {
		t.Fatalf("token=%s", tok)
	}
}

func TestListIssuesUsesCache(t *testing.T) {
	c := &Client{
		client:     gh.NewClient(nil),
		auth:       true,
		issueCache: cache.NewTTLCache[[]model.IssueItem](time.Minute),
	}
	now := time.Now()
	c.issueCache.Set("o/r", []model.IssueItem{{Number: 9, Title: "cached"}}, now)

	issues, err := c.ListIssues(context.Background(), "o", "r")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(issues) != 1 || issues[0].Number != 9 {
		t.Fatalf("unexpected issues=%+v", issues)
	}
}

func TestDiffUsesCache(t *testing.T) {
	c := &Client{
		client:    gh.NewClient(nil),
		auth:      true,
		diffCache: cache.NewTTLCache[string](time.Minute),
	}
	now := time.Now()
	c.diffCache.Set("o/r/1", "cached-diff", now)

	diff, err := c.PullRequestDiff(context.Background(), "o", "r", 1)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if diff != "cached-diff" {
		t.Fatalf("diff=%s", diff)
	}
}

func TestListMyPullRequestsAndIssues(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/user", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"login":"me"}`))
	})
	mux.HandleFunc("/api/v3/search/issues", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q == "is:pr is:open author:me" {
			_, _ = w.Write([]byte(`{"items":[{"number":11,"title":"my-pr","state":"open","updated_at":"2026-01-03T00:00:00Z","html_url":"http://x/pr/11","repository_url":"https://api.github.com/repos/o/r","pull_request":{"url":"http://x/pr/11"}},{"number":12,"title":"closed-pr","state":"closed","updated_at":"2026-01-06T00:00:00Z","html_url":"http://x/pr/12","repository_url":"https://api.github.com/repos/o/r","pull_request":{"url":"http://x/pr/12"}}]}`))
			return
		}
		if q == "is:issue is:open author:me" {
			_, _ = w.Write([]byte(`{"items":[{"number":21,"title":"my-issue","state":"open","labels":[{"name":"bug"}],"updated_at":"2026-01-04T00:00:00Z","html_url":"http://x/i/21","repository_url":"https://api.github.com/repos/o/r","user":{"login":"me"},"assignees":[{"login":"me"}]},{"number":24,"title":"closed-authored-issue","state":"closed","updated_at":"2026-01-08T00:00:00Z","html_url":"http://x/i/24","repository_url":"https://api.github.com/repos/o/r","user":{"login":"me"},"assignees":[{"login":"me"}]},{"number":23,"title":"pr-should-be-filtered","state":"open","updated_at":"2026-01-05T00:00:00Z","html_url":"http://x/p/23","repository_url":"https://api.github.com/repos/o/r2","user":{"login":"me"},"pull_request":{"url":"x"}}]}`))
			return
		}
		if q == "is:issue is:open assignee:me" {
			_, _ = w.Write([]byte(`{"items":[{"number":21,"title":"my-issue","state":"open","labels":[{"name":"bug"}],"updated_at":"2026-01-04T00:00:00Z","html_url":"http://x/i/21","repository_url":"https://api.github.com/repos/o/r","user":{"login":"me"},"assignees":[{"login":"me"}]},{"number":22,"title":"assigned-issue","state":"open","labels":[{"name":"enhancement"}],"updated_at":"2026-01-05T00:00:00Z","html_url":"http://x/i/22","repository_url":"https://api.github.com/repos/o/r2","user":{"login":"other"},"assignees":[{"login":"me"}]},{"number":25,"title":"closed-assigned-issue","state":"closed","updated_at":"2026-01-09T00:00:00Z","html_url":"http://x/i/25","repository_url":"https://api.github.com/repos/o/r2","user":{"login":"other"},"assignees":[{"login":"me"}]},{"number":23,"title":"pr-should-be-filtered","state":"open","updated_at":"2026-01-05T00:00:00Z","html_url":"http://x/p/23","repository_url":"https://api.github.com/repos/o/r2","user":{"login":"other"},"pull_request":{"url":"x"}}]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/api/v3/repos/o/r/pulls/11", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"number":11,"head":{"sha":"sha11"}}`))
	})
	mux.HandleFunc("/api/v3/repos/o/r/commits/sha11/status", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"state":"success"}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	apiURL := srv.URL + "/"
	ghc, err := gh.NewClient(nil).WithEnterpriseURLs(apiURL, apiURL)
	if err != nil {
		t.Fatalf("gh client: %v", err)
	}

	c := &Client{
		client:       ghc,
		auth:         true,
		myPRCache:    cache.NewTTLCache[[]model.AccountPullRequestItem](time.Minute),
		myIssueCache: cache.NewTTLCache[[]model.AccountIssueItem](time.Minute),
		viewerCache:  cache.NewTTLCache[string](time.Minute),
	}

	prs, err := c.ListMyPullRequests(context.Background())
	if err != nil {
		t.Fatalf("my prs err=%v", err)
	}
	if len(prs) != 1 || prs[0].Number != 11 || prs[0].RepoFull != "o/r" {
		t.Fatalf("unexpected my prs=%+v", prs)
	}
	if prs[0].CIStatus != "SUCCESS" {
		t.Fatalf("expected ci status SUCCESS, got=%s", prs[0].CIStatus)
	}

	issues, err := c.ListMyIssues(context.Background())
	if err != nil {
		t.Fatalf("my issues err=%v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("unexpected my issues len=%d", len(issues))
	}
	if issues[0].Number != 22 || issues[0].CreatedByMe || !issues[0].AssignedToMe {
		t.Fatalf("expected first issue only assigned to me and latest updated: %+v", issues[0])
	}
	if len(issues[0].Labels) != 1 || issues[0].Labels[0] != "enhancement" {
		t.Fatalf("expected labels on first issue, got=%v", issues[0].Labels)
	}
	if issues[1].Number != 21 || !issues[1].CreatedByMe || !issues[1].AssignedToMe {
		t.Fatalf("expected second issue merged as created+assigned by me: %+v", issues[1])
	}
	if len(issues[1].Labels) != 1 || issues[1].Labels[0] != "bug" {
		t.Fatalf("expected labels on merged issue, got=%v", issues[1].Labels)
	}
}

func TestRepositoryFullNameFromURL(t *testing.T) {
	if got := repositoryFullNameFromURL("https://api.github.com/repos/o/r"); got != "o/r" {
		t.Fatalf("repo full name=%s", got)
	}
	if got := repositoryFullNameFromURL("https://api.github.com/repos/o/r/"); got != "o/r" {
		t.Fatalf("repo full name with slash=%s", got)
	}
	if got := repositoryFullNameFromURL("bad"); got != "-" {
		t.Fatalf("expected dash for bad url, got=%s", got)
	}
}
