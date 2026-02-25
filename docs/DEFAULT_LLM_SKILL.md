<!-- START_SECTION:CORE_PRINCIPLES -->
- **Single Responsibility Principle (SRP):** Every file (CSS, Go, JS) must have a single, well-defined purpose. This must be reflected in both the file's content and its naming convention.

- **Mandatory Dependency Injection (DI):**
    - **No Global State:** Avoid direct system calls (OS, Network) in logic.
    - **Interfaces:** Define interfaces for external dependencies (`Downloader`, `ProcessManager`).
    - **Composition:** Main structs must hold these interfaces.
    - **Injection:** `cmd/<app_name>/main.go` is the ONLY place where "Real" implementations are injected.

- **Framework-less Development:** For Web projects, use only the **Standard Library** (HTML/CSS/JS). No external frameworks or libraries are allowed.
- **CSS-First Interactivity:** Minimize JavaScript usage. All UI interactivity (toggles, menus, states) must be implemented using pure CSS whenever possible.
- **Minimalist JS:** Use JavaScript only as a last resort for logic that cannot be handled by CSS or the Go backend.

- **Strict File Structure:**
    - **Flat Hierarchy:** Go libraries must avoid subdirectories. Keep files in the root.
    - **Max 500 lines:** Files exceeding 500 lines MUST be subdivided and renamed by domain.
    - **Test Organization:** If >5 test files exist in the root, move **ALL** tests to a `tests/` directory.
<!-- END_SECTION:CORE_PRINCIPLES -->

<!-- START_SECTION:TESTING -->
- **Testing Runner (`gotest`):** For Go tests, ALWAYS use the globally installed `gotest` CLI command. **DO NOT** use `go test` directly, and **DO NOT** invoke it via `go run github.com/tinywasm/devflow/cmd/gotest`. Simply type `gotest` (no arguments) for the full suite, or `gotest -run TestName`. It automatically handles `-vet`, `-race`, `-cover`, WASM tests, and README badges.
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
<!-- END_SECTION:TESTING -->

<!-- START_SECTION:DOCUMENTATION -->
- **Documentation First:** You MUST update the documentation *before* coding or running `gopush`.
- **Context-Aware Rule Compilation:** Every `IMPLEMENTATION.md` MUST start with a "Development Rules" section that copies/pastes the relevant constraints from this skill file (e.g., WASM restrictions, DI rules).

- **Standard Documents:**
    - **`docs/ARQUITECTURE.md`:** Defines WHAT & WHY (abstract design, constraints). NO implementation code.
    - **`docs/IMPLEMENTATION.md`:** Defines HOW (steps, reference code, test strategy).
    - **`docs/SKILL.md`:** (On demand) Provides an LLM-friendly, highly condensed summary of the library's context and constraints.
    - **Modular Docs:** If `ARQUITECTURE.md` or `IMPLEMENTATION.md` become too large, they must be divided into domain-specific, uppercase, underscore-separated files (e.g., `docs/BUS_ARCHITECTURE.md`).

- **Diagram Standards:**
    - **Format & Location:** Markdown files (`*.md`) containing Mermaid code, stored in `docs/diagrams/` and linked from the architecture documents.
    - **Simplicity:** Use simple, vertical, linear flowcharts (`flowchart TD`). **NEVER** use the `subgraph` directive (ruins TUI rendering). Use `<br/>` for line breaks inside standard nodes instead of quoting text strings.

- **Readme Indexing:** The `README.md` must act as an index. Every file in `docs/` must be linked from `README.md` to avoid saturating it.
<!-- END_SECTION:DOCUMENTATION -->

<!-- START_SECTION:PROTOCOLS -->
- **Language Protocol:** Plans and documentation must be generated in **English**, while chat conversations with the user must be in **Spanish**.
- **Strategic Justification:** Always provide the rationale for your chosen solution based on best practices, offering alternatives and explaining why the selection fits the context.
- **Explicit Execution:** Never start writing/modifying actual codebase source code unless explicitly told to "execute the plan", "ok", or "ejecuta".
<!-- END_SECTION:PROTOCOLS -->

<!-- START_SECTION:WASM -->
- **WebAssembly Environment (`tinywasm`):**
    - **Global MCP Server:** The LLM interacts with projects exclusively via the global MCP server on port 3030. If it is not running, the LLM must start it using the `tinywasm -mcp` command.
    - **Starting Development:** Use the `start_development` MCP tool to run the project compiler and file watcher in the background (headless mode). **Do NOT** run `tinywasm` directly in a shell yourself to start a project.
    - **TUI Client (Human):** The human developer attaches to live logs by running `tinywasm` in their terminal (acting as a view-only SSE client). If they press `Ctrl+C`, the TUI closes but the project continues compiling/running in the background for you. To fully stop the active project, they press `q`.
- **Frontend Go Compatibility:** Use standard library replacements for tinygo compatibility. Use `tinywasm/fmt` instead of `fmt`/`strings`/`strconv`/`errors`; `tinywasm/time` instead of `time`; and `tinywasm/json` instead of `encoding/json`.
- **Frontend Optimization:** Avoid using `map` declarations in WASM code to prevent binary bloat. Use structs or slices for small collections instead.
<!-- END_SECTION:WASM -->

<!-- START_SECTION:CLAUDE_CODE -->
- **Claude Code Plan Mode:**
    - **Plan Mirroring:** When Plan Mode creates a plan file at `~/.claude/plans/`, you MUST ALSO create or update `docs/PLAN.md` inside the active project's repository with the same final plan content. The system plan file is infrastructure; `docs/PLAN.md` is the canonical source of truth for execution agents.
    - **Plan Location Priority:** Always prefer writing plans inside the project repository. If Plan Mode forces a system path, treat it as a draft and mirror the final content to the project before requesting approval via `ExitPlanMode`.
<!-- END_SECTION:CLAUDE_CODE -->

<!-- START_SECTION:LLM_COLLABORATION -->
- **Guiding Other LLMs (e.g., Jules):**
    - **Universal Planning Location:** NEVER store implementation plans in agent-specific internal directories (e.g., Antigravity's `.gemini` folders). All plans MUST be saved directly within the project's repository.
    - **The Master Prompt (`docs/PLAN.md`):** Whenever a project has pending tasks, there MUST ALWAYS be an `PLAN.md` file. This acts as the entry point for execution agents. It is the master orchestrator and MUST link to all other relevant documents (`README.md`, `ARQUITECTURE.md`, `IMPLEMENTATION.md`, or modular docs).
    - **Planning Process (Conversational First):** Execution agents don't always know where to start or what to check. Therefore, the creation of `PLAN.md` MUST be done in structured, sequential steps. The planning agent MUST first perform a Q&A process with the user in the chat (asking targeted questions and offering suggestions) to define the full scope and make architectural decisions. 
    - **Final Resolutions Only:** The Q&A discussion MUST remain in the chat. The `PLAN.md` file must ONLY contain the final resolutions, structured into clear, sequential execution steps. It must not contain the conversational history.
    - **Modular Context:** Avoid massive, monolithic instruction files. Break down instructions by domain into separate, focused implementation guides (e.g., `CHART_BAR_IMPL.md`).
    - **Legacy Reference Code:** If porting established logic (e.g., mathematics or physics formulas), append snippets of the legacy reference code at the bottom of the modular context files. Explicitly instruct the downstream LLM on which logic to recycle and which legacy dependencies to replace with the new architecture calls.
<!-- END_SECTION:LLM_COLLABORATION -->

<!-- START_SECTION:USER_CUSTOM -->
<!-- This section is preserved during sync. Add your custom LLM instructions here. -->
<!-- END_SECTION:USER_CUSTOM -->
