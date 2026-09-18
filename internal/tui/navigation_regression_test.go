package tui

import (
	"errors"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"testing"

	ghprovider "github.com/Fairfarren/peekgit/internal/provider/github"
	tea "github.com/charmbracelet/bubbletea"
	gh "github.com/google/go-github/v57/github"
)

type emptyAccountTransport struct{}

func (emptyAccountTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"login":"test-user","items":[]}`))}, nil
}

func TestWorkspaceNavigationCompletesEmptyAccountLoad(t *testing.T) {
	cases := []struct {
		name string
		key  tea.KeyMsg
		from startTab
		to   startTab
	}{
		{"右键", tea.KeyMsg{Type: tea.KeyRight}, startTabPR, startTabIssue},
		{"l键", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}, startTabPR, startTabIssue},
		{"左键", tea.KeyMsg{Type: tea.KeyLeft}, startTabIssue, startTabPR},
		{"h键", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}, startTabIssue, startTabPR},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := newTestApp()
			a.screen = screenWorkspaces
			a.startTab = tc.from
			a.gh = ghprovider.NewWithClient(gh.NewClient(&http.Client{Transport: emptyAccountTransport{}}))

			_, cmd := a.Update(tc.key)

			if cmd == nil {
				t.Fatal("切换页签没有返回加载命令")
			}
			a.Update(cmd())
			if a.startLoading || a.startTab != tc.to || a.startPRErr != "" || a.startIssueErr != "" {
				t.Fatalf("加载未完成: tab=%v, loading=%v, PR=%q, Issue=%q", a.startTab, a.startLoading, a.startPRErr, a.startIssueErr)
			}
		})
	}
}

func Test_浏览器命令_传递目标且保留启动失败(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result error
	}{{"启动成功", nil}, {"启动失败", errors.New("系统拒绝启动")}} {
		t.Run(tc.name, func(t *testing.T) {
			original := runBrowserCommand
			var args []string
			// 系统进程边界必须隔离；参数副本代表交给系统的打开目标。
			runBrowserCommand = func(cmd *exec.Cmd) error { args = append([]string(nil), cmd.Args...); return tc.result }
			t.Cleanup(func() { runBrowserCommand = original })
			target := "https://example.com/pr/42?tab=files"

			err := openBrowser(target)

			if !errors.Is(err, tc.result) || len(args) == 0 || args[len(args)-1] != target {
				t.Fatalf("命令 = %v，错误 = %v", args, err)
			}
		})
	}
}
