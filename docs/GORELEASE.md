# gorelease

Create GitHub Release with cross-platform binaries for an existing tag.

`gorelease` is a release-only tool: it does not create tags or commits. It cross-compiles
a Go project's `cmd/` binaries for multiple platforms and uploads them as assets to a
GitHub Release.

## Installation

```bash
go install github.com/tinywasm/devflow/cmd/gorelease@latest
```

## Usage

```bash
gorelease [tag]
```

### Arguments

* **tag**: An optional explicit version tag (e.g., `v1.2.3`). If not provided, it is read
  from `git.GetLatestTag()` (the highest semver tag in the repository).

### Behavior

1. **Validation**: Verifies that the repository has a `cmd/` directory with at least one subdirectory.
2. **Tag Resolution**: If no tag is provided, reads the latest tag from git.
3. **Cross-Compilation**: Compiles all commands found in `cmd/` for the following platforms:
    * Linux (amd64, arm64)
    * macOS (arm64, amd64)
    * Windows (amd64)

   It injects the version into `main.Version` and uses optimization flags (`-s -w -trimpath`).

4. **Checksums**: Generates a `checksums.txt` file (SHA256) for all compiled binaries and includes it as a release asset.

5. **Target Resolution**: Automatically decides where to publish the release:
    * If `origin` is **PUBLIC**, it publishes to `origin` (classic behavior).
    * If `origin` is **PRIVATE**, it derives a public repository name: `<owner>/<folder-name>`.
      If that repository exists and is public, it publishes there using the `--repo` flag.
      This allows maintaining private source code while distributing public binaries.

6. **GitHub Release**: Creates a GitHub Release using the `gh` CLI, with the tag name as
   the title, and uploads the cross-compiled binaries and checksums as release assets.
5. **Cleanup**: Automatically removes the temporary directory used for compilation.

### Targets

The default compilation targets are:

| OS | Architecture | Artifact Name |
|---|---|---|
| linux | amd64 | `<cmd>-linux-amd64` |
| linux | arm64 | `<cmd>-linux-arm64` |
| darwin | arm64 | `<cmd>-darwin-arm64` |
| darwin | amd64 | `<cmd>-darwin-amd64` |
| windows | amd64 | `<cmd>-windows-amd64.exe` |

### Requirements

* `go` installed and in PATH.
* `gh` (GitHub CLI) installed, authenticated, and in PATH.
* A standard Go project structure with a `go.mod` file and a `cmd/` directory.
* An existing tag in the git repository (created by `gopush` or `codejob`).

## Examples

```bash
# Release the latest tag (highest semver)
gorelease

# Release an explicit tag
gorelease v1.2.3
```

## CI/CD Usage

`gorelease` can be run in CI/CD environments (headless) by providing a GitHub token via environment variables.

```yaml
# GitHub Actions example
- name: Run gorelease
  run: go run github.com/tinywasm/devflow/cmd/gorelease@latest
  env:
    GH_TOKEN: ${{ secrets.GITHUB_TOKEN }} # Or a PAT with 'repo' scope
```

`gorelease` prioritizes `GH_TOKEN`, then `GITHUB_TOKEN`, and falls back to the system keyring only if neither is present.

## Integration with `codejob`

The `-release` flag in `codejob` automatically triggers a release after `gopush`:

```bash
# Publish module + create GitHub Release in one step
codejob 'feat: new feature' -release

# With explicit tag
codejob 'feat: new feature' v1.2.3 -release
```

## Related

* [gopush](GOPUSH.md) - Publish Go modules and create tags.
* [codejob](CODEJOB.md) - Orchestrate coding tasks with optional release.
* [Flow Diagram](diagrams/GORELEASE_FLOW.md) - Visual representation of the `gorelease` process.
