package main

import "testing"

func TestCompatStatus(t *testing.T) {
	tests := []struct {
		stars  int
		status string
		label  string
	}{
		{5, "great", "Runs great"},
		{4, "good", "Runs well"},
		{3, "playable", "Some issues"},
		{2, "issues", "Major issues"},
		{1, "broken", "Broken"},
		{0, "untested", "Untested"},
	}
	for _, tt := range tests {
		status, label := compatStatus(tt.stars)
		if status != tt.status || label != tt.label {
			t.Errorf("compatStatus(%d) = %q, %q; want %q, %q", tt.stars, status, label, tt.status, tt.label)
		}
	}
}
