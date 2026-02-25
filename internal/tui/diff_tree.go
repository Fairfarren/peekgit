package tui

import (
	"path/filepath"
	"sort"
	"strings"
)

// FileDiff represents a single file's diff content
type FileDiff struct {
	Path     string // file path (e.g., "internal/tui/app.go")
	OldPath  string // original path for renames
	NewPath  string // new path for renames
	Content  string // raw diff content for this file
	AddLines int    // number of added lines
	DelLines int    // number of deleted lines
	IsBinary bool   // whether this is a binary file
	IsNew    bool   // whether this is a new file
	IsDelete bool   // whether this file was deleted
}

// DiffTree represents the hierarchical structure of changed files
type DiffTree struct {
	Files    []FileDiff
	Tree     *DiffNode
	FileList []string // flattened list of file paths for navigation
}

// DiffNode represents a node in the file tree (can be directory or file)
type DiffNode struct {
	Name     string
	IsDir    bool
	Children []*DiffNode
	File     *FileDiff // nil for directories
	Expanded bool      // for directories
}

// ParseDiff parses raw diff content and extracts file diffs
func ParseDiff(raw string) *DiffTree {
	if raw == "" {
		return &DiffTree{}
	}

	files := parseDiffFiles(raw)
	tree := buildTree(files)
	fileList := flattenTree(tree)

	return &DiffTree{
		Files:    files,
		Tree:     tree,
		FileList: fileList,
	}
}

// parseDiffFiles extracts individual file diffs from raw diff content
func parseDiffFiles(raw string) []FileDiff {
	var files []FileDiff
	lines := strings.Split(raw, "\n")

	var currentFile *FileDiff
	var contentLines []string

	for _, line := range lines {
		// Check for new file header
		if strings.HasPrefix(line, "diff --git ") {
			// Save previous file if exists
			if currentFile != nil {
				currentFile.Content = strings.Join(contentLines, "\n")
				files = append(files, *currentFile)
			}

			// Parse the file path from "diff --git a/path b/path"
			currentFile = parseDiffHeader(line)
			contentLines = []string{line}
			continue
		}

		// Accumulate content for current file
		if currentFile != nil {
			contentLines = append(contentLines, line)

			// Detect file status
			if strings.HasPrefix(line, "new file mode ") {
				currentFile.IsNew = true
			}
			if strings.HasPrefix(line, "deleted file mode ") {
				currentFile.IsDelete = true
			}
			if strings.HasPrefix(line, "Binary files ") {
				currentFile.IsBinary = true
			}

			// Count additions and deletions
			if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
				currentFile.AddLines++
			}
			if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
				currentFile.DelLines++
			}

			// Handle rename detection
			if strings.HasPrefix(line, "rename from ") {
				currentFile.OldPath = strings.TrimPrefix(line, "rename from ")
			}
			if strings.HasPrefix(line, "rename to ") {
				currentFile.NewPath = strings.TrimPrefix(line, "rename to ")
			}
		}
	}

	// Don't forget the last file
	if currentFile != nil {
		currentFile.Content = strings.Join(contentLines, "\n")
		files = append(files, *currentFile)
	}

	return files
}

// parseDiffHeader extracts file path from "diff --git a/path b/path"
func parseDiffHeader(line string) *FileDiff {
	// Format: "diff --git a/path/to/file b/path/to/file"
	parts := strings.SplitN(line, " ", 4)
	if len(parts) < 4 {
		return &FileDiff{Path: "unknown"}
	}

	// Extract path from "a/path" and "b/path"
	aPath := strings.TrimPrefix(parts[2], "a/")
	bPath := strings.TrimPrefix(parts[3], "b/")

	// Use b/path as the main path (shows the destination)
	path := bPath
	if path == "" || path == "/dev/null" {
		path = strings.TrimPrefix(parts[2], "a/")
	}

	return &FileDiff{
		Path:    path,
		OldPath: aPath,
		NewPath: bPath,
	}
}

// buildTree constructs a hierarchical tree from file paths
func buildTree(files []FileDiff) *DiffNode {
	root := &DiffNode{
		Name:     "",
		IsDir:    true,
		Children: []*DiffNode{},
		Expanded: true,
	}

	for i := range files {
		addPathToTree(root, &files[i])
	}

	// Sort tree nodes
	sortTree(root)

	return root
}

// addPathToTree adds a file to the tree structure
func addPathToTree(root *DiffNode, file *FileDiff) {
	parts := strings.Split(file.Path, string(filepath.Separator))
	current := root

	for i, part := range parts {
		isLast := i == len(parts)-1

		if isLast {
			// This is a file
			current.Children = append(current.Children, &DiffNode{
				Name:  part,
				IsDir: false,
				File:  file,
			})
		} else {
			// This is a directory
			found := false
			for _, child := range current.Children {
				if child.IsDir && child.Name == part {
					current = child
					found = true
					break
				}
			}
			if !found {
				newDir := &DiffNode{
					Name:     part,
					IsDir:    true,
					Children: []*DiffNode{},
					Expanded: true,
				}
				current.Children = append(current.Children, newDir)
				current = newDir
			}
		}
	}
}

// sortTree sorts tree nodes alphabetically, directories first
func sortTree(node *DiffNode) {
	if !node.IsDir {
		return
	}

	sort.Slice(node.Children, func(i, j int) bool {
		// Directories come first
		if node.Children[i].IsDir != node.Children[j].IsDir {
			return node.Children[i].IsDir
		}
		return node.Children[i].Name < node.Children[j].Name
	})

	for _, child := range node.Children {
		sortTree(child)
	}
}

// flattenTree returns a flat list of file paths in tree order
func flattenTree(root *DiffNode) []string {
	var result []string
	flattenNode(root, &result)
	return result
}

// flattenNode recursively collects file paths
func flattenNode(node *DiffNode, result *[]string) {
	if !node.IsDir && node.File != nil {
		*result = append(*result, node.File.Path)
		return
	}

	for _, child := range node.Children {
		flattenNode(child, result)
	}
}

// GetFileContent returns the diff content for a specific file path
func (dt *DiffTree) GetFileContent(path string) string {
	for _, f := range dt.Files {
		if f.Path == path {
			return f.Content
		}
	}
	return ""
}

// GetFileByIndex returns the file at the given index in the flattened list
func (dt *DiffTree) GetFileByIndex(idx int) *FileDiff {
	if idx < 0 || idx >= len(dt.Files) {
		return nil
	}
	path := dt.FileList[idx]
	for i := range dt.Files {
		if dt.Files[i].Path == path {
			return &dt.Files[i]
		}
	}
	return nil
}
