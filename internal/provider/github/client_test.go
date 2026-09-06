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


func TestNewWithClient(t *testing.T) {
	cNil := NewWithClient(nil)
	if cNil.Authenticated() {
		t.Fatalf("expected nil client to be unauthenticated")
	}
	cValid := NewWithClient(gh.NewClient(nil))
	if !cValid.Authenticated() {
		t.Fatalf("expected client to be authenticated")
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

func TestNewClient(t *testing.T) {
	c := New(context.Background(), true)
	if c == nil || c.Authenticated() {
		t.Fatalf("expected unauthenticated client when noGitHub=true")
	}
}

func TestBuildIssueStateLabel(t *testing.T) {
	cases := []struct {
		state     string
		createdBy bool
		assigned  bool
		want      string
	}{
		{"", true, true, "OPEN | 我创建+指派我"},
		{"open", true, false, "open | 我创建"},
		{"open", false, true, "open | 指派我"},
		{"open", false, false, "open"},
	}
	for _, tc := range cases {
		got := buildIssueStateLabel(tc.state, tc.createdBy, tc.assigned)
		if got != tc.want {
			t.Errorf("buildIssueStateLabel(%q, %v, %v) = %q, want %q", tc.state, tc.createdBy, tc.assigned, got, tc.want)
		}
	}
}

func TestSplitRepoFull(t *testing.T) {
	o, r, ok := splitRepoFull("owner/repo")
	if !ok || o != "owner" || r != "repo" {
		t.Fatalf("splitRepoFull(owner/repo) = %s, %s, %v", o, r, ok)
	}
	_, _, ok = splitRepoFull("invalid")
	if ok {
		t.Fatalf("expected false for invalid repo full name")
	}
}

func TestListPRFilesUnauthenticated(t *testing.T) {
	c := &Client{}
	_, err := c.ListPRFiles(context.Background(), "owner", "repo", 1)
	if err != ErrUnauthenticated {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestPullRequestCIStateInvalidRepo(t *testing.T) {
	c := &Client{}
	state := c.pullRequestCIState(context.Background(), "invalid", 1)
	if state != "UNKNOWN" {
		t.Fatalf("expected UNKNOWN for invalid repo full name, got %s", state)
	}
}

func TestListMyPullRequestsErrors(t *testing.T) {
	c := &Client{}
	_, err := c.ListMyPullRequests(context.Background())
	if err != ErrUnauthenticated {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestListMyIssuesErrors(t *testing.T) {
	c := &Client{}
	_, err := c.ListMyIssues(context.Background())
	if err != ErrUnauthenticated {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestNewWithToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "my-test-token")
	c := New(context.Background(), false)
	if c == nil || !c.Authenticated() {
		t.Fatalf("expected authenticated client")
	}

	_ = os.Unsetenv("GITHUB_TOKEN")
	orig := runGhAuthToken
	runGhAuthToken = func(context.Context) (string, error) {
		return "", errors.New("err")
	}
	defer func() { runGhAuthToken = orig }()
	c2 := New(context.Background(), false)
	if c2 == nil || c2.Authenticated() {
		t.Fatalf("expected unauthenticated client when no token")
	}
}

func TestListPRFilesSuccess(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/o/r/pulls/1/files", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"filename":"a.txt","status":"modified","additions":2,"deletions":1}]`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ghc, _ := gh.NewClient(nil).WithEnterpriseURLs(srv.URL+"/", srv.URL+"/")
	c := &Client{client: ghc, auth: true}
	files, err := c.ListPRFiles(context.Background(), "o", "r", 1)
	if err != nil || len(files) != 1 || files[0].GetFilename() != "a.txt" {
		t.Fatalf("unexpected files=%v, err=%v", files, err)
	}
}

func TestListPRsAndIssuesAndDiffErrors(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/err/err/pulls", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("/api/v3/repos/err/err/issues", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("/api/v3/repos/err/err/pulls/1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ghc, _ := gh.NewClient(nil).WithEnterpriseURLs(srv.URL+"/", srv.URL+"/")
	c := &Client{client: ghc, auth: true, prCache: cache.NewTTLCache[[]model.PullRequestItem](time.Minute), issueCache: cache.NewTTLCache[[]model.IssueItem](time.Minute), diffCache: cache.NewTTLCache[string](time.Minute)}

	if _, err := c.ListPRs(context.Background(), "err", "err"); err == nil {
		t.Fatal("expected error from ListPRs")
	}
	if _, err := c.ListIssues(context.Background(), "err", "err"); err == nil {
		t.Fatal("expected error from ListIssues")
	}
	if _, err := c.PullRequestDiff(context.Background(), "err", "err", 1); err == nil {
		t.Fatal("expected error from PullRequestDiff")
	}
}

func TestMyPRsAndIssuesCacheAndErrors(t *testing.T) {
	// Viewer error
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/user", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ghc, _ := gh.NewClient(nil).WithEnterpriseURLs(srv.URL+"/", srv.URL+"/")
	c := &Client{
		client:       ghc,
		auth:         true,
		viewerCache:  cache.NewTTLCache[string](time.Minute),
		myPRCache:    cache.NewTTLCache[[]model.AccountPullRequestItem](time.Minute),
		myIssueCache: cache.NewTTLCache[[]model.AccountIssueItem](time.Minute),
	}

	if _, err := c.ListMyPullRequests(context.Background()); err == nil {
		t.Fatal("expected error when viewer fails")
	}
	if _, err := c.ListMyIssues(context.Background()); err == nil {
		t.Fatal("expected error when viewer fails")
	}

	// Cache hit
	c.viewerCache.Set("viewer", "cached-user", time.Now())
	c.myPRCache.Set("my-prs:cached-user", []model.AccountPullRequestItem{{Number: 100}}, time.Now())
	c.myIssueCache.Set("my-issues:cached-user", []model.AccountIssueItem{{Number: 200}}, time.Now())

	prs, err := c.ListMyPullRequests(context.Background())
	if err != nil || len(prs) != 1 || prs[0].Number != 100 {
		t.Fatalf("expected cached prs, got %v, %v", prs, err)
	}
	issues, err := c.ListMyIssues(context.Background())
	if err != nil || len(issues) != 1 || issues[0].Number != 200 {
		t.Fatalf("expected cached issues, got %v, %v", issues, err)
	}

	// Search errors
	mux2 := http.NewServeMux()
	mux2.HandleFunc("/api/v3/user", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"login":"user2"}`))
	})
	mux2.HandleFunc("/api/v3/search/issues", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv2 := httptest.NewServer(mux2)
	defer srv2.Close()

	ghc2, _ := gh.NewClient(nil).WithEnterpriseURLs(srv2.URL+"/", srv2.URL+"/")
	c2 := &Client{
		client:       ghc2,
		auth:         true,
		viewerCache:  cache.NewTTLCache[string](time.Minute),
		myPRCache:    cache.NewTTLCache[[]model.AccountPullRequestItem](time.Minute),
		myIssueCache: cache.NewTTLCache[[]model.AccountIssueItem](time.Minute),
	}
	if _, err := c2.ListMyPullRequests(context.Background()); err == nil {
		t.Fatal("expected search error")
	}
	if _, err := c2.ListMyIssues(context.Background()); err == nil {
		t.Fatal("expected search error")
	}
}

func TestViewerLoginEmptyAndCIStateEdgeCases(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/user", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"login":""}`))
	})
	mux.HandleFunc("/api/v3/repos/o/r/pulls/1", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"number":1,"head":{"sha":""}}`))
	})
	mux.HandleFunc("/api/v3/repos/o/r/pulls/2", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"number":2,"head":{"sha":"sha2"}}`))
	})
	mux.HandleFunc("/api/v3/repos/o/r/commits/sha2/status", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mux.HandleFunc("/api/v3/repos/o/r/pulls/3", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"number":3,"head":{"sha":"sha3"}}`))
	})
	mux.HandleFunc("/api/v3/repos/o/r/commits/sha3/status", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"state":""}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ghc, _ := gh.NewClient(nil).WithEnterpriseURLs(srv.URL+"/", srv.URL+"/")
	c := &Client{client: ghc, auth: true, viewerCache: cache.NewTTLCache[string](time.Minute)}

	_, err := c.currentViewerLogin(context.Background())
	if err == nil || !strings.Contains(err.Error(), "empty viewer login") {
		t.Fatalf("expected empty viewer login error, got %v", err)
	}

	if st := c.pullRequestCIState(context.Background(), "o/r", 1); st != "UNKNOWN" {
		t.Fatalf("expected UNKNOWN for empty sha, got %s", st)
	}
	if st := c.pullRequestCIState(context.Background(), "o/r", 2); st != "UNKNOWN" {
		t.Fatalf("expected UNKNOWN for status error, got %s", st)
	}
	if st := c.pullRequestCIState(context.Background(), "o/r", 3); st != "UNKNOWN" {
		t.Fatalf("expected UNKNOWN for empty status state, got %s", st)
	}
	if st := c.pullRequestCIState(context.Background(), "o/r", 999); st != "UNKNOWN" {
		t.Fatalf("expected UNKNOWN for 404 PR, got %s", st)
	}
}

func TestIssueLabelNamesAndCollectEdgeCases(t *testing.T) {
	it := &gh.Issue{
		Labels: []*gh.Label{
			{Name: gh.String("")},
			{Name: gh.String("valid")},
		},
	}
	names := issueLabelNames(it)
	if len(names) != 1 || names[0] != "valid" {
		t.Fatalf("unexpected names: %v", names)
	}

	merged := make(map[string]model.AccountIssueItem)
	now := time.Now()
	older := now.Add(-10 * time.Minute)
	newer := now

	it1 := &gh.Issue{
		Number:        gh.Int(1),
		Title:         gh.String("old"),
		RepositoryURL: gh.String("https://api.github.com/repos/o/r"),
		State:         gh.String("open"),
		UpdatedAt:     &gh.Timestamp{Time: older},
		User:          &gh.User{Login: gh.String("me")},
	}
	collectIssue(merged, it1, "me")

	it2 := &gh.Issue{
		Number:        gh.Int(1),
		Title:         gh.String("new"),
		RepositoryURL: gh.String("https://api.github.com/repos/o/r"),
		State:         gh.String("open"),
		UpdatedAt:     &gh.Timestamp{Time: newer},
		User:          &gh.User{Login: gh.String("me")},
	}
	collectIssue(merged, it2, "me")

	if merged["o/r#1"].Title != "new" {
		t.Fatalf("expected title updated to 'new', got %s", merged["o/r#1"].Title)
	}
}

func TestRepositoryFullNameFromURLEdgeCases(t *testing.T) {
	if res := repositoryFullNameFromURL(""); res != "-" {
		t.Fatalf("expected '-', got %q", res)
	}
	if res := repositoryFullNameFromURL("https://github.com/no-repos-prefix"); res != "-" {
		t.Fatalf("expected '-', got %q", res)
	}
	if res := repositoryFullNameFromURL("https://api.github.com/repos/onlyowner"); res != "-" {
		t.Fatalf("expected '-', got %q", res)
	}
}

func TestDefaultRunGhAuthToken(t *testing.T) {
	// Call default runGhAuthToken function
	_, _ = runGhAuthToken(context.Background())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _ = runGhAuthToken(ctx)
}

func TestListPRFilesPagination(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/repos/o/r/pulls/1/files", func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		if page == "2" {
			_, _ = w.Write([]byte(`[{"filename":"b.txt"}]`))
			return
		}
		w.Header().Set("Link", `<http://`+r.Host+`/api/v3/repos/o/r/pulls/1/files?page=2>; rel="next"`)
		_, _ = w.Write([]byte(`[{"filename":"a.txt"}]`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ghc, _ := gh.NewClient(nil).WithEnterpriseURLs(srv.URL+"/", srv.URL+"/")
	c := &Client{client: ghc, auth: true}
	files, err := c.ListPRFiles(context.Background(), "o", "r", 1)
	if err != nil || len(files) != 2 {
		t.Fatalf("expected 2 files from pagination, got %v, %v", files, err)
	}
}

func TestSearchAssigneeError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/search/issues", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if strings.Contains(q, "author:") {
			_, _ = w.Write([]byte(`{"items":[]}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ghc, _ := gh.NewClient(nil).WithEnterpriseURLs(srv.URL+"/", srv.URL+"/")
	c := &Client{client: ghc, auth: true}
	_, _, err := c.searchAuthorAndAssignee(context.Background(), "testuser")
	if err == nil {
		t.Fatal("expected error on assignee search, got nil")
	}
}
