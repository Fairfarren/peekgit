package tui

import (
	"testing"
)

func TestParseDiff(t *testing.T) {
	raw := `diff --git a/file1.go b/file1.go
index 123..456 100644
--- a/file1.go
+++ b/file1.go
@@ -1,1 +1,2 @@
+new line
 line
diff --git a/file2.go b/file2.go
new file mode 100644
index 000..789
--- /dev/null
+++ b/file2.go
@@ -0,0 +1,1 @@
+content`

	dt := ParseDiff(raw)
	if len(dt.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(dt.Files))
	}

	f1 := dt.GetFileByIndex(0)
	if f1.Path != "file1.go" || f1.AddLines != 1 {
		t.Fatalf("unexpected file1: %+v", f1)
	}

	f2 := dt.GetFileByIndex(1)
	if f2.Path != "file2.go" || !f2.IsNew {
		t.Fatalf("unexpected file2: %+v", f2)
	}
}

func TestBuildDiffTree(t *testing.T) {
	files := []FileDiff{
		{Path: "dir/file1.go", Content: "diff1"},
		{Path: "dir/subdir/file2.go", Content: "diff2"},
	}

	dt := BuildDiffTree(files)
	if len(dt.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(dt.Files))
	}

	if dt.Tree.Name != "" || !dt.Tree.IsDir {
		t.Fatalf("unexpected root")
	}

	if len(dt.Tree.Children) != 1 || dt.Tree.Children[0].Name != "dir" {
		t.Fatalf("expected dir child")
	}

	dir := dt.Tree.Children[0]
	if len(dir.Children) != 2 {
		t.Fatalf("expected 2 children in dir")
	}
}

func TestGetFileContent(t *testing.T) {
	dt := &DiffTree{
		Files: []FileDiff{
			{Path: "test.go", Content: "hello"},
		},
	}

	if dt.GetFileContent("test.go") != "hello" {
		t.Fatalf("expected hello")
	}
	if dt.GetFileContent("nonexistent") != "" {
		t.Fatalf("expected empty")
	}
}
