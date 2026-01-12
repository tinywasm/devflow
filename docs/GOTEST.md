# gotest

Automated Go testing: runs vet, stdlib tests, race detection, coverage, and WASM tests.

## Installation

```bash
go install github.com/tinywasm/devflow/cmd/gotest@latest
```

## Usage

```bash
gotest
```

## What it does

1. Runs `go vet ./...`
2. Runs `go test -race -cover ./...` (stdlib tests only)
3. Calculates coverage
4. Auto-detects and runs WASM tests if found (`*Wasm*_test.go`)
5. Updates README badges

## Output

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
