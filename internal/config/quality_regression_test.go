package config

import (
	"path/filepath"
	"reflect"
	"testing"
)

func Test_全局配置_展开用户目录并保留普通路径(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "home", "example")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	original := readConfigFile
	readConfigFile = func(string) ([]byte, error) {
		return []byte(`{"workspaces":{"项目":["~","~/repo","~\\other","plain"]}}`), nil
	}
	t.Cleanup(func() { readConfigFile = original })
	want := WorkspaceMap{"项目": {home, filepath.Join(home, "repo"), filepath.Join(home, "other"), "plain"}}

	got, err := LoadGlobalConfig()

	if err != nil || !reflect.DeepEqual(got.Workspaces, want) {
		t.Fatalf("配置 = %+v，错误 = %v", got, err)
	}
}
