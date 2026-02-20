package tui

import (
	"strings"

	"github.com/Fairfarren/peekgit/internal/model"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#1A5CCC", Dark: "#6EA8FF"})
	wsPathStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#7A7A7A", Dark: "#888888"})
	tokenOKStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#1B8C3A", Dark: "#5FD97F"})
	tokenBadStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B83232", Dark: "#FF6B6B"})
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#7A7A7A", Dark: "#666666"})
	errStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#B83232", Dark: "#FF6B6B"})
	loadingStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B8860B", Dark: "#FFD700"})

	syncSyncedStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#1B8C3A", Dark: "#5FD97F"})
	syncAheadStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B8860B", Dark: "#FFD700"})
	syncBehindStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B83232", Dark: "#FF6B6B"})
	syncDivergedStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#8A3DB8", Dark: "#CE93D8"})
	syncUnknownStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#7A7A7A", Dark: "#666666"})

	cardNameStyle   = lipgloss.NewStyle().Bold(true)
	dirtyStyle      = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#D4880F", Dark: "#FFAB40"})
	labelDimStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#7A7A7A", Dark: "#888888"})
	prLabelStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#1A8FA8", Dark: "#4FC3F7"})
	issueLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#8A3DB8", Dark: "#CE93D8"})

	tabActiveStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#1A5CCC", Dark: "#6EA8FF"}).Underline(true)
	tabInactiveStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#7A7A7A", Dark: "#666666"})

	selectedMarkerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#1A5CCC", Dark: "#6EA8FF"})
	numberStyle         = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#1A8FA8", Dark: "#4FC3F7"})
	authorStyle         = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#7A7A7A", Dark: "#888888"})
	dateStyle           = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#7A7A7A", Dark: "#666666"})

	diffAddStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#1B8C3A", Dark: "#5FD97F"})
	diffDelStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B83232", Dark: "#FF6B6B"})
	diffHunkStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#1A8FA8", Dark: "#4FC3F7"})
	diffMetaStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#B8860B", Dark: "#FFD700"})
	diffHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#1A5CCC", Dark: "#6EA8FF"})
	searchInfoStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B8860B", Dark: "#FFD700"})
)

func renderSyncColored(state model.SyncState, ahead int, behind int) string {
	sym := model.SyncSymbol(state, ahead, behind)
	switch state {
	case model.SyncSynced:
		return syncSyncedStyle.Render(sym)
	case model.SyncAhead:
		return syncAheadStyle.Render(sym)
	case model.SyncBehind:
		return syncBehindStyle.Render(sym)
	case model.SyncDiverged:
		return syncDivergedStyle.Render(sym)
	default:
		return syncUnknownStyle.Render(sym)
	}
}

func colorizeDiff(raw string) string {
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git"):
			lines[i] = diffMetaStyle.Render(line)
		case strings.HasPrefix(line, "index "),
			strings.HasPrefix(line, "new file"),
			strings.HasPrefix(line, "deleted file"),
			strings.HasPrefix(line, "similarity"),
			strings.HasPrefix(line, "rename "),
			strings.HasPrefix(line, "Binary files"):
			lines[i] = diffMetaStyle.Render(line)
		case strings.HasPrefix(line, "--- "),
			strings.HasPrefix(line, "+++ "):
			lines[i] = diffMetaStyle.Render(line)
		case strings.HasPrefix(line, "@@"):
			lines[i] = diffHunkStyle.Render(line)
		case strings.HasPrefix(line, "+"):
			lines[i] = diffAddStyle.Render(line)
		case strings.HasPrefix(line, "-"):
			lines[i] = diffDelStyle.Render(line)
		}
	}
	return strings.Join(lines, "\n")
}
