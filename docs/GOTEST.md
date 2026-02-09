# gotest

Automated Go testing: runs vet, stdlib tests, race detection, coverage, and WASM tests.

## Installation

```bash
go install github.com/tinywasm/devflow/cmd/gotest@latest
```

## Usage

```bash
gotest [go test flags]
```

**No arguments**: Runs the full test suite (vet, race, cover, wasm, badges)
**With arguments**: Passes flags to `go test` (fast path, no vet/wasm/badges/cache)

### Examples

```bash
gotest              # Full suite
gotest -v           # Verbose output (filtered)
gotest -run TestFoo # Run specific test
gotest -bench .     # Run benchmarks
```

## What it does

### Without arguments (full suite):

1. Runs `go vet ./...`
2. Runs `go test -race -cover ./...` (stdlib tests only)
3. Calculates coverage
4. Auto-detects and runs WASM tests if found (`*Wasm*_test.go`)
5. Updates README badges

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
✅ vet ok, ✅ tests stdlib ok, ✅ race detection ok, ✅ coverage: 71%, ✅ tests wasm ok
```

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

## Notes

- Supports all standard `go test` flags
- Auto-detects test types when running full suite
- Filters verbose output automatically (even with `-v`)
- Badge updates in `README.md` under `BADGES_SECTION` (only for full suite runs)
- Cache is disabled when using custom flags for accurate results
