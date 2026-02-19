package tui

import "testing"

func TestComputeColumns(t *testing.T) {
	if c := computeColumns(40, 44, 2); c != 1 {
		t.Fatalf("columns=%d", c)
	}
	if c := computeColumns(100, 44, 2); c != 2 {
		t.Fatalf("columns=%d", c)
	}
}

func TestComputeCardWidth(t *testing.T) {
	w := computeCardWidth(100, 2, 2)
	if w != 49 {
		t.Fatalf("width=%d", w)
	}
}

func TestMoveIndex(t *testing.T) {
	if got := moveIndex(0, 5, 2, "down"); got != 2 {
		t.Fatalf("got=%d", got)
	}
	if got := moveIndex(4, 5, 2, "down"); got != 4 {
		t.Fatalf("got=%d", got)
	}
}
