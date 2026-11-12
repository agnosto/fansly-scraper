package utils

import "testing"

func TestFetchHook(t *testing.T) {
	got := -1
	SetFetchHook(func(n int) { got = n })
	ReportFetch(42)
	if got != 42 {
		t.Errorf("expected hook to receive 42, got %d", got)
	}
	SetFetchHook(nil)
	ReportFetch(7) // must not panic with nil hook
}
