package devflow

import (
	"crypto/sha256"
	"fmt"
	"github.com/tinywasm/command"
	"io"
	"os"
	"path/filepath"
)

// ReleaseOnly creates a GitHub Release with cross-platform binaries for an existing tag.
// If tag is empty, it uses the latest tag from git.GetLatestTag().
func (g *Go) ReleaseOnly(tag string, gh *GitHub) error {
	// 1. Resolve tag first
	if tag == "" {
		var err error
		tag, err = g.git.GetLatestTag()
		if err != nil {
			return fmt.Errorf("failed to get latest tag: %w", err)
		}
		if tag == "" {
			return fmt.Errorf("no tags found in repository")
		}
	}

	// 2. Resolve target repository
	absRoot, err := filepath.Abs(g.rootDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path of root dir: %w", err)
	}

	target, err := g.resolvePublishTarget(filepath.Base(absRoot), gh)
	if err != nil {
		return err
	}

	// 3. List cmd/ directories before starting
	cmds, err := g.listCmdDirs(g.rootDir)
	if err != nil {
		return err
	}
	if len(cmds) == 0 {
		return fmt.Errorf("no cmd/ found in %s", g.rootDir)
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
		// Pass tag to CrossCompile for version injection
		assets, err = g.CrossCompileWithTag(tmpDir, cmds, DefaultTargets(), g.rootDir, tag)
	}

	if err != nil {
		return fmt.Errorf("cross-compilation failed: %w", err)
	}

	// 5. Generate Checksums
	checksumsPath := filepath.Join(tmpDir, "checksums.txt")
	f, err := os.Create(checksumsPath)
	if err != nil {
		return fmt.Errorf("failed to create checksums.txt: %w", err)
	}
	for _, asset := range assets {
		h := sha256.New()
		af, err := os.Open(asset)
		if err != nil {
			f.Close()
			return fmt.Errorf("failed to open asset for checksum: %w", err)
		}
		if _, err := io.Copy(h, af); err != nil {
			af.Close()
			f.Close()
			return fmt.Errorf("failed to calculate checksum: %w", err)
		}
		af.Close()
		fmt.Fprintf(f, "%x  %s\n", h.Sum(nil), filepath.Base(asset))
	}
	f.Close()
	assets = append(assets, checksumsPath)

	// 6. Create GitHub Release
	url, err := gh.CreateRelease(tag, assets, target)
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
		{"linux", "arm64"},
		{"darwin", "arm64"},
		{"darwin", "amd64"},
		{"windows", "amd64"},
	}
}

func (g *Go) resolvePublishTarget(folderName string, gh *GitHub) (string, error) {
	owner, _, visibility, err := gh.repoInfo("")
	if err != nil {
		// Visibility undetermined, fall back to origin
		return "", nil
	}

	if visibility == "PUBLIC" {
		return "", nil
	}

	// Origin is private -> candidate = owner/folderName
	candidate := owner + "/" + folderName
	_, _, candidateVisibility, err := gh.repoInfo(candidate)
	if err != nil {
		return "", fmt.Errorf("origin is private, but derived repo %s does not exist or is not accessible. A public repository is required for distribution: %w", candidate, err)
	}

	if candidateVisibility != "PUBLIC" {
		return "", fmt.Errorf("origin is private, and derived repo %s is also %s. A PUBLIC repository is required for distribution", candidate, candidateVisibility)
	}

	return candidate, nil
}

func crossBuildArgs(tag, cmd, outputPath string) []string {
	ldflags := "-s -w"
	if tag != "" {
		ldflags += fmt.Sprintf(" -X main.Version=%s", tag)
	}

	return []string{
		"build",
		"-o", outputPath,
		"-trimpath",
		"-ldflags=" + ldflags,
		"./cmd/" + cmd,
	}
}

// CrossCompileWithTag builds the specified commands for multiple platforms with version injection
func (g *Go) CrossCompileWithTag(tmpDir string, cmds []string, targets []CrossTarget, repoDir, tag string) ([]string, error) {
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

			// Use crossBuildArgs for versioning and optimization flags
			args := crossBuildArgs(tag, cmd, outputPath)
			buildCmd := command.Exec("go", args...)
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

// CrossCompile builds the specified commands for multiple platforms
func CrossCompile(tmpDir string, cmds []string, targets []CrossTarget, repoDir string) ([]string, error) {
	g := &Go{}
	return g.CrossCompileWithTag(tmpDir, cmds, targets, repoDir, "")
}
