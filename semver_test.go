package devflow

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1, v2 string
		want   int
	}{
		{"v1.0.0", "v1.0.0", 0},
		{"v1.0.0", "v1.0.1", -1},
		{"v1.0.1", "v1.0.0", 1},
		{"1.0.0", "v1.0.0", 0}, // Loose prefix handling
		{"v2.0", "v1.9.9", 1},
		{"v1.2", "v1.2.0", 0}, // Partial check (simplified implementation treats missing as 0 effectively in loop or stops)
		// Actually my implementation loop maxLen limits.
		// "1.2" parts=["1","2"], "1.2.0" parts=["1","2","0"]
		// i=2: n1=0 (default int), n2=0. So equal. Correct.
	}

	for _, tt := range tests {
		got := CompareVersions(tt.v1, tt.v2)
		if got != tt.want {
			t.Errorf("CompareVersions(%q, %q) = %d; want %d", tt.v1, tt.v2, got, tt.want)
		}
	}
}
