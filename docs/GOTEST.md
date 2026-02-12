# gotest

Automated Go testing: runs vet, stdlib tests, race detection, coverage, and WASM tests.

## Installation

```bash
go install github.com/tinywasm/devflow/cmd/gotest@latest
```

## Usage

```bash
gotest [-t seconds] [go test flags]
```

**No arguments**: Runs the full test suite (vet, race, cover, wasm, badges)
**With arguments**: Passes flags to `go test` (fast path, no vet/wasm/badges/cache)

### Options

| Flag | Description | Default |
|------|-------------|---------|
| `-t N` | Per-package timeout in seconds | `30` |
| `-no-cache` | Force re-execution of tests, skipping cache | `false` |

### Examples

```bash
gotest              # Full suite, 30s timeout
gotest -t 120       # Full suite, 120s timeout
gotest -run TestFoo # Run specific test, 30s timeout
gotest -t 60 -run X # Custom args, 60s timeout
gotest -bench .     # Run benchmarks
```

### Note on Verbose Output
`gotest` now runs internal tests with `-v` by default, but uses `ConsoleFilter` to keep the output clean. It only shows failed tests and critical summaries, making verbose diagnostic data available without the noise.

## What it does

### Without arguments (full suite):

1. Runs `go vet ./...`
23. Runs `go test -race -cover ./...` (stdlib tests)
4. **Exact weighted coverage** using profile merging (`go tool cover`) across all packages.
5. Auto-detects and runs WASM tests if found (`*Wasm*_test.go`)
6. Detects slowest test (if > 2.0s)
7. Detects WASM released function calls
8. Updates README badges
9. Displays total execution time

### With arguments (fast path):

1. Runs `go test [your flags] ./...` (stdlib tests)
2. Auto-detects and runs WASM tests with same flags if found
3. Skips vet and badge updates
4. Always filters output for clean results
5. Race detection only if explicitly requested (e.g., `gotest -race -run TestFoo`)

## Test Caching

`gotest` includes an intelligent caching mechanism to avoid re-running tests when the code hasn't changed.

- **How it works**: It generates a unique key for the current module based on its git state (last commit hash + hash of uncommitted changes).
- **Behavior**: If a match is found in the cache, `gotest` returns the previous successful result immediately without executing any tests.
- **Persistence**: Caches are stored in `/tmp/gotest-cache/` and are automatically invalidated if any `.go` file or the git state changes.
- **Note**: Cache is **disabled** when using custom flags (e.g., `-run`, `-v`, `-bench`) to ensure accurate results.

## Output

**Full suite (no arguments):**
```
✅ vet ok, ✅ race detection ok, ✅ tests stdlib ok, ✅ tests wasm ok, ✅ coverage: 85%, ⚠️ slow: TestFoo (3.2s) (12.4s)
```

**Partial runs:**
- Displays individual package results when using flags.
- Summarizes slow tests if detected.
- Includes total execution time in parentheses.

### Special Detection
- **❌ timeout**: Reports tests that exceeded the per-package timeout (default 30s).
- **⚠️ slow**: Automatically reports the single slowest test if it exceeds 2.0s.
- **⚠️ WASM**: Summarizes calls to released functions in WASM tests.
- **(×N)**: Automatically deduplicates identical output lines for cleaner logs.

**Custom flags (e.g., `gotest -run TestFoo`):**
```
✅ tests stdlib ok
```

**Custom flags with WASM tests (e.g., `gotest -run TestWasmFoo`):**
```
✅ tests stdlib ok, ✅ tests wasm ok
```

**Cached run:**
The message is identical to the original run, but it executes instantly.

**On failure:**
Shows only failed tests with error details, filters out passing tests. Failed runs are never cached.

**Note:** Output is always filtered for clean results, even when using `-v` flag.

## Exit codes

- `0` - All tests passed
- `1` - Tests failed, vet issues, or race conditions detected

## Timeout

By default, each package has a **30-second** timeout. If a test hangs or takes too long, Go's test framework kills the package and `gotest` reports the offending test:

```
❌ timeout: TestSlowOperation (exceeded 30s)
```

Override with `-t`:
```bash
gotest -t 120       # 120s per package
```

If you pass `-timeout` directly (Go's native flag), `gotest` respects it and does not inject its own:
```bash
gotest -timeout 2m  # Go-native flag, gotest won't override
```

## Notes

- Supports all standard `go test` flags
- Auto-detects test types when running full suite
- Filters verbose output automatically (even with `-v`)
- Badge updates in `README.md` under `BADGES_SECTION` (only for full suite runs)
- Cache is disabled when using custom flags for accurate results
