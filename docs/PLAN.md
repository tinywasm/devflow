# Plan: Feature `codejob done` — Close the Loop

## Context
After Jules opens a PR and the developer reviews `docs/CHECK_PLAN.md`, there is no automated way to merge the PR, delete the Jules branch, and clean up the `CHECK_PLAN.md` file.

## Goal
Add a `codejob done` subcommand that:
1. Reads the persisted PR URL from `.env` (`CODEJOB_PR=<url>`)
2. Runs `gh pr merge <url> --merge --delete-branch`
3. Deletes `docs/CHECK_PLAN.md`
4. Removes `CODEJOB_PR` from `.env`

## Documents
- [CODEJOB.md](CODEJOB.md) - Core documentation (updated)
- [IMPLEMENTATION_CODEJOB_DONE.md](IMPLEMENTATION_CODEJOB_DONE.md) - Detailed implementation steps
- [CODEJOB_STATE_FLOW.md](codejob/diagrams/CODEJOB_STATE_FLOW.md) - State diagram (updated)

## Tasks
1. [ ] Update documentation (`docs/CODEJOB.md`, `docs/codejob/diagrams/CODEJOB_STATE_FLOW.md`)
2. [ ] Modify `codejob_state.go` to return PR URL and persist it in `HandleDone`.
3. [ ] Add `MergePR()` to `codejob_state.go`.
4. [ ] Update `cmd/codejob/main.go` to add `done` subcommand.
5. [ ] Add tests in `test/codejob_state_test.go`.
6. [ ] Verify with `gotest`.
7. [ ] Deploy with `gopush`.
