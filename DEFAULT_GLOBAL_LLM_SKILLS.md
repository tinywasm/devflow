<!-- START_SECTION:CORE_PRINCIPLES -->
- **Single Responsibility Principle (SRP):** Every file (CSS, Go, JS) must have a single, well-defined purpose. This must be reflected in both the file's content and its naming convention.

- **Framework-less Development:** For HTML/Web projects, use only the **Standard Library**. No external frameworks or libraries are allowed.

- **CSS-First Interactivity:** Minimize JavaScript usage. All UI interactivity (toggles, menus, states) must be implemented using pure CSS whenever possible.

- **Minimalist JS:** Use JavaScript only as a last resort for logic that cannot be handled by CSS or the Go backend.
<!-- END_SECTION:CORE_PRINCIPLES -->

<!-- START_SECTION:TESTING -->
- **Testing:** For Go tests, always use `gotest` (`github.com/tinywasm/devflow/cmd/gotest`). It automatically runs `vet`, standard tests with `-race` and `-cover`, and detects/runs WASM tests. It features intelligent caching (based on git state) for instant feedback on unchanged code, and updates README badges automatically.

- **Publishing:** If all tests pass when using `gotest`, you can publish changes using `gopush` (`github.com/tinywasm/devflow/cmd/gopush`). This command runs tests, commits, tags, pushes, and updates dependent modules automatically.
<!-- END_SECTION:TESTING -->

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
