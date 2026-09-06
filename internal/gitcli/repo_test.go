package gitcli

import (
	"context"
	"errors"
	"testing"

	"github.com/Fairfarren/peekgit/internal/model"
)

type fakeExec struct {
	out map[string]string
	err map[string]error
}

func (f fakeExec) Run(_ context.Context, _ string, args ...string) (string, error) {
	k := key(args...)
	if e, ok := f.err[k]; ok {
		return "", e
	}
	if v, ok := f.out[k]; ok {
		return v, nil
	}
	return "", nil
}

func key(args ...string) string {
	s := ""
	for i, a := range args {
		if i > 0 {
			s += " "
		}
		s += a
	}
	return s
}

func TestParseOwnerRepo(t *testing.T) {
	cases := []struct {
		url   string
		owner string
		repo  string
		ok    bool
	}{
		{"git@github.com:fair/peek.git", "fair", "peek", true},
		{"https://github.com/fair/peek.git", "fair", "peek", true},
		{"https://github.com/fair/peek/", "fair", "peek", true},
		{"ssh://example.com/a/b", "", "", false},
	}

	for _, tc := range cases {
		o, r, err := ParseOwnerRepo(tc.url)
		if tc.ok && err != nil {
			t.Fatalf("url %s err %v", tc.url, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("url %s expected err", tc.url)
		}
		if tc.ok && (o != tc.owner || r != tc.repo) {
			t.Fatalf("got %s/%s", o, r)
		}
	}
}

func TestRefreshRepoSynced(t *testing.T) {
	fx := fakeExec{out: map[string]string{
		key("rev-parse", "--is-inside-work-tree"):                               "true",
		key("symbolic-ref", "--short", "HEAD"):                                  "main",
		key("remote", "get-url", "origin"):                                      "https://github.com/fair/peekgit.git",
		key("fetch", "origin", "--prune", "--quiet"):                            "",
		key("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"): "origin/main",
		key("rev-list", "--left-right", "--count", "HEAD...origin/main"):        "0 0",
		key("status", "--porcelain"):                                            "",
	}}

	cli := NewWithExecutor(fx)
	rs := cli.RefreshRepo(context.Background(), "peek", "/tmp/peek")
	if rs.Branch != "main" {
		t.Fatalf("branch %s", rs.Branch)
	}
	if rs.Sync != model.SyncSynced {
		t.Fatalf("sync %v", rs.Sync)
	}
}

func TestRefreshRepoNotARepo(t *testing.T) {
	fx := fakeExec{
		err: map[string]error{
			key("rev-parse", "--is-inside-work-tree"): errors.New("not a git repository"),
		},
	}
	cli := NewWithExecutor(fx)
	rs := cli.RefreshRepo(context.Background(), "peek", "/tmp/peek")
	if rs.Error != model.RepoErrNotARepo {
		t.Fatalf("expected RepoErrNotARepo, got %v", rs.Error)
	}
}

func TestRefreshRepoNoRemote(t *testing.T) {
	fx := fakeExec{out: map[string]string{
		key("rev-parse", "--is-inside-work-tree"): "true",
		key("symbolic-ref", "--short", "HEAD"):    "main",
		key("status", "--porcelain"):              "",
	}, err: map[string]error{
		key("remote", "get-url", "origin"):                                      errors.New("no remote"),
		key("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"): errors.New("no upstream"),
		key("show-ref", "--verify", "refs/remotes/origin/main"):                 errors.New("missing"),
	}}

	cli := NewWithExecutor(fx)
	rs := cli.RefreshRepo(context.Background(), "peek", "/tmp/peek")
	if rs.Error != model.RepoErrNoRemote {
		t.Fatalf("error %s", rs.Error)
	}
}

func TestRefreshRepoFetchError(t *testing.T) {
	fx := fakeExec{
		out: map[string]string{
			key("rev-parse", "--is-inside-work-tree"):                               "true",
			key("symbolic-ref", "--short", "HEAD"):                                  "main",
			key("remote", "get-url", "origin"):                                      "https://github.com/fair/peekgit.git",
			key("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"): "origin/main",
			key("rev-list", "--left-right", "--count", "HEAD...origin/main"):        "0 0",
			key("status", "--porcelain"):                                            "",
		},
		err: map[string]error{
			key("fetch", "origin", "--prune", "--quiet"): errors.New("network error"),
		},
	}
	cli := NewWithExecutor(fx)
	rs := cli.RefreshRepo(context.Background(), "peek", "/tmp/peek")
	if rs.Error != model.RepoErrFetch {
		t.Fatalf("expected RepoErrFetch, got %v", rs.Error)
	}
}

func TestRefreshRepoAheadBehindError(t *testing.T) {
	fx := fakeExec{
		out: map[string]string{
			key("rev-parse", "--is-inside-work-tree"):                               "true",
			key("symbolic-ref", "--short", "HEAD"):                                  "main",
			key("remote", "get-url", "origin"):                                      "https://github.com/fair/peekgit.git",
			key("fetch", "origin", "--prune", "--quiet"):                            "",
			key("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"): "origin/main",
			key("status", "--porcelain"):                                            "",
		},
		err: map[string]error{
			key("rev-list", "--left-right", "--count", "HEAD...origin/main"): errors.New("rev-list error"),
		},
	}
	cli := NewWithExecutor(fx)
	rs := cli.RefreshRepo(context.Background(), "peek", "/tmp/peek")
	if rs.Sync != model.SyncUnknown {
		t.Fatalf("expected SyncUnknown, got %v", rs.Sync)
	}
}

func TestListBranches(t *testing.T) {
	fx := fakeExec{out: map[string]string{
		key("for-each-ref", "--format=%(refname:short)|%(upstream:short)|%(HEAD)", "refs/heads"): "main|origin/main|*\nfeat|origin/feat| ",
		key("rev-list", "--left-right", "--count", "HEAD...origin/main"):                         "0 0",
		key("rev-list", "--left-right", "--count", "HEAD...origin/feat"):                         "2 1",
	}}

	cli := NewWithExecutor(fx)
	b, err := cli.ListBranches(context.Background(), "/tmp/x", false)
	if err != nil {
		t.Fatalf("list branches failed: %v", err)
	}
	if len(b) != 2 {
		t.Fatalf("len %d", len(b))
	}
	if !b[0].Current {
		t.Fatalf("first should be current")
	}
}

func TestCurrentBranchDetachedFallback(t *testing.T) {
	fx := fakeExec{out: map[string]string{
		key("rev-parse", "--short", "HEAD"): "abc123",
	}, err: map[string]error{
		key("symbolic-ref", "--short", "HEAD"): errors.New("detached"),
	}}

	cli := NewWithExecutor(fx)
	b := cli.currentBranch(context.Background(), "/tmp/x")
	if b != "detached@abc123" {
		t.Fatalf("branch=%s", b)
	}
}

func TestResolveUpstreamFallbackToOriginBranch(t *testing.T) {
	fx := fakeExec{err: map[string]error{
		key("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"): errors.New("none"),
	}, out: map[string]string{
		key("show-ref", "--verify", "refs/remotes/origin/main"): "ok",
	}}
	cli := NewWithExecutor(fx)
	up := cli.resolveUpstream(context.Background(), "/tmp/x", "main")
	if up != "origin/main" {
		t.Fatalf("upstream=%s", up)
	}
}

func TestAheadBehindInvalidOutput(t *testing.T) {
	fx := fakeExec{out: map[string]string{
		key("rev-list", "--left-right", "--count", "HEAD...origin/main"): "x",
	}}
	cli := NewWithExecutor(fx)
	_, _, err := cli.aheadBehind(context.Background(), "/tmp/x", "origin/main")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestClassifySync(t *testing.T) {
	if classifySync(0, 0) != model.SyncSynced {
		t.Fatalf("synced failed")
	}
	if classifySync(1, 0) != model.SyncAhead {
		t.Fatalf("ahead failed")
	}
	if classifySync(0, 1) != model.SyncBehind {
		t.Fatalf("behind failed")
	}
	if classifySync(1, 1) != model.SyncDiverged {
		t.Fatalf("diverged failed")
	}
}

func TestParseOwnerRepoFromRemote(t *testing.T) {
	fx := fakeExec{out: map[string]string{
		key("remote", "get-url", "origin"): "https://github.com/abc/def.git",
	}}
	cli := NewWithExecutor(fx)
	o, r, err := cli.ParseOwnerRepoFromRemote(context.Background(), "/tmp/x")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if o != "abc" || r != "def" {
		t.Fatalf("owner/repo=%s/%s", o, r)
	}
}

func TestListBranchesError(t *testing.T) {
	fx := fakeExec{err: map[string]error{
		key("for-each-ref", "--format=%(refname:short)|%(upstream:short)|%(HEAD)", "refs/heads"): errors.New("boom"),
	}}
	cli := NewWithExecutor(fx)
	_, err := cli.ListBranches(context.Background(), "/tmp/x", false)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestCheckoutBranchAndPull(t *testing.T) {
	fx := fakeExec{out: map[string]string{
		key("checkout", "feat"): "",
		key("pull", "--quiet"): "",
	}}
	cli := NewWithExecutor(fx)
	if err := cli.CheckoutBranch(context.Background(), "/tmp/x", "feat"); err != nil {
		t.Fatalf("checkout failed: %v", err)
	}
	if err := cli.Pull(context.Background(), "/tmp/x"); err != nil {
		t.Fatalf("pull failed: %v", err)
	}
}

func TestHasPendingChanges(t *testing.T) {
	// Case 1: dirty
	fx1 := fakeExec{out: map[string]string{
		key("status", "--porcelain"): "M file.go\n",
	}}
	cli1 := NewWithExecutor(fx1)
	if !cli1.HasPendingChanges(context.Background(), "/tmp/x") {
		t.Fatalf("expected dirty to have pending changes")
	}

	// Case 2: clean but ahead
	fx2 := fakeExec{out: map[string]string{
		key("status", "--porcelain"):                                            "",
		key("symbolic-ref", "--short", "HEAD"):                                  "main",
		key("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"): "origin/main",
		key("fetch", "origin", "--quiet"):                                       "",
		key("rev-list", "--left-right", "--count", "HEAD...origin/main"):        "1 0",
	}}
	cli2 := NewWithExecutor(fx2)
	if !cli2.HasPendingChanges(context.Background(), "/tmp/x") {
		t.Fatalf("expected ahead to have pending changes")
	}

	// Case 3: clean but behind
	fxBehind := fakeExec{out: map[string]string{
		key("status", "--porcelain"):                                            "",
		key("symbolic-ref", "--short", "HEAD"):                                  "main",
		key("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"): "origin/main",
		key("fetch", "origin", "--quiet"):                                       "",
		key("rev-list", "--left-right", "--count", "HEAD...origin/main"):        "0 1",
	}}
	cliBehind := NewWithExecutor(fxBehind)
	if !cliBehind.HasPendingChanges(context.Background(), "/tmp/x") {
		t.Fatalf("expected behind to have pending changes")
	}

	// Case 4: clean and synced
	fx3 := fakeExec{out: map[string]string{
		key("status", "--porcelain"):                                            "",
		key("symbolic-ref", "--short", "HEAD"):                                  "main",
		key("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"): "origin/main",
		key("fetch", "origin", "--quiet"):                                       "",
		key("rev-list", "--left-right", "--count", "HEAD...origin/main"):        "0 0",
	}}
	cli3 := NewWithExecutor(fx3)
	if cli3.HasPendingChanges(context.Background(), "/tmp/x") {
		t.Fatalf("expected synced to have no pending changes")
	}

	// Case 5: fetch error
	fxFetchErr := fakeExec{
		out: map[string]string{
			key("status", "--porcelain"):                                            "",
			key("symbolic-ref", "--short", "HEAD"):                                  "main",
			key("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"): "origin/main",
		},
		err: map[string]error{
			key("fetch", "origin", "--quiet"): errors.New("network error"),
		},
	}
	cliFetchErr := NewWithExecutor(fxFetchErr)
	if cliFetchErr.HasPendingChanges(context.Background(), "/tmp/x") {
		t.Fatalf("expected fetch error to return false")
	}

	// Case 6: aheadBehind error
	fxAheadBehindErr := fakeExec{
		out: map[string]string{
			key("status", "--porcelain"):                                            "",
			key("symbolic-ref", "--short", "HEAD"):                                  "main",
			key("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"): "origin/main",
			key("fetch", "origin", "--quiet"):                                       "",
		},
		err: map[string]error{
			key("rev-list", "--left-right", "--count", "HEAD...origin/main"): errors.New("rev-list error"),
		},
	}
	cliAheadBehindErr := NewWithExecutor(fxAheadBehindErr)
	if cliAheadBehindErr.HasPendingChanges(context.Background(), "/tmp/x") {
		t.Fatalf("expected aheadBehind error to return false")
	}
}

func TestHasRemoteUpdate(t *testing.T) {
	// Case 1: behind
	fx1 := fakeExec{out: map[string]string{
		key("symbolic-ref", "--short", "HEAD"):                                  "main",
		key("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"): "origin/main",
		key("fetch", "origin", "--quiet"):                                       "",
		key("rev-list", "--left-right", "--count", "HEAD...origin/main"):        "0 2",
	}}
	cli1 := NewWithExecutor(fx1)
	if !cli1.HasRemoteUpdate(context.Background(), "/tmp/x") {
		t.Fatalf("expected behind to have remote update")
	}

	// Case 2: synced
	fx2 := fakeExec{out: map[string]string{
		key("symbolic-ref", "--short", "HEAD"):                                  "main",
		key("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"): "origin/main",
		key("fetch", "origin", "--quiet"):                                       "",
		key("rev-list", "--left-right", "--count", "HEAD...origin/main"):        "0 0",
	}}
	cli2 := NewWithExecutor(fx2)
	if cli2.HasRemoteUpdate(context.Background(), "/tmp/x") {
		t.Fatalf("expected synced to have no remote update")
	}
}

func TestNewCLI(t *testing.T) {
	cli := New()
	if cli == nil || cli.exec == nil {
		t.Fatalf("expected valid CLI instance")
	}
}

func TestOSExecutor(t *testing.T) {
	exec := osExecutor{}
	out, err := exec.Run(context.Background(), "", "version")
	if err != nil {
		t.Fatalf("git version failed: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("expected non-empty output")
	}

	// Case: cancelled context produces err with empty stderr
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = exec.Run(ctx, "", "version")
	if err == nil {
		t.Fatal("expected error for canceled context, got nil")
	}

	// Case: git error producing stderr
	_, err = exec.Run(context.Background(), "", "invalid-git-subcommand-12345")
	if err == nil {
		t.Fatal("expected error for invalid command, got nil")
	}
}

func TestCurrentBranchDetachedAndUnknown(t *testing.T) {
	// Case 1: detached HEAD
	fx1 := fakeExec{
		err: map[string]error{
			key("symbolic-ref", "--short", "HEAD"): errors.New("not a symbolic ref"),
		},
		out: map[string]string{
			key("rev-parse", "--short", "HEAD"): "abcdef1",
		},
	}
	cli1 := NewWithExecutor(fx1)
	if b := cli1.currentBranch(context.Background(), "/tmp/x"); b != "detached@abcdef1" {
		t.Fatalf("expected detached@abcdef1, got %s", b)
	}

	// Case 2: rev-parse error -> unknown
	fx2 := fakeExec{
		err: map[string]error{
			key("symbolic-ref", "--short", "HEAD"): errors.New("not a symbolic ref"),
			key("rev-parse", "--short", "HEAD"):    errors.New("rev-parse err"),
		},
	}
	cli2 := NewWithExecutor(fx2)
	if b := cli2.currentBranch(context.Background(), "/tmp/x"); b != "unknown" {
		t.Fatalf("expected unknown, got %s", b)
	}
}

func TestResolveUpstreamEdgeCases(t *testing.T) {
	fx := fakeExec{
		err: map[string]error{
			key("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"): errors.New("no upstream"),
		},
	}
	cli := NewWithExecutor(fx)

	// empty branch
	if up := cli.resolveUpstream(context.Background(), "/tmp/x", ""); up != "" {
		t.Fatalf("expected empty upstream, got %s", up)
	}
	// detached branch
	if up := cli.resolveUpstream(context.Background(), "/tmp/x", "detached@123456"); up != "" {
		t.Fatalf("expected empty upstream, got %s", up)
	}
}

func TestAheadBehindErrorBranches(t *testing.T) {
	cli := NewWithExecutor(fakeExec{
		err: map[string]error{
			key("rev-list", "--left-right", "--count", "HEAD...origin/main"): errors.New("rev-list err"),
		},
	})
	_, _, err := cli.aheadBehind(context.Background(), "/tmp/x", "origin/main")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// parts len != 2
	cliLen := NewWithExecutor(fakeExec{
		out: map[string]string{
			key("rev-list", "--left-right", "--count", "HEAD...origin/main"): "1 2 3",
		},
	})
	_, _, err = cliLen.aheadBehind(context.Background(), "/tmp/x", "origin/main")
	if err == nil {
		t.Fatal("expected error on invalid parts length, got nil")
	}

	// atoi ahead err
	cliAtoi1 := NewWithExecutor(fakeExec{
		out: map[string]string{
			key("rev-list", "--left-right", "--count", "HEAD...origin/main"): "abc 2",
		},
	})
	_, _, err = cliAtoi1.aheadBehind(context.Background(), "/tmp/x", "origin/main")
	if err == nil {
		t.Fatal("expected error on non-int ahead, got nil")
	}

	// atoi behind err
	cliAtoi2 := NewWithExecutor(fakeExec{
		out: map[string]string{
			key("rev-list", "--left-right", "--count", "HEAD...origin/main"): "1 xyz",
		},
	})
	_, _, err = cliAtoi2.aheadBehind(context.Background(), "/tmp/x", "origin/main")
	if err == nil {
		t.Fatal("expected error on non-int behind, got nil")
	}
}

func TestIsDirtyError(t *testing.T) {
	cli := NewWithExecutor(fakeExec{
		err: map[string]error{
			key("status", "--porcelain"): errors.New("status err"),
		},
	})
	if cli.isDirty(context.Background(), "/tmp/x") {
		t.Fatal("expected isDirty to be false on error")
	}
}

func TestParseOwnerRepoFromRemoteError(t *testing.T) {
	cli := NewWithExecutor(fakeExec{
		err: map[string]error{
			key("remote", "get-url", "origin"): errors.New("remote err"),
		},
	})
	_, _, err := cli.ParseOwnerRepoFromRemote(context.Background(), "/tmp/x")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSplitGitHubPath(t *testing.T) {
	if res := splitGitHubPath(""); res != nil {
		t.Fatalf("expected nil, got %v", res)
	}
	if res := splitGitHubPath("///"); res != nil {
		t.Fatalf("expected nil, got %v", res)
	}
	res := splitGitHubPath("/owner/repo/.")
	if len(res) != 2 || res[0] != "owner" || res[1] != "repo" {
		t.Fatalf("unexpected res: %v", res)
	}
}

func TestListBranchesEdgeCases(t *testing.T) {
	// Error branch
	cliErr := NewWithExecutor(fakeExec{
		err: map[string]error{
			key("for-each-ref", "--format=%(refname:short)|%(upstream:short)|%(HEAD)", "refs/heads"): errors.New("ref err"),
		},
	})
	_, err := cliErr.ListBranches(context.Background(), "/tmp/x", false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Empty output
	cliEmpty := NewWithExecutor(fakeExec{
		out: map[string]string{
			key("for-each-ref", "--format=%(refname:short)|%(upstream:short)|%(HEAD)", "refs/heads"): "   \n  ",
		},
	})
	branches, err := cliEmpty.ListBranches(context.Background(), "/tmp/x", false)
	if err != nil || len(branches) != 0 {
		t.Fatalf("expected empty branches, got %v, %v", branches, err)
	}

	// Invalid line format in parseBranchInfo
	cliInvalidLine := NewWithExecutor(fakeExec{
		out: map[string]string{
			key("for-each-ref", "--format=%(refname:short)|%(upstream:short)|%(HEAD)", "refs/heads"): "invalid_line_without_pipes\nmain|origin/main|*",
			key("rev-list", "--left-right", "--count", "HEAD...origin/main"):                         "0 0",
		},
	})
	branches, err = cliInvalidLine.ListBranches(context.Background(), "/tmp/x", false)
	if err != nil || len(branches) != 1 {
		t.Fatalf("expected 1 branch after skipping invalid line, got %d", len(branches))
	}
}

func TestHasPendingChangesEdgeCases(t *testing.T) {
	// Case: no upstream configured
	cli1 := NewWithExecutor(fakeExec{
		out: map[string]string{
			key("symbolic-ref", "--short", "HEAD"): "main",
			key("status", "--porcelain"):           "",
		},
		err: map[string]error{
			key("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"): errors.New("no upstream"),
			key("show-ref", "--verify", "refs/remotes/origin/main"):                 errors.New("no remote"),
		},
	})
	if cli1.HasPendingChanges(context.Background(), "/tmp/x") {
		t.Fatal("expected false when no upstream")
	}
}

func TestHasRemoteUpdateEdgeCases(t *testing.T) {
	// Case 1: no upstream
	cliNoUp := NewWithExecutor(fakeExec{
		out: map[string]string{
			key("symbolic-ref", "--short", "HEAD"): "main",
		},
		err: map[string]error{
			key("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"): errors.New("no upstream"),
			key("show-ref", "--verify", "refs/remotes/origin/main"):                 errors.New("no remote"),
		},
	})
	if cliNoUp.HasRemoteUpdate(context.Background(), "/tmp/x") {
		t.Fatal("expected false when no upstream")
	}

	// Case 2: fetch error
	cliFetchErr := NewWithExecutor(fakeExec{
		out: map[string]string{
			key("symbolic-ref", "--short", "HEAD"):                                  "main",
			key("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"): "origin/main",
		},
		err: map[string]error{
			key("fetch", "origin", "--quiet"): errors.New("network error"),
		},
	})
	if cliFetchErr.HasRemoteUpdate(context.Background(), "/tmp/x") {
		t.Fatal("expected false when fetch fails")
	}

	// Case 3: aheadBehind error
	cliAheadBehindErr := NewWithExecutor(fakeExec{
		out: map[string]string{
			key("symbolic-ref", "--short", "HEAD"):                                  "main",
			key("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"): "origin/main",
			key("fetch", "origin", "--quiet"):                                       "",
		},
		err: map[string]error{
			key("rev-list", "--left-right", "--count", "HEAD...origin/main"): errors.New("rev-list err"),
		},
	})
	if cliAheadBehindErr.HasRemoteUpdate(context.Background(), "/tmp/x") {
		t.Fatal("expected false when aheadBehind fails")
	}
}
