---
name: testing
description: Testing workflow with gotest CLI, gopush publishing, mocking patterns, WASM/stdlib dual testing, and diagram-driven testing. Use when writing or running tests.
---

# Testing

- **Testing Runner (`gotest`):** For Go tests, ALWAYS use the globally installed `gotest` CLI command. **DO NOT** use `go test` directly, and **DO NOT** invoke it via `go run github.com/tinywasm/devflow/cmd/gotest`. Simply type `gotest` (no arguments) for the full suite, or `gotest -run TestName`. It automatically handles `-vet`, `-race`, `-cover`, WASM tests, and README badges.
- **`gotest` in Agent Plans:** When writing a `PLAN.md` destined for an external agent (e.g., Jules), you MUST include the following installation step as the **first prerequisite** in the plan, because external agents run in isolated environments where `gotest` is not globally available:
    ```bash
    go install github.com/tinywasm/devflow/cmd/gotest@latest
    ```
- **Publishing (`gopush`):** If tests pass and docs are updated, ALWAYS use the globally installed `gopush 'your commit message'` CLI command to deploy. **DO NOT** use standard `git commit` / `git push`, and **DO NOT** invoke it via `go run`. It handles testing, tagging, pushing, and updating dependencies automatically.

- **Standard Library Only:** **NEVER** use external assertion libraries (e.g., `testify`, `gomega`). Use only the standard `testing`, `net/http/httptest`, and `reflect` APIs.
- **Mocking (No I/O):** Tests MUST use Mocks for all external interfaces to remain fast, deterministic, and side-effect free.

- **WASM/Stlib Dual Testing Pattern:**
    - **Separation:** Use build tags for isomorphic code (`frontWasm_test.go` -> `//go:build wasm`, `backStlib_test.go` -> `//go:build !wasm`).
    - **Shared Logic:** Both files MUST call a shared test runner (e.g., `RunAPITests(t)`) to avoid duplication.
    - **Unified Setup:** Use a single `setup_test.go` to initialize the library/test server once.

- **Diagram-Driven Testing (DDT) & Black-Box Validation:**
    - **Flow Coverage:** Logic flows defined in `docs/diagrams/*.md` MUST have corresponding Integration Tests covering all branches/diamonds.
    - **Visual/Binary Outputs:** Never write tests that perform exact string/byte matching on complex binary formats (e.g., PDFs, SVGs). Floating-point variations make them brittle. Use black-box integration testing that outputs a file to disk (`os.WriteFile`) to be verified visually by the developer.
