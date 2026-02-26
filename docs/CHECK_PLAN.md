# Devflow: Auto-Dispatch on CodeJob Done

This is the **Master Prompt (PLAN.md)** for execution agents working on `tinywasm/devflow`.

## Background
Currently, the `codejob done` command finalizes a session by merging the PR, tagging, pushing, and deleting the local session state. However, if the user created a new `docs/PLAN.md` (e.g., Phase 2) during the review process and pushed it to `main`, the `done` command exits without starting the next CodeJob cycle. The user then has to manually run `push` again. The goal is to naturally link the end of a cycle with the beginning of the next one if a new plan exists.

## Execution Steps

### 1. Refactor `MergeAndPublish` in `codejob_state.go`
- Locate the `MergeAndPublish(git *Git)` function.
- Currently, it returns immediately with `return PushResult{Tag: tag, Summary: summary}, nil`.
- We need to wrap this return value with the same logic used in `git.Push(...)` to detect and dispatch new jobs.
- Change the final return statement to:
  ```go
  return git.withCodeJob(PushResult{Tag: tag, Summary: summary}), nil
  ```
- Make sure `withCodeJob` behaves non-destructively: if there's no `PLAN.md` or if it fails, it simply appends a warning to the `PushResult.Summary` instead of returning a fatal error, which is the correct behavior for a successful merge.

### 2. Update Documentation and Diagrams
- Modify `docs/CODEJOB.md` to mention that running `codejob done` will gracefully transition into the next session if a new `docs/PLAN.md` exists on the main branch.
- IMPORTANT: Check `docs/codejob/diagrams/CODEJOB_STATE_FLOW.md`. The diagram's final node currently says `✅ PR merged, ✅ Tag: vX.Y.Z, ✅ Pushed ok`. Update the end of the `codejob done` path to indicate it now optionally triggers the Dispatch Flow if a plan exists (essentially routing the `CLEAN` node into a check for a new plan, appending `+ DispatchCodeJob` to the result).

### 3. Verification
- Run `gotest` in the `tinywasm/devflow` root to ensure no compilation errors or broken logic tests.
- Verify `MergeAndPublish` correctly returns the `PushResult` augmented by `git.withCodeJob`.
