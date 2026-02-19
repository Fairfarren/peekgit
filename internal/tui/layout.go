package tui

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

func moveIndex(current int, total int, columns int, key string) int {
	if total <= 0 {
		return 0
	}
	if current < 0 {
		current = 0
	}
	if current >= total {
		current = total - 1
	}

	next := current
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

	if next < 0 {
		return 0
	}
	if next >= total {
		return total - 1
	}
	return next
}
