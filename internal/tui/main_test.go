package tui

import (
	"os"
	"os/exec"
	"testing"
)

func TestMain(m *testing.M) {
	// 变异可能反转任意 URL 守卫；默认拒绝桌面命令，具体测试再注入可观察的结果桩。
	runBrowserCommand = func(*exec.Cmd) error { panic("单元测试禁止启动真实浏览器或 Finder，请注入命令桩") }
	os.Exit(m.Run())
}

func stubBrowserTargets(t *testing.T) *[]string {
	original := openBrowser
	var targets []string
	// 打开目标是可观察的副作用；记录意图而不启动用户的桌面程序。
	openBrowser = func(url string) error { targets = append(targets, url); return nil }
	t.Cleanup(func() { openBrowser = original })
	return &targets
}
