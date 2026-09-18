package model

import (
	"strconv"
	"time"
)

type SyncState int

const (
	SyncUnknown SyncState = iota
	SyncSynced
	SyncAhead
	SyncBehind
	SyncDiverged
)

type RepoError string

const (
	RepoErrNone     RepoError = ""
	RepoErrFetch    RepoError = "fetch"
	RepoErrAuth     RepoError = "auth"
	RepoErrNoRemote RepoError = "no-remote"
	RepoErrNotARepo RepoError = "not-a-repo"
	RepoErrUpstream RepoError = "no-upstream"
	RepoErrUnknown  RepoError = "unknown"
)

type RepoStatus struct {
	Name      string
	Path      string
	Branch    string
	Upstream  string
	Ahead     int
	Behind    int
	Dirty     bool
	Sync      SyncState
	PROpen    *int
	IssueOpen *int
	Error     RepoError
	UpdatedAt time.Time
}

type BranchInfo struct {
	Name       string
	Upstream   string
	Ahead      int
	Behind     int
	Current    bool
	Dirty      bool
	SyncSymbol string
}

type PullRequestItem struct {
	Number      int
	Title       string
	Author      string
	HeadBranch  string
	BaseBranch  string
	UpdatedAt   time.Time
	Draft       bool
	HTMLURL     string
	DiffContent string
}

type IssueItem struct {
	Number    int
	Title     string
	Labels    []string
	UpdatedAt time.Time
	HTMLURL   string
	Body      string
}

type AccountPullRequestItem struct {
	Number     int
	Title      string
	RepoFull   string
	UpdatedAt  time.Time
	HTMLURL    string
	StateLabel string
	CIStatus   string
}

type AccountIssueItem struct {
	Number       int
	Title        string
	Labels       []string
	RepoFull     string
	UpdatedAt    time.Time
	HTMLURL      string
	StateLabel   string
	CreatedByMe  bool
	AssignedToMe bool
}

func SyncSymbol(state SyncState, ahead int, behind int) string {
	switch state {
	case SyncSynced:
		return "✓"
	case SyncAhead:
		return "↑" + itoa(ahead)
	case SyncBehind:
		return "↓" + itoa(behind)
	case SyncDiverged:
		return "↑" + itoa(ahead) + " ↓" + itoa(behind)
	default:
		return "—"
	}
}

func itoa(v int) string {
	return strconv.Itoa(v)
}
