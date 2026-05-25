package devflow

import (
	"fmt"
	"os"
	"path/filepath"
)

// Release executes the complete release workflow: tests, push, cross-compile, and GitHub release
func (g *Go) Release(message, tag string, gh *GitHub) error {
	// 1. List cmd/ directories before starting
	cmds, err := g.listCmdDirs(g.rootDir)
	if err != nil {
		return err
	}
	if len(cmds) == 0 {
		return fmt.Errorf("no cmd/ found in %s", g.rootDir)
	}

	// 2. Run full gopush workflow
	res, err := g.Push(message, tag, false, false, false, false, false, false, "..")
	if err != nil {
		return err
	}

	createdTag := res.Tag
	if createdTag == "" {
		return fmt.Errorf("release failed: no tag was created")
	}

	// 3. Create temp directory for artifacts
	tmpDir, err := os.MkdirTemp("", "gorelease-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// 4. Cross-compile
	var assets []string
	if g.crossCompileFn != nil {
		assets, err = g.crossCompileFn(tmpDir, cmds, DefaultTargets(), g.rootDir)
	} else {
		assets, err = CrossCompile(tmpDir, cmds, DefaultTargets(), g.rootDir)
	}

	if err != nil {
		return fmt.Errorf("cross-compilation failed: %w", err)
	}

	// 5. Create GitHub Release
	url, err := gh.CreateRelease(createdTag, assets)
	if err != nil {
		return fmt.Errorf("failed to create GitHub release: %w", err)
	}

	g.consoleOutput(fmt.Sprintf("✅ Release → %s", url))
	return nil
}

// DefaultTargets returns the standard set of platforms for release
func DefaultTargets() []CrossTarget {
	return []CrossTarget{
		{"linux", "amd64"},
		{"darwin", "arm64"},
		{"windows", "amd64"},
	}
}

// CrossCompile builds the specified commands for multiple platforms
func CrossCompile(tmpDir string, cmds []string, targets []CrossTarget, repoDir string) ([]string, error) {
	var assets []string

	for _, target := range targets {
		for _, cmd := range cmds {
			name := cmd
			suffix := fmt.Sprintf("-%s-%s", target.GOOS, target.GOARCH)
			if target.GOOS == "windows" {
				suffix += ".exe"
			}
			outputName := name + suffix
			outputPath := filepath.Join(tmpDir, outputName)

			// Use exec.Command directly for better cross-platform support with Env
			buildCmd := ExecCommand("go", "build", "-o", outputPath, "./cmd/"+cmd)
			buildCmd.Dir = repoDir
			buildCmd.Env = append(os.Environ(),
				"CGO_ENABLED=0",
				"GOOS="+target.GOOS,
				"GOARCH="+target.GOARCH,
			)

			outputBytes, err := buildCmd.CombinedOutput()
			if err != nil {
				return nil, fmt.Errorf("failed to build %s for %s/%s: %w\nOutput: %s",
					cmd, target.GOOS, target.GOARCH, err, string(outputBytes))
			}

			assets = append(assets, outputPath)
		}
	}

	return assets, nil
}
