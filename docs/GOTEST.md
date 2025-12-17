# gotest

Automated Go testing with minimal output.

## Installation

```bash
go install github.com/tinywasm/devflow/cmd/gotest@latest
```

## Usage

```bash
gotest           # Quiet mode (default)
gotest -v        # Verbose mode
```

## What it does

1. Runs `go vet ./...`
2. Runs `go test ./...`
3. Runs `go test -race ./...` (stdlib tests only)
4. Calculates coverage
5. Auto-detects and runs WASM tests if found (`*Wasm*_test.go`)
6. Updates README badges

## Output

**Quiet mode (default):**
```
✅ vet ok, ✅ tests stdlib ok, ✅ race detection ok, ✅ coverage: 71%
```

**With WASM tests:**
```
✅ vet ok, ✅ tests stdlib ok, ✅ race detection ok, ✅ coverage: 71%, ✅ tests wasm ok
```

**On failure:**
Shows only failed tests with error details, filters out passing tests.

## Exit codes

- `0` - All tests passed
- `1` - Tests failed, vet issues, or race conditions detected

## Notes

- No flags required - auto-detects test types
- Filters verbose output automatically
- Badge updates in `README.md` under `BADGES_SECTION`
