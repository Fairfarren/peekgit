package gitcli

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"reflect"
	"testing"
	"time"

	"github.com/Fairfarren/peekgit/internal/model"
)

func Test_命令失败_保留标准错误或执行错误(t *testing.T) {
	for _, tc := range []struct{ name, stderr, want string }{
		{"标准错误", "  无法访问远程\n", "无法访问远程"},
		{"空标准错误", "", "执行失败"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := runGitCommand
			runGitCommand = func(cmd *exec.Cmd) error {
				_, _ = io.WriteString(cmd.Stderr, tc.stderr)
				return errors.New("执行失败")
			}
			t.Cleanup(func() { runGitCommand = original })

			err := New().CheckoutBranch(context.Background(), "", "main")

			if err == nil || err.Error() != tc.want {
				t.Fatalf("错误 = %v，期望 %s", err, tc.want)
			}
		})
	}
}

func Test_刷新仓库_使用显式上游或回退分支(t *testing.T) {
	for _, tc := range []struct {
		name, upstream, want string
		err                  error
	}{
		{"显式上游", "origin/feature", "origin/feature", nil},
		{"空上游", "", "origin/main", nil},
		{"查询失败", "", "origin/main", errors.New("没有上游")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx := fakeExec{out: map[string]string{
				"symbolic-ref --short HEAD":                               "main",
				"rev-parse --abbrev-ref --symbolic-full-name @{upstream}": tc.upstream,
			}, err: map[string]error{}}
			if tc.err != nil {
				fx.err["rev-parse --abbrev-ref --symbolic-full-name @{upstream}"] = tc.err
			}

			got := NewWithExecutor(fx).RefreshRepo(context.Background(), "仓库", "仓库路径")

			if got.Upstream != tc.want {
				t.Fatalf("上游 = %q，期望 %q", got.Upstream, tc.want)
			}
		})
	}
}

func Test_分支列表_保留同步状态与错误回退(t *testing.T) {
	fx := fakeExec{out: map[string]string{
		"for-each-ref --format=%(refname:short)|%(upstream:short)|%(HEAD) refs/heads": "main|origin/main|*\nlocal||\nbroken|origin/broken|",
		"rev-list --left-right --count HEAD...origin/main":                            "2 3",
		"rev-list --left-right --count HEAD...":                                       "9 9",
	}, err: map[string]error{"rev-list --left-right --count HEAD...origin/broken": errors.New("读取失败")}}
	want := []model.BranchInfo{
		{Name: "main", Upstream: "origin/main", Current: true, Dirty: true, Ahead: 2, Behind: 3, SyncSymbol: "↑2 ↓3"},
		{Name: "local", Dirty: true, SyncSymbol: "—"},
		{Name: "broken", Upstream: "origin/broken", Dirty: true, SyncSymbol: "—"},
	}

	got, err := NewWithExecutor(fx).ListBranches(context.Background(), "仓库路径", true)

	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("分支 = %+v，错误 = %v", got, err)
	}
}

type deadlineExecutor struct{}

func (deadlineExecutor) Run(ctx context.Context, _ string, args ...string) (string, error) {
	switch args[0] {
	case "symbolic-ref":
		return "main", nil
	case "rev-parse":
		return "origin/main", nil
	case "rev-list":
		return "1 1", nil
	case "fetch":
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) < time.Second {
			return "", errors.New("请求没有合理的超时预算")
		}
	}
	return "", nil
}

func Test_远程探测_为请求保留三秒预算(t *testing.T) {
	cli := NewWithExecutor(deadlineExecutor{})
	for _, tc := range []struct {
		name  string
		probe func(context.Context, string) bool
	}{{"待同步", cli.HasPendingChanges}, {"远程更新", cli.HasRemoteUpdate}} {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.probe(context.Background(), "仓库路径")

			if !got {
				t.Fatal("请求因超时预算错误而丢失更新")
			}
		})
	}
}
