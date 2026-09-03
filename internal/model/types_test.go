package model

import "testing"

func TestSyncSymbol(t *testing.T) {
	if got := SyncSymbol(SyncSynced, 0, 0); got != "✓" {
		t.Fatalf("got %s", got)
	}
	if got := SyncSymbol(SyncAhead, 2, 0); got != "↑2" {
		t.Fatalf("got %s", got)
	}
	if got := SyncSymbol(SyncBehind, 0, 3); got != "↓3" {
		t.Fatalf("got %s", got)
	}
	if got := SyncSymbol(SyncDiverged, 1, 4); got != "↑1 ↓4" {
		t.Fatalf("got %s", got)
	}
	if got := SyncSymbol(SyncUnknown, 0, 0); got != "—" {
		t.Fatalf("got %s", got)
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		in   int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{-1, "-1"},
		{42, "42"},
		{-42, "-42"},
		{100, "100"},
		{-100, "-100"},
	}
	for _, tt := range tests {
		if got := itoa(tt.in); got != tt.want {
			t.Errorf("itoa(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
