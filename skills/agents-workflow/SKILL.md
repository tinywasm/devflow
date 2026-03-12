---
name: agent-workflow
description: Multi-agent planning workflow with PLAN.md as master orchestrator, stage-driven execution, modular context files, and codejob dispatch. Use when creating plans for execution agents like Jules.
---

# CodeJob Agent Workflow

- **Guiding Other LLMs (e.g., Jules):**
    - **Universal Planning Location:** NEVER store implementation plans in agent-specific internal directories (e.g., Antigravity's `.gemini` folders). All plans MUST be saved directly within the project's repository.
    - **The Master Prompt (`docs/PLAN.md`):** Whenever a project has pending tasks, there MUST ALWAYS be a `PLAN.md` file. This acts as the entry point for execution agents. It is the master orchestrator and MUST link to all other relevant documents (`README.md`, `ARCHITECTURE.md`, or modular docs).
    - **Planning Process (Conversational First):** Execution agents don't always know where to start or what to check. Therefore, the creation of `PLAN.md` MUST be done in structured, sequential steps. The planning agent MUST first perform a Q&A process with the user in the chat (asking targeted questions and offering suggestions) to define the full scope and make architectural decisions. 
    - **Final Resolutions Only:** The Q&A discussion MUST remain in the chat. The `PLAN.md` file must ONLY contain the final resolutions, structured into clear, sequential execution steps. It must not contain the conversational history.
    - **Modular Context & Stage-Driven Execution:** Avoid massive, monolithic instruction files. For complex features, use `PLAN.md` as a master orchestrator (an index or checklist) and break down instructions into separate, explicit, sequentially numbered stage files (e.g., `PLAN_STAGE_1_MODELS.md`, `PLAN_STAGE_2_CORE.md`).
    - **Navigation within Stages:** Each stage file MUST include navigation links at the top to previous and next stages (e.g., `← [Stage 1](PLAN_STAGE_1_MODELS.md) | Next → [Stage 3](PLAN_STAGE_3_CORE.md)`). By dividing tasks into separate stage files, the executing LLM processes them sequentially and effectively without losing context or skipping steps.
    - **Legacy Reference Code:** If porting established logic (e.g., mathematics or physics formulas), append snippets of the legacy reference code at the bottom of the modular context files. Explicitly instruct the downstream LLM on which logic to recycle and which legacy dependencies to replace with the new architecture calls.
    - **Dispatching Work (`codejob`):** After creating or updating `docs/PLAN.md`, use the `codejob` CLI to orchestrate the task.
        - **1. Dispatch**: Run `codejob` (no args) to send `docs/PLAN.md` to the agent.
        - **2. Review**: When the agent finishes, `codejob` automatically renames `docs/PLAN.md` to `docs/CHECK_PLAN.md`, fetches changes, and switches to the agent's branch for local review.
        - **3. Resolve**:
            - **Approve**: If the work is correct, run `codejob 'commit message' [tag]` to merge the PR and publish via `gopush`.
            - **Iterate**: If adjustments are needed, create a **new** `docs/PLAN.md` and run `codejob` again. The tool will merge the old PR, delete `CHECK_PLAN.md`, and dispatch the new plan.
        - **Note**: `codejob` is a **local developer tool** — it MUST **NEVER** appear inside `PLAN.md` or any plan sent to an agent. Its purpose is orchestration, not execution.
