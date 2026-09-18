package workspace

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func stubAbsolutePathError(t *testing.T) error {
	original := absolutePath
	failure := errors.New("路径包含无效字符")
	absolutePath = func(string) (string, error) { return "", failure }
	t.Cleanup(func() { absolutePath = original })
	return failure
}

func Test_深度扫描_传播路径解析错误(t *testing.T) {
	failure := stubAbsolutePathError(t)

	repos, err := ScanReposWithDepth("无效路径", 2)

	if repos != nil || !errors.Is(err, failure) {
		t.Fatalf("扫描结果 = %v，错误 = %v", repos, err)
	}
}

func Test_配置扫描_跳过无法解析的路径(t *testing.T) {
	for _, path := range []string{"无效路径", "无效路径/*"} {
		t.Run(path, func(t *testing.T) {
			stubAbsolutePathError(t)

			repos, err := ScanRepos([]string{path})

			if len(repos) != 0 || err != nil {
				t.Fatalf("扫描结果 = %v，错误 = %v", repos, err)
			}
		})
	}
}

func stubScanFilesystem(t *testing.T) {
	oldStat, oldRead, oldDir := statPath, readFile, readDirectory
	t.Cleanup(func() { statPath, readFile, readDirectory = oldStat, oldRead, oldDir })
}

func Test_扫描_读取目录失败时返回空集合(t *testing.T) {
	stubScanFilesystem(t)
	statPath = func(string) (fs.FileInfo, error) { return nil, fs.ErrNotExist }
	readDirectory = func(string) ([]fs.DirEntry, error) { return nil, fs.ErrPermission }

	repos, err := ScanReposWithDepth("目录", 2)

	if len(repos) != 0 || err != nil {
		t.Fatalf("扫描结果 = %v，错误 = %v", repos, err)
	}
}

func Test_仓库检查_传播文件系统错误(t *testing.T) {
	for _, boundary := range []string{"仓库状态", "工作树文件", "工作树目标"} {
		t.Run(boundary, func(t *testing.T) {
			stubScanFilesystem(t)
			files := fstest.MapFS{".git": &fstest.MapFile{Data: []byte("gitdir: target")}}
			file, _ := files.Stat(".git")
			statPath = func(path string) (fs.FileInfo, error) {
				if boundary == "仓库状态" || filepath.Base(path) == "target" {
					return nil, fs.ErrPermission
				}
				return file, nil
			}
			readFile = func(string) ([]byte, error) {
				if boundary == "工作树文件" {
					return nil, fs.ErrPermission
				}
				return []byte("gitdir: target"), nil
			}

			ok, err := IsGitRepo("仓库")

			if ok || !errors.Is(err, fs.ErrPermission) {
				t.Fatalf("仓库检查 = %v，错误 = %v", ok, err)
			}
		})
	}
}
