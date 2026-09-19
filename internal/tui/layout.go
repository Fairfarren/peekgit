package tui

func getResponsiveCardMinWidth(termWidth int) int {
	if termWidth >= 200 {
		return 50
	}
	if termWidth >= 120 {
		return 44
	}
	if termWidth >= 80 {
		return 35
	}
	return 25
}

func computeColumns(availableWidth int, cardMinWidth int, gap int) int {
	if availableWidth <= 0 {
		return 1
	}
	denom := cardMinWidth + gap
	if denom <= 0 {
		return 1
	}
	cols := (availableWidth + gap) / denom
	return max(1, cols)
}

func computeCardWidth(availableWidth int, columns int, gap int) int {
	columns = max(1, columns)
	return max(1, (availableWidth-gap*(columns-1))/columns)
}

func clamp(val, lower, upper int) int {
	return max(lower, min(val, upper))
}

func moveIndex(current int, total int, columns int, key string) int {
	total = max(1, total)
	current = clamp(current, 0, total-1)

	var next int
	switch key {
	case "up":
		next = current - columns
	case "down":
		next = current + columns
	case "left":
		next = current - 1
	case "right":
		next = current + 1
	default:
		return current
	}

	return clamp(next, 0, total-1)
}
