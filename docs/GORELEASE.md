# gorelease

Create GitHub Release with cross-platform binaries for an existing tag.

`gorelease` is a release-only tool: it does not create tags or commits. It cross-compiles
a Go project's `cmd/` binaries for multiple platforms and uploads them as assets to a
GitHub Release.

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
    * Linux (amd64)
    * macOS (arm64)
    * Windows (amd64)
4. **GitHub Release**: Creates a GitHub Release using the `gh` CLI, with the tag name as
   the title, and uploads the cross-compiled binaries as release assets.
5. **Cleanup**: Automatically removes the temporary directory used for compilation.

### Targets

The default compilation targets are:

| OS | Architecture | Artifact Name |
|---|---|---|
| linux | amd64 | `<cmd>-linux-amd64` |
| darwin | arm64 | `<cmd>-darwin-arm64` |
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
