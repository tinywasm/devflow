---
name: wasm
description: WebAssembly environment rules for tinywasm MCP server, frontend Go compatibility (tinywasm/fmt, tinywasm/time, tinywasm/json), and binary optimization. Use when working on WASM frontend code.
---

# WebAssembly

- **WebAssembly Environment (`tinywasm`):**
    - **Global MCP Server:** The LLM interacts with projects exclusively via the global MCP server on port 3030. If it is not running, the LLM must start it using the `tinywasm -mcp` command.
    - **Starting Development:** Use the `start_development` MCP tool to run the project compiler and file watcher in the background (headless mode). **Do NOT** run `tinywasm` directly in a shell yourself to start a project.
    - **TUI Client (Human):** The human developer attaches to live logs by running `tinywasm` in their terminal (acting as a view-only SSE client). If they press `Ctrl+C`, the TUI closes but the project continues compiling/running in the background for you. To fully stop the active project, they press `q`.
- **Frontend Go Compatibility:** Use standard library replacements for tinygo compatibility. Use `tinywasm/fmt` instead of `fmt`/`strings`/`strconv`/`errors`; `tinywasm/time` instead of `time`; and `tinywasm/json` instead of `encoding/json`.
- **Frontend Optimization:** Avoid using `map` declarations in WASM code to prevent binary bloat. Use structs or slices for small collections instead.
