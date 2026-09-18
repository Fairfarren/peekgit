package tui

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Fairfarren/peekgit/internal/config"
	"github.com/Fairfarren/peekgit/internal/gitcli"
	ghprovider "github.com/Fairfarren/peekgit/internal/provider/github"
	"github.com/Fairfarren/peekgit/internal/workspace"
	tea "github.com/charmbracelet/bubbletea"
	gh "github.com/google/go-github/v57/github"
)

type budgetTransport struct{}

func (budgetTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	deadline, ok := r.Context().Deadline()
	if !ok || time.Until(deadline) < time.Second {
		return nil, errors.New("请求预算不足")
	}
	body := `{"login":"tester","items":[]}`
	if strings.Contains(r.URL.Path, "/pulls/") {
		body = "diff --git a/a b/a\n+新增内容"
	}
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
}

type budgetGitExecutor struct{}

func (budgetGitExecutor) Run(ctx context.Context, _ string, args ...string) (string, error) {
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) < time.Second {
		return "", errors.New("Git 请求预算不足")
	}
	switch args[0] {
	case "remote":
		return "https://github.com/example/repo.git", nil
	case "symbolic-ref":
		return "main", nil
	case "rev-parse":
		return "origin/main", nil
	case "rev-list":
		return "0 1", nil
	}
	return "", nil
}

func Test_账号刷新_网络请求有执行预算(t *testing.T) {
	a := fixtureApp()
	a.startTab = startTabPR
	a.gh = ghprovider.NewWithClient(gh.NewClient(&http.Client{Transport: budgetTransport{}}))
	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	a.Update(cmd())

	if a.startPRErr != "" || a.startIssueErr != "" {
		t.Fatalf("PR 错误 = %s，Issue 错误 = %s", a.startPRErr, a.startIssueErr)
	}
}

func Test_加载差异_账号与仓库请求均有执行预算(t *testing.T) {
	for _, page := range []screen{screenWorkspaces, screenDetail} {
		t.Run(string(rune('0'+page)), func(t *testing.T) {
			a := fixtureApp()
			a.screen = page
			a.startTab = startTabPR
			a.git = gitcli.NewWithExecutor(budgetGitExecutor{})
			a.gh = ghprovider.NewWithClient(gh.NewClient(&http.Client{Transport: budgetTransport{}}))
			_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

			msg := cmd().(diffLoadedMsg)

			if msg.err != nil || !strings.Contains(msg.content, "新增内容") {
				t.Fatalf("内容 = %q，错误 = %v", msg.content, msg.err)
			}
		})
	}
}

func Test_拉取仓库_请求预算允许完成(t *testing.T) {
	for _, key := range []rune{'f', 'F'} {
		t.Run(string(key), func(t *testing.T) {
			a := fixtureApp()
			a.screen = screenHome
			a.git = gitcli.NewWithExecutor(budgetGitExecutor{})
			_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})

			msg := cmd()

			switch result := msg.(type) {
			case pullDoneMsg:
				if result.err != nil {
					t.Fatal(result.err)
				}
			case pullAllDoneMsg:
				if result.failed != 0 || result.completed != 10 {
					t.Fatalf("拉取结果 = %+v", result)
				}
			default:
				t.Fatalf("未知消息 %T", msg)
			}
		})
	}
}

func Test_工作区检查_请求预算允许发现更新(t *testing.T) {
	stubLifecycle(t)
	scanWorkspaceReposFn = func(config.Config, []string) ([]workspace.RepoDir, error) {
		return []workspace.RepoDir{{Name: "仓库", Path: "项目"}}, nil
	}
	a := New(config.Config{NoGitHub: true, IntervalSec: 300})
	a.workspaces = []string{"项目"}
	a.git = gitcli.NewWithExecutor(budgetGitExecutor{})
	_, cmd := a.Update(tickMsg(time.Time{}))
	messages := cmd().(tea.BatchMsg)

	msg := messages[1]().(workspaceCheckDoneMsg)

	if !msg.hasUpdate {
		t.Fatal("工作区检查未发现远程更新")
	}
}
