# gorelease

Publish Go module + create GitHub Release with cross-platform binaries.

`gorelease` extends the `gopush` workflow. It first runs the complete `gopush` process (tests, commit, tag, push, local install, and dependent updates), and then cross-compiles the project's binaries for multiple platforms and uploads them as assets to a new GitHub Release.

## Usage

```bash
gorelease 'commit message' [tag]
```

### Arguments

* **message**: The commit message for the release. Required.
* **tag**: An optional explicit version tag (e.g., `v1.2.3`). If not provided, it is automatically generated based on the latest tag.

### Behavior

1. **Validation**: Verifies that the repository has a `cmd/` directory with at least one subdirectory.
2. **Push**: Executes `g.Push()`, which runs tests, commits changes, creates a tag, and pushes to the remote repository.
3. **Cross-Compilation**: Compiles all commands found in `cmd/` for the following platforms:
    * Linux (amd64)
    * macOS (arm64)
    * Windows (amd64)
4. **GitHub Release**: Creates a GitHub Release using the `gh` CLI, with the tag name as the title, and uploads the cross-compiled binaries as release assets.
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

## Examples

```bash
gorelease 'feat: add new command line tool'
gorelease 'fix: critical bug' 'v0.5.1'
```

## Related

* [gopush](GOPUSH.md) - The underlying workflow for publishing Go modules.
* [Flow Diagram](diagrams/GORELEASE_FLOW.md) - Visual representation of the `gorelease` process.
