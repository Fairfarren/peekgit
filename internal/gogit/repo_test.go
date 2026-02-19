package gogit

import (
	"testing"

	"github.com/Fairfarren/peekgit/internal/model"
)

func TestNew(t *testing.T) {
	cli := New()
	if cli == nil {
		t.Fatal("New() returned nil")
	}
}

func TestClassifySync(t *testing.T) {
	if classifySync(0, 0) != model.SyncSynced {
		t.Fatal("synced failed")
	}
	if classifySync(1, 0) != model.SyncAhead {
		t.Fatal("ahead failed")
	}
	if classifySync(0, 1) != model.SyncBehind {
		t.Fatal("behind failed")
	}
	if classifySync(1, 1) != model.SyncDiverged {
		t.Fatal("diverged failed")
	}
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

func TestParseOwnerRepoFromRemote(t *testing.T) {
	// 这个测试需要真实的 git 仓库，暂时跳过
	t.Skip("needs real git repository")
}

func TestCurrentBranch(t *testing.T) {
	// 这个测试需要真实的 git 仓库，暂时跳过
	t.Skip("needs real git repository")
}

func TestIsDirty(t *testing.T) {
	// 这个测试需要真实的 git 仓库，暂时跳过
	t.Skip("needs real git repository")
}

func TestAheadBehind(t *testing.T) {
	// 这个测试需要真实的 git 仓库，暂时跳过
	t.Skip("needs real git repository")
}

func TestResolveUpstream(t *testing.T) {
	// 这个测试需要真实的 git 仓库，暂时跳过
	t.Skip("needs real git repository")
}

func TestRefreshRepo(t *testing.T) {
	// 这个测试需要真实的 git 仓库，暂时跳过
	t.Skip("needs real git repository")
}

func TestListBranches(t *testing.T) {
	// 这个测试需要真实的 git 仓库，暂时跳过
	t.Skip("needs real git repository")
}

func TestCheckoutBranch(t *testing.T) {
	// 这个测试需要真实的 git 仓库，暂时跳过
	t.Skip("needs real git repository")
}

func TestPull(t *testing.T) {
	// 这个测试需要真实的 git 仓库，暂时跳过
	t.Skip("needs real git repository")
}
