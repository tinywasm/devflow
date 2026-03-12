---
name: documentation
description: Documentation standards for ARCHITECTURE.md, PLAN.md, DESIGN.md, SPECS.md, SKILL.md, diagrams, and README indexing. Use when creating or updating project documentation.
---

# Documentation

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
