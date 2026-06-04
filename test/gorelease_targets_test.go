package devflow_test

import (
	"testing"

	"github.com/tinywasm/devflow"
)

// EXPECTED (mejora C): DefaultTargets must cover the full public distribution
// matrix. Public binary distribution reaches a heterogeneous audience:
//   - linux/amd64, linux/arm64 (ARM servers, CI, Raspberry)
//   - darwin/arm64 (Apple Silicon), darwin/amd64 (Intel Macs)
//   - windows/amd64
// darwin/amd64 and linux/arm64 are currently missing.
func TestDefaultTargets_CoversDistributionMatrix(t *testing.T) {
	targets := devflow.DefaultTargets()

	want := map[string]bool{
		"linux/amd64":   false,
		"linux/arm64":   false,
		"darwin/arm64":  false,
		"darwin/amd64":  false,
		"windows/amd64": false,
	}
	for _, tg := range targets {
		want[tg.GOOS+"/"+tg.GOARCH] = true
	}

	for k, found := range want {
		if !found {
			t.Errorf("DefaultTargets missing %s", k)
		}
	}
}
