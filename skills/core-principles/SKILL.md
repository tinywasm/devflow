---
name: core-principles
description: Go development principles including SRP, dependency injection, framework-less web, CSS-first interactivity, and file structure constraints. Use for any Go project setup or code review.
---

# Core Principles

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
