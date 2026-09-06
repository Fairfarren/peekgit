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
	if got := moveIndex(2, 5, 2, "up"); got != 0 {
		t.Fatalf("got=%d", got)
	}
	if got := moveIndex(1, 5, 2, "left"); got != 0 {
		t.Fatalf("got=%d", got)
	}
	if got := moveIndex(1, 5, 2, "right"); got != 2 {
		t.Fatalf("got=%d", got)
	}
	if got := moveIndex(1, 5, 2, "other"); got != 1 {
		t.Fatalf("got=%d", got)
	}
	if got := moveIndex(0, 0, 2, "down"); got != 0 {
		t.Fatalf("got=%d", got)
	}
}

func TestComputeLayoutEdgeCases(t *testing.T) {
	if got := computeColumns(0, 44, 2); got != 1 {
		t.Errorf("got %d, want 1", got)
	}
	if got := computeColumns(50, -2, 2); got != 1 {
		t.Errorf("got %d, want 1", got)
	}
	if got := computeCardWidth(50, 1, 2); got != 50 {
		t.Errorf("got %d, want 50", got)
	}
	if got := computeCardWidth(0, 1, 2); got != 1 {
		t.Errorf("got %d, want 1", got)
	}
	if got := computeCardWidth(10, 5, 5); got != 1 {
		t.Errorf("got %d, want 1", got)
	}
}

func TestClamp(t *testing.T) {
	if got := clamp(-5, 0, 10); got != 0 {
		t.Errorf("clamp(-5, 0, 10) = %d, want 0", got)
	}
	if got := clamp(15, 0, 10); got != 10 {
		t.Errorf("clamp(15, 0, 10) = %d, want 10", got)
	}
	if got := clamp(5, 0, 10); got != 5 {
		t.Errorf("clamp(5, 0, 10) = %d, want 5", got)
	}
	if got := clamp(0, 0, 10); got != 0 {
		t.Errorf("clamp(0, 0, 10) = %d, want 0", got)
	}
	if got := clamp(10, 0, 10); got != 10 {
		t.Errorf("clamp(10, 0, 10) = %d, want 10", got)
	}
}
