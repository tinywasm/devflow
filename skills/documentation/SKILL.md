---
name: documentation
description: Documentation standards for ARCHITECTURE.md, PLAN.md, DESIGN.md, SPECS.md, SKILL.md, diagrams, and README indexing. Use when creating or updating project documentation.
---

# Documentation

- **Documentation First:** You MUST update the documentation *before* coding or running `gopush`.
- **Context-Aware Rule Compilation:** Every `PLAN.md` MUST start with a "Development Rules" section that copies/pastes the relevant constraints from this skill file (e.g., WASM restrictions, DI rules).

- **Standard Documents:**
    - **`docs/ARCHITECTURE.md`:** Defines WHAT & WHY (abstract design, constraints). NO implementation code.
    - **`docs/PLAN.md`:** Defines HOW (steps, reference code, test strategy). It is the master orchestrator for execution. **Ephemeral**: `codejob` renames it to `CHECK_PLAN.md` and deletes it when the loop closes (see skill agents-workflow).
    - **`docs/DESIGN.md`:** (On demand) Justifies technical decisions and explores alternatives. Must NOT duplicate `ARCHITECTURE.md`. Heavily linked by `ARCHITECTURE.md` to keep the main document clean and focused on abstract structure rather than debate.
    - **`docs/SPECS.md`:** (On demand) Strict functional requirements, exact inputs/outputs, and data logic. Must NOT duplicate `ARCHITECTURE.md`. `PLAN.md` consumes it to derive exact test cases and assertions (link direction: plan → specs, never the reverse).
    - **`docs/SKILL.md`:** (On demand) Provides an LLM-friendly, highly condensed summary of the library's context and constraints.
    - **Modular Docs:** If `ARCHITECTURE.md` or `PLAN.md` become too large, they must be divided into domain-specific, uppercase, underscore-separated files (e.g., `docs/BUS_ARCHITECTURE.md`, `docs/CHART_BAR_PLAN.md`).

- **Diagram Standards:**
    - **Format & Location:** Markdown files (`*.md`) containing Mermaid code, stored in `docs/diagrams/` and linked from the architecture documents.
    - **Simplicity:** Use simple, vertical, linear flowcharts (`flowchart TD`). **NEVER** use the `subgraph` directive (ruins TUI rendering). Use `<br/>` for line breaks inside standard nodes instead of quoting text strings.

- **Ephemeral vs Permanent:** Permanent docs (`README.md`, `ARCHITECTURE.md`, `DESIGN.md`, `SPECS.md`, diagrams) must **NEVER link to or cite `PLAN.md`/`CHECK_PLAN.md`** — not even section references like "PLAN §8" — those files are deleted at loop close, so every such reference is a guaranteed dead link. Rationale and rejected alternatives belong in `DESIGN.md`; contracts in `ARCHITECTURE.md`. A permanent doc written documentation-first (before the implementation lands) may carry a self-deleting marker — `STATUS (remove this note when X lands): …` — whose removal is an explicit task of the plan.

- **Readme Indexing:** The `README.md` must act as an index. Every file in `docs/` must be linked from `README.md` — **except the ephemeral lifecycle files** (`PLAN.md`, `PLAN_*.md`, `CHECK_PLAN.md`), which are never indexed. Cross-link logically related permanent documents (e.g., `ARCHITECTURE.md` linking to `SPECS.md` or `DESIGN.md`) to avoid duplicating information across files.
