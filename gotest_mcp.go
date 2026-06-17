package devflow

import (
	"fmt"

	"github.com/tinywasm/context"
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

// Tools implements mcp.ToolProvider.
func (p *GoTestProvider) Tools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name: "run_tests",
			Description: "Comprehensive Go test suite: runs vet, stdlib tests with race detection, exact " +
				"coverage analysis, and auto-detected WASM tests. Full suite (no args) includes badges update " +
				"and slow test detection. Fast path (with -run/flags) skips vet and badges. Intelligent caching " +
				"by git state; cache disabled with custom flags.",
			InputSchema: `{"type":"object","properties":{"run":{"type":"string","description":"Optional: run only tests matching this name/pattern (e.g. TestFoo). Empty runs full suite: vet, race, coverage, WASM, badges."}}}`,
			Execute:     p.execute,
		},
	}
}

func (p *GoTestProvider) execute(_ *context.Context, req mcp.Request) (*mcp.Result, error) {
	run := ""
	if req.Params.Arguments != "" {
		// Arguments is a JSON string; extract "run" key manually to avoid stdlib json dependency
		// Simple extraction: look for "run":"<value>"
		args := req.Params.Arguments
		const key = `"run":"`
		if i := indexOf(args, key); i >= 0 {
			start := i + len(key)
			end := indexOf(args[start:], `"`)
			if end >= 0 {
				run = args[start : start+end]
			}
		}
	}

	var summary string
	var err error

	if run == "" {
		summary, err = p.g.Test(nil, false, 0, false, false)
	} else {
		summary, err = p.g.Test([]string{"-run", run}, false, 0, false, false)
	}

	text := summary
	if err != nil {
		text += fmt.Sprintf("\nError: %v", err)
	}
	return mcp.Text(text), nil
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
