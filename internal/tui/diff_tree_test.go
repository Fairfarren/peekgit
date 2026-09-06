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

func TestParseDiffExtended(t *testing.T) {
	raw := `diff --git a/deleted.go b/dev/null
deleted file mode 100644
--- a/deleted.go
+++ /dev/null
@@ -1,1 +0,0 @@
-old line
diff --git a/bin.png b/bin.png
Binary files a/bin.png and b/bin.png differ
diff --git a/old.go b/new.go
rename from old.go
rename to new.go
`
	dt := ParseDiff(raw)
	if len(dt.Files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(dt.Files))
	}
	fDel := dt.GetFileByIndex(0)
	if !fDel.IsDelete || fDel.DelLines != 1 {
		t.Errorf("unexpected deleted file: %+v", fDel)
	}
	fBin := dt.GetFileByIndex(1)
	if !fBin.IsBinary {
		t.Errorf("unexpected binary file: %+v", fBin)
	}
	fRen := dt.GetFileByIndex(2)
	if fRen.OldPath != "old.go" || fRen.NewPath != "new.go" {
		t.Errorf("unexpected rename: %+v", fRen)
	}

	// Out of bounds GetFileByIndex
	if f := dt.GetFileByIndex(-1); f != nil {
		t.Errorf("expected nil for negative index")
	}
	if f := dt.GetFileByIndex(100); f != nil {
		t.Errorf("expected nil for out of range index")
	}
}

func TestParseDiffHeaderInvalid(t *testing.T) {
	if f := parseDiffHeader("not a header"); f.Path != "unknown" {
		t.Errorf("expected unknown path, got %s", f.Path)
	}
	if f := parseDiffHeader("diff --git no_slashes"); f.Path != "unknown" {
		t.Errorf("expected unknown path, got %s", f.Path)
	}
	if f := parseDiffHeader("diff --git a/single_path"); f.Path != "single_path" {
		t.Errorf("expected single_path, got %s", f.Path)
	}
	if f := parseDiffHeader("diff --git a/test b/"); f.Path != "test" {
		t.Errorf("expected test, got %s", f.Path)
	}
}

func TestParseDiffEmptyAndGetFileByIndexMissing(t *testing.T) {
	dt := ParseDiff("")
	if dt == nil || len(dt.Files) != 0 {
		t.Fatalf("expected empty diff tree for empty raw string")
	}

	dtMissing := &DiffTree{
		Files:    []FileDiff{{Path: "file1.go"}},
		FileList: []string{"missing.go"},
	}
	if f := dtMissing.GetFileByIndex(0); f != nil {
		t.Fatalf("expected nil when file in FileList is missing from Files")
	}
}
