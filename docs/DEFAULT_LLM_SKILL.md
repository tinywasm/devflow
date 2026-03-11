<!-- START_SECTION:CORE_PRINCIPLES -->
- **Single Responsibility Principle (SRP):** Every file (CSS, Go, JS) must have a single, well-defined purpose. This must be reflected in both the file's content and its naming convention.

- **Mandatory Dependency Injection (DI):**
    - **No Global State:** Avoid direct system calls (OS, Network) in logic.
    - **Interfaces:** Define interfaces for external dependencies (`Downloader`, `ProcessManager`).
    - **Composition:** Main structs must hold these interfaces.
    - **Injection:** `cmd/<app_name>/main.go` is the ONLY place where "Real" implementations are injected.
    - **Thin Main / Fat Library:** `cmd/*/main.go` files MUST be minimal — only argument parsing and dependency injection. ALL business logic MUST live in exported, testable library functions. Never put orchestration logic, conditionals, or error handling beyond basic print/exit in main.

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
<!-- END_SECTION:TESTING -->

<!-- START_SECTION:DOCUMENTATION -->
- **Documentation First:** You MUST update the documentation *before* coding or running `gopush`.
- **Context-Aware Rule Compilation:** Every `PLAN.md` MUST start with a "Development Rules" section that copies/pastes the relevant constraints from this skill file (e.g., WASM restrictions, DI rules).

- **Standard Documents:**
    - **`docs/ARCHITECTURE.md`:** Defines WHAT & WHY (abstract design, constraints). NO implementation code.
    - **`docs/PLAN.md`:** Defines HOW (steps, reference code, test strategy). It is the master orchestrator for execution.
    - **`docs/DESIGN.md`:** (On demand) Justifies technical decisions and explores alternatives. Must NOT duplicate `ARCHITECTURE.md`. Heavily linked by `ARCHITECTURE.md` to keep the main document clean and focused on abstract structure rather than debate.
    - **`docs/SPECS.md`:** (On demand) Strict functional requirements, exact inputs/outputs, and data logic. Must NOT duplicate `ARCHITECTURE.md`. Links closely to `PLAN.md` to guide exact test cases and assertions.
    - **`docs/SKILL.md`:** (On demand) Provides an LLM-friendly, highly condensed summary of the library's context and constraints.
    - **Modular Docs:** If `ARCHITECTURE.md` or `PLAN.md` become too large, they must be divided into domain-specific, uppercase, underscore-separated files (e.g., `docs/BUS_ARCHITECTURE.md`, `docs/CHART_BAR_PLAN.md`).

- **Diagram Standards:**
    - **Format & Location:** Markdown files (`*.md`) containing Mermaid code, stored in `docs/diagrams/` and linked from the architecture documents.
    - **Simplicity:** Use simple, vertical, linear flowcharts (`flowchart TD`). **NEVER** use the `subgraph` directive (ruins TUI rendering). Use `<br/>` for line breaks inside standard nodes instead of quoting text strings.

- **Readme Indexing:** The `README.md` must act as an index. Every file in `docs/` must be linked from `README.md`. Cross-link logically related documents (e.g., `ARCHITECTURE.md` or `PLAN.md` contextually linking to `SPECS.md` or `DESIGN.md`) to avoid duplicating information across files.
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
    - **The Master Prompt (`docs/PLAN.md`):** Whenever a project has pending tasks, there MUST ALWAYS be a `PLAN.md` file. This acts as the entry point for execution agents. It is the master orchestrator and MUST link to all other relevant documents (`README.md`, `ARCHITECTURE.md`, or modular docs).
    - **Planning Process (Conversational First):** Execution agents don't always know where to start or what to check. Therefore, the creation of `PLAN.md` MUST be done in structured, sequential steps. The planning agent MUST first perform a Q&A process with the user in the chat (asking targeted questions and offering suggestions) to define the full scope and make architectural decisions. 
    - **Final Resolutions Only:** The Q&A discussion MUST remain in the chat. The `PLAN.md` file must ONLY contain the final resolutions, structured into clear, sequential execution steps. It must not contain the conversational history.
    - **Modular Context & Stage-Driven Execution:** Avoid massive, monolithic instruction files. For complex features, use `PLAN.md` as a master orchestrator (an index or checklist) and break down instructions into separate, explicit, sequentially numbered stage files (e.g., `PLAN_STAGE_1_MODELS.md`, `PLAN_STAGE_2_CORE.md`).
    - **Navigation within Stages:** Each stage file MUST include navigation links at the top to previous and next stages (e.g., `← [Stage 1](PLAN_STAGE_1_MODELS.md) | Next → [Stage 3](PLAN_STAGE_3_CORE.md)`). By dividing tasks into separate stage files, the executing LLM processes them sequentially and effectively without losing context or skipping steps.
    - **Legacy Reference Code:** If porting established logic (e.g., mathematics or physics formulas), append snippets of the legacy reference code at the bottom of the modular context files. Explicitly instruct the downstream LLM on which logic to recycle and which legacy dependencies to replace with the new architecture calls.
    - **Dispatching Work (`codejob`):** After creating or updating `docs/PLAN.md`, if `codejob` is installed locally (verify with `which codejob`), use it to dispatch the task to the external agent: `codejob` (no args) dispatches `docs/PLAN.md`, and `codejob 'commit message'` closes the loop after reviewing the PR (merge, publish, update deps). `codejob` is a **local developer workflow tool** — it MUST **NEVER** appear inside `PLAN.md` or any plan sent to an external agent. Its purpose is to trigger the external agent, not to be part of its instructions.
<!-- END_SECTION:LLM_COLLABORATION -->

<!-- START_SECTION:USER_CUSTOM -->
<!-- This section is preserved during sync. Add your custom LLM instructions here. -->
<!-- END_SECTION:USER_CUSTOM -->
