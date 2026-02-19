package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
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
