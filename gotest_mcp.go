package devflow

import (
	"fmt"

	"github.com/tinywasm/mcp"
)

// GoTestProvider exposes the gotest suite as a single MCP tool.
type GoTestProvider struct {
	g *Go
}

// NewGoTestProvider creates a new GoTestProvider.
func NewGoTestProvider(g *Go) *GoTestProvider {
	return &GoTestProvider{g: g}
}

// GetMCPTools implements mcp.ToolProvider.
func (p *GoTestProvider) GetMCPTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name: "run_tests",
			Description: "Comprehensive Go test suite: runs vet, stdlib tests with race detection, exact " +
				"coverage analysis, and auto-detected WASM tests. Full suite (no args) includes badges update " +
				"and slow test detection. Fast path (with -run/flags) skips vet and badges. Intelligent caching " +
				"by git state; cache disabled with custom flags.",
			Parameters: []mcp.Parameter{
				{
					Name:        "run",
					Type:        "string",
					Description: "Optional: run only tests matching this name/pattern (e.g. TestFoo). Empty runs full suite: vet, race, coverage, WASM, badges. With custom flags uses fast path: vet and badges skipped, cache disabled.",
				},
			},
			Execute: p.execute,
		},
	}
}

func (p *GoTestProvider) execute(args map[string]any) {
	run := ""
	if v, ok := args["run"]; ok {
		if s, ok := v.(string); ok {
			run = s
		}
	}

	var summary string
	var err error

	// Full suite: vet + race + coverage + WASM + badges + cache
	// Fast path: only go test + WASM auto-detect, no vet/badges/cache
	if run == "" {
		summary, err = p.g.Test(nil, false, 0, false, false) // full suite
	} else {
		summary, err = p.g.Test([]string{"-run", run}, false, 0, false, false) // fast path
	}

	// We use the logger to return the summary, as required by the MCP adapter in this repo's version.
	// Summary includes success/failure markers (✅/❌).
	p.g.log(summary)
	if err != nil {
		p.g.log(fmt.Sprintf("Error: %v", err))
	}
}
