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
	errStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#B83232", Dark: "#FF6B6B"})
	loadingStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B8860B", Dark: "#FFD700"})

	syncSyncedStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#1B8C3A", Dark: "#5FD97F"})
	syncAheadStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B8860B", Dark: "#FFD700"})
	syncBehindStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B83232", Dark: "#FF6B6B"})
	syncDivergedStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#8A3DB8", Dark: "#CE93D8"})
	syncUnknownStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#7A7A7A", Dark: "#666666"})

	dirtyStyle      = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#D4880F", Dark: "#FFAB40"})
	labelDimStyle   = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#7A7A7A", Dark: "#888888"})

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

	// Split diff view styles
	dirStyle          = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#1A8FA8", Dark: "#4FC3F7"})
	panelBorderFocus  = lipgloss.AdaptiveColor{Light: "#1A5CCC", Dark: "#6EA8FF"}
	panelBorderBlur   = lipgloss.AdaptiveColor{Light: "#C0C0C0", Dark: "#444444"}

	// File tree styles
	cursorStyle       = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#1A5CCC", Dark: "#6EA8FF"}).Bold(true)

	// Card styles
	CardStyle         = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.AdaptiveColor{Light: "#D0D0D0", Dark: "#444444"}).
				Padding(0, 1)

	CardSelectedStyle = lipgloss.NewStyle().
				Border(lipgloss.ThickBorder()).
				BorderForeground(lipgloss.AdaptiveColor{Light: "#1A5CCC", Dark: "#6EA8FF"}).
				Padding(0, 1)

	CardHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#333333", Dark: "#FFFFFF"})

	TagStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#1A8FA8", Dark: "#4FC3F7"})

	TagDirtyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#D4880F", Dark: "#FFAB40"})

	// Status block styles
	StatusBlockSyncedStyle   = lipgloss.NewStyle().Background(lipgloss.Color("#5FD97F")).Foreground(lipgloss.Color("#000000"))
	StatusBlockAheadStyle    = lipgloss.NewStyle().Background(lipgloss.Color("#FFD700")).Foreground(lipgloss.Color("#000000"))
	StatusBlockBehindStyle   = lipgloss.NewStyle().Background(lipgloss.Color("#FF6B6B")).Foreground(lipgloss.Color("#FFFFFF"))
	StatusBlockDivergedStyle = lipgloss.NewStyle().Background(lipgloss.Color("#CE93D8")).Foreground(lipgloss.Color("#000000"))

	// Help bar styles
	HelpBarStyle = lipgloss.NewStyle().
			Background(lipgloss.AdaptiveColor{Light: "#E8E8E8", Dark: "#333333"}).
			Foreground(lipgloss.AdaptiveColor{Light: "#555555", Dark: "#AAAAAA"}).
			Padding(0, 1)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#1A5CCC", Dark: "#6EA8FF"}).
			Bold(true)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#999999"})

	// Error banner styles
	ErrorBannerStyle = lipgloss.NewStyle().
			Background(lipgloss.AdaptiveColor{Light: "#FFEBEE", Dark: "#3D1F1F"}).
			Foreground(lipgloss.AdaptiveColor{Light: "#B83232", Dark: "#FF6B6B"}).
			Bold(true).
			Padding(0, 1)

	// File tree styles
	TreeDirStyle      = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#1A8FA8", Dark: "#4FC3F7"}).Bold(true)
	TreeFileStyle     = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#333333", Dark: "#CCCCCC"})
	TreeSelectedStyle = lipgloss.NewStyle().
				Background(lipgloss.AdaptiveColor{Light: "#D0E8FF", Dark: "#1A3A5C"}).
				Foreground(lipgloss.AdaptiveColor{Light: "#1A5CCC", Dark: "#6EA8FF"}).
				Bold(true)

	FileStatusNewStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#5FD97F"))
	FileStatusDelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B"))
	FileStatusModStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700"))
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
