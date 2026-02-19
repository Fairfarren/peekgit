package model

import "time"

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
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	b := make([]byte, 0, 11)
	for v > 0 {
		d := v % 10
		b = append([]byte{byte('0' + d)}, b...)
		v /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
