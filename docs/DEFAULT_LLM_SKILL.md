
<!-- START_SECTION:CORE_PRINCIPLES -->
- **Single Responsibility Principle (SRP):** Every file (CSS, Go, JS) must have a single, well-defined purpose. This must be reflected in both the file's content and its naming convention.

- **Mandatory Dependency Injection (DI):**
    - **No Global State:** Avoid direct system calls (OS, Network) in logic.
    - **Interfaces:** Define interfaces for external dependencies (`Downloader`, `ProcessManager`).
    - **Composition:** Main structs must hold these interfaces.
    - **Injection:** `cmd/<app_name>/main.go` is the ONLY place where "Real" implementations are injected.

- **Framework-less Development:** For HTML/Web projects, use only the **Standard Library**. No external frameworks or libraries are allowed.

- **CSS-First Interactivity:** Minimize JavaScript usage. All UI interactivity (toggles, menus, states) must be implemented using pure CSS whenever possible.

- **Minimalist JS:** Use JavaScript only as a last resort for logic that cannot be handled by CSS or the Go backend.

- **Strict File Structure:**
    - **Flat Hierarchy:** Go libraries must avoid subdirectories. Keep files in the root.
    - **Max 500 lines:** Files exceeding 500 lines MUST be subdivided and renamed by domain.
    - **Test Organization:** If >5 test files exist in root, move **ALL** tests to `tests/`.
<!-- END_SECTION:CORE_PRINCIPLES -->

<!-- START_SECTION:TESTING -->
- **Testing:** For Go tests, always use `gotest` (`github.com/tinywasm/devflow/cmd/gotest`). It automatically runs `vet`, standard tests with `-race` and `-cover`, and detects/runs WASM tests. It features intelligent caching (based on git state) for instant feedback on unchanged code, and updates README badges automatically.
  
- **Diagram-Driven Testing (DDT):**
    - **Flow Coverage:** If a logic flow exists in a `docs/diagrams/*.md` (e.g. `PROCESS_FLOW.md`), it **MUST** have a corresponding Integration Test covering every branch (diamonds) and failure mode (timeouts/errors).

- **Standard Library Only:** **NEVER** use external assertion libraries (`testify`, `gomega`). Use only `testing`, `net/http/httptest`, `reflect`.

- **Mandatory Dependency Injection & Mocking:**
    - Since we avoid global state, tests **MUST** use Mocks for all external interfaces (`Keyring`, `Downloader`, etc.).
    - This ensures tests are fast (no I/O), deterministic, and safe (no side effects).

- **WASM/Stlib Dual Testing Pattern (Backend vs Frontend):**
    - **Separate Implementation:** Use build tags to separate logic.
        - `frontWasm_test.go` -> `//go:build wasm`
        - `backStlib_test.go` -> `//go:build !wasm`
    - **Shared Runner:** Both files MUST call a shared test runner (e.g., `RunAPITests(t)`) to avoid code duplication.
    - **Unified Setup:** Use a single `setup_test.go` to initialize the library/test server once for all tests.

- **Publishing:** Before publishing, you MUST update the documentation (see **Documentation Standards**). If all tests pass when using `gotest`, you can publish changes using `gopush` (`github.com/tinywasm/devflow/cmd/gopush`). This command runs tests, commits, tags, pushes, and updates dependent modules automatically.
<!-- END_SECTION:TESTING -->

<!-- START_SECTION:DOCUMENTATION -->
- **Documentation Standards:**
    - **Documentation First:** You MUST update the documentation *before* suggesting or executing `push` or `gopush`. Since APIs usually change during development, the documentation must reflect the current state.
    
    - **Context-Aware Rule Compilation:**
        - Every `IMPLEMENTATION.md` **MUST** start with a **"Development Rules"** section.
        - This section **MUST** copy/paste the **relevant** rules from `DEFAULT_LLM_SKILL.md` (e.g., if Backend, include DI/StartLib/DDT; if Frontend, include WASM rules).
        - **Goal**: Ensure the LLM has the specific constraints for *that* specific project explicitly in context.

    - **Standard Documents:**
        - **`docs/ARQUITECTURE.md`**: Defines **WHAT** & **WHY**. abstract design, diagrams, contracts, constraints. NO implementation code.
        - **`docs/IMPLEMENTATION.md`**: Defines **HOW**. Steps, reference code, config examples, test strategy.
    
    - **Modular Docs:** Large requirements must be split into multiple `.md` files in the `docs/` folder.
    - **Naming Convention:** Files in `docs/` must use the format `DOC_NAME.md` (UPPERCASE, separated by underscores). Example: `docs/BUS_ARCHITECTURE.md`.
    
    - **Diagram Standards:**
        - **Format**: Markdown files (`*.md`) containing Mermaid code.
        - **Location**: `docs/diagrams/`.
        - **Referencing**: MUST be linked from `ARQUITECTURE.md` and `IMPLEMENTATION.md`.

    - **Readme Indexing:** The `README.md` must act as an index. Every file in `docs/` must be linked from the `README.md` to avoid saturating it with too much information.
    - **SKILL.md:** On demand, you may be asked to generate a `docs/SKILL.md` file. This file should provide a precise, non-redundant, and comprehensive summary of the library's operation for an LLM to have complete context. It must also be linked from the `README.md`.
<!-- END_SECTION:DOCUMENTATION -->

<!-- START_SECTION:PROTOCOLS -->
- **Language Protocol:** Plans must always be in **English**, while chat conversation must be in **Spanish**.

- **Strategic Justification:** In planning mode, always provide the rationale for the solution. Justify it based on best practices and current industry standards. Provide alternatives, select the best option, and explain why it is the best fit for the context.

- **Modular Documentation:** If the requirement is large, split it into multiple `.md` files in the format/location `[LIBRARY_NAME]/docs/[PLAN_NAME].md` of the involved library. The central plan must orchestrate these files.

- **Explicit Execution:** Never start coding unless explicitly told to "execute the plan","ok" or "ejecuta" (in English or Spanish).
<!-- END_SECTION:PROTOCOLS -->

<!-- START_SECTION:WASM -->
- **WebAssembly Environment (tinywasm):** Use `tinywasm` for WASM projects. Running it without parameters scaffolds `web/client.go` with basic code, compiles front/back in-memory, and starts an MCP server on port 3030 with hot-reload. This provides tools for monitoring, browser automation (logs, screenshots), and manual recompilation without polluting the project. **Important:** `tinywasm` is a TUI application — never run it from your own shell (it will block indefinitely). The developer starts it in their IDE terminal or an external terminal. You interact exclusively via the MCP server on port 3030.

- **Frontend Go Compatibility:** If the Go code destination is the frontend (WebAssembly), maximum compatibility with TinyGo is required, as this is the focus of the framework. Consequently, the standard library should not be used for this purpose; for example, use `tinywasm/fmt` instead of `fmt`, `strings`, `strconv`, `errors`, and `path/filepath`; also use `tinywasm/time` instead of `time`, and `tinywasm/json` instead of `encoding/json`.


- **Frontend Optimization:** Avoid using maps in WebAssembly/Frontend code if possible. TinyGo's map implementation increases binary size and runtime overhead significantly. Use structs or slices for small collections instead.
<!-- END_SECTION:WASM -->

<!-- START_SECTION:USER_CUSTOM -->
<!-- This section is preserved during sync. Add your custom LLM instructions here. -->
<!-- END_SECTION:USER_CUSTOM -->
