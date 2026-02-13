package devflow_test

import "github.com/tinywasm/devflow"

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
		{"v0.4.6", "v0.0.51", 1},
		{"v0.0.51", "v0.4.6", -1},
		{"v0.4.6", "v0.4.6", 0},
	}

	for _, tt := range tests {
		got := devflow.CompareVersions(tt.v1, tt.v2)
		if got != tt.want {
			t.Errorf("devflow.CompareVersions(%q, %q) = %d; want %d", tt.v1, tt.v2, got, tt.want)
		}
	}
}
