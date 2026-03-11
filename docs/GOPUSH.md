# gopush (universal)

Automated build and publish workflow. It automatically detects if the project is a Go module and applies the appropriate pipeline.

## Usage

```bash
gopush 'commit message' [tag]
```

## Arguments

- **commit message**: Required. The message for the git commit.
- **tag**: Optional. The tag to create. If not provided, it will be auto-generated.
- **--skip-race** or **-R**: Optional. Skip race detection tests (only applicable to Go projects).

## Behavior

### For Go Projects (contains `go.mod`)

1. Verifies `go.mod`
2. Runs `gotest` (vet, tests, race, coverage, badges)
3. Commits changes with your message
4. Creates/uses tag
5. Intelligent push: Pushes to remote (auto-pulls/rebases if remote is ahead).
6. Automatically installs binaries with version tag (if `cmd/` exists)
7. Finds dependent modules in search path
8. For each dependent (in parallel):
   - Removes replace directive for published module
   - Runs `go get module@tag` and `go mod tidy`
   - If no other replaces exist: auto-publish dependent
   - Dependent results print in real-time to the console.
9. Executes backup (asynchronous)

### For Non-Go Projects

1. Commits changes with your message
2. Creates/uses tag
3. Intelligent push: Pushes to remote (auto-pulls/rebases if remote is ahead).
4. Executes backup (asynchronous)

See: [GOPUSH_FLOW.md](diagrams/GOPUSH_FLOW.md)

## Output

**Go Project Success:**
```
✅ vet ok, ✅ tests stdlib ok, ✅ race detection ok, ✅ coverage: 71%, ✅ Tag: v1.0.1, ✅ Pushed ok
```

**Non-Go Project Success:**
```
✅ Tag: v0.1.0, ✅ Pushed ok
```

## Examples

```bash
# Simple push
gopush 'feat: new feature'

# With specific tag
gopush 'fix: critical bug' 'v2.1.3'
```

## Exit codes

- `0` - Success
- `1` - Tests failed, git operation failed, or verification failed

## Note: Special characters in commit messages

When your commit message contains backticks (`` ` ``), `$`, or other shell special characters, use **single quotes** to prevent shell interpretation:

```bash
# ❌ Backticks will fail (shell tries to execute as commands)
gopush "feat: Add `afterLine` parameter"

# ✅ Use single quotes
gopush 'feat: Add `afterLine` parameter'

# ✅ Or escape backticks
gopush "feat: Add \`afterLine\` parameter"
```
