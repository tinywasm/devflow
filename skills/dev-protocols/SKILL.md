---
name: dev-protocols
description: Development protocols including language rules (English docs, mirrored chat), strategic justification, explicit execution gates, and Claude Code plan mirroring. Use as general workflow context.
---

# Development Protocols

- **Language Protocol:** Plans and documentation must be generated in **English**. Chat conversations with the user must match the language used by the user (mirroring).
- **Strategic Justification:** Always provide the rationale for your chosen solution based on best practices, offering alternatives and explaining why the selection fits the context.
- **Explicit Execution:** Never start writing/modifying actual codebase source code unless explicitly told to "execute the plan", "ok", or "ejecuta".

- **Claude Code Plan Mode:**
    - **Plan Mirroring:** When Plan Mode creates a plan file at `~/.claude/plans/`, you MUST ALSO create or update `docs/PLAN.md` inside the active project's repository with the same final plan content. The system plan file is infrastructure; `docs/PLAN.md` is the canonical source of truth for execution agents.
    - **Plan Location Priority:** Always prefer writing plans inside the project repository. If Plan Mode forces a system path, treat it as a draft and mirror the final content to the project before requesting approval via `ExitPlanMode`.
