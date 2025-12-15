# Badges

Generate SVG badges for your README (test status, coverage, Go version, etc.).

## Overview

`badges` generates a static SVG image containing multiple badges and updates your `README.md` to display it. It is designed to work with minimal configuration, suitable for CI/CD pipelines or local development.

## Usage

### CLI

```bash
# Generate and update README with defaults
badges

# Customize values
badges -test-status="Failing" -coverage=50 -module-name="my-mod"
```

### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-module-name` | Module name | `testmodule` |
| `-test-status` | Test status label | `Passing` |
| `-coverage` | Coverage percentage | `85` |
| `-race-status` | Race detection status | `Clean` |
| `-vet-status` | Go vet status | `OK` |
| `-license` | License type | `MIT` |
| `-readme` | Path to README file | `README.md` |

### Library

You can importantly use the `badges` package directly in your Go code to generate SVGs programmatically.

```go
import "github.com/tinywasm/devflow"

func main() {
    // Define badges in "Label:Value:Color" format
    // Special commands:
    // - output_svgfile:<path>
    // - readmefile:<path>
    
    handler := devflow.NewBadges(
        "Go:1.20:#007d9c",
        "Tests:Passing:#4c1",
        "Coverage:90%:#4c1",
        "output_svgfile:docs/my_badges.svg",
    )
    
    // Generate SVG and get markdown update args
    _, err := handler.BuildBadges()
    if err != nil {
        panic(err)
    }
}
```

## How it works

1.  **SVG Generation**: Creates a single SVG file containing all defined badges.
2.  **README Update**: Updates a specific section in your markdown file (default `BADGES_SECTION`) with a link to the generated SVG.

### Markdown Section

Ensure your `README.md` has the following section markers:

```markdown
<!-- START_SECTION:BADGES_SECTION -->
<!-- END_SECTION:BADGES_SECTION -->
```

The tool will inject the badge image between these markers.
