package tui

func getResponsiveCardMinWidth(termWidth int) int {
	switch {
	case termWidth >= 200:
		return 50
	case termWidth >= 120:
		return 44
	case termWidth >= 80:
		return 35
	default:
		return 25
	}
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
	if cols < 1 {
		return 1
	}
	return cols
}

func computeCardWidth(availableWidth int, columns int, gap int) int {
	if columns <= 1 {
		if availableWidth < 1 {
			return 1
		}
		return availableWidth
	}
	w := (availableWidth - gap*(columns-1)) / columns
	if w < 1 {
		return 1
	}
	return w
}

func clamp(val, min, max int) int {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func moveIndex(current int, total int, columns int, key string) int {
	if total <= 0 {
		return 0
	}
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
