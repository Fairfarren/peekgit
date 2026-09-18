package config

import (
	"reflect"
	"testing"
)

func Test_全局配置_展开用户目录并保留普通路径(t *testing.T) {
	t.Setenv("HOME", "/home/example")
	original := readConfigFile
	readConfigFile = func(string) ([]byte, error) {
		return []byte(`{"workspaces":{"项目":["~","~/repo","~\\other","plain"]}}`), nil
	}
	t.Cleanup(func() { readConfigFile = original })
	want := WorkspaceMap{"项目": {"/home/example", "/home/example/repo", "/home/example/other", "plain"}}

	got, err := LoadGlobalConfig()

	if err != nil || !reflect.DeepEqual(got.Workspaces, want) {
		t.Fatalf("配置 = %+v，错误 = %v", got, err)
	}
}
