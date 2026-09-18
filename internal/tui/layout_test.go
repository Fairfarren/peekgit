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

func Test_列布局_边界尺寸与间距(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		width, min, gap, want int
	}{
		{"零宽度", 0, -2, 3, 1}, {"间距加入总宽", 90, 44, 2, 2}, {"间距加入卡片宽", 100, 44, 8, 2},
		{"负宽度", -1, 10, 2, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := computeColumns(tc.width, tc.min, tc.gap)

			if got != tc.want {
				t.Fatalf("列数 = %d，期望 %d", got, tc.want)
			}
		})
	}
}

func Test_卡片宽度_多列间距只计列间(t *testing.T) {
	got := computeCardWidth(120, 3, 6)

	if got != 36 {
		t.Fatalf("卡片宽度 = %d，期望 36", got)
	}
}

func Test_越界索引导航_从最后一项恢复(t *testing.T) {
	got := moveIndex(20, 5, 2, "left")

	if got != 3 {
		t.Fatalf("索引 = %d，期望 3", got)
	}
}
