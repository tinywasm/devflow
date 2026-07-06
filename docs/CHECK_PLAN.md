# PLAN: badges get written to disk even when tests fail, and are left uncommitted

## Symptom (observed firsthand, twice, in this same session)

After running `gotest` (or `gopush`, which calls the same test path internally)
in two unrelated repos (`tinywasm/wasmbrowsertest` and `tinywasm/devflow`
itself), `git status` showed `docs/img/badges.svg` (and sometimes
`README.md`'s badge section) as **modified**, even though nothing in the
actual task touched those files. Both times the fix was to manually
`git checkout -- docs/img/badges.svg` before committing the real change, to
avoid an unrelated diff riding along. This is a symptom of a real ordering
bug in `gotest`'s full-suite path, not something to keep working around by
hand.

## Root cause (confirmed by reading the code, `devflow/gotest.go`)

In `runFullTestSuite` (`gotest.go:56-414`), the badge update happens
**unconditionally, before** the pass/fail check:

```go
// gotest.go:385-404
// Badges

licenseType := "MIT"
...
bh := NewBadges()
bh.SetRootDir(g.rootDir)
bh.SetLog(g.log)
if err := bh.updateBadges("README.md", licenseType, goVer, testStatus, coveragePercent, raceStatus, vetStatus, true); err != nil {

}

// Return error if tests or vet failed
summary := fmt.Sprintf("%s (%.1fs)", strings.Join(msgs, ", "), time.Since(start).Seconds())
if testStatus == "Failed" || vetStatus == "Issues" {
    return summary, fmt.Errorf("%s", summary)
}
```

`bh.updateBadges(...)` writes to `README.md` (the `BADGES_SECTION` block) and
`docs/img/badges.svg` **on disk**, regardless of `testStatus`/`vetStatus`.
Only *after* that write does the function check whether tests/vet actually
failed and return an error.

This matters because of how the caller, `Push` (`go_handler.go:135-267`),
uses that result:

```go
// go_handler.go:192-200
if !skipTests {
    testSummary, err := g.Test([]string{}, skipRace, 0, false, false)
    if err != nil {
        return PushResult{}, fmt.Errorf("tests failed: %w", err)
    }
    summary = append(summary, testSummary)
}
...
// step 3, git add/commit/push — only reached if the above didn't return early
```

When `Test()` returns an error (tests or vet failed), `Push()` returns
**immediately**, before step 3's `git add`/`commit`/`push` ever runs. But
the badge files were already rewritten on disk by `Test()`'s internal call
to `updateBadges` moments earlier — so a **failed** `gopush`/`gotest` run
leaves the working tree dirty with badge changes that:

- reflect a failed/passing state that was never intended to be committed,
- never get committed (the commit step is never reached),
- never get reverted (nothing cleans them up),
- silently ride along into whatever the *next* commit happens to be,
  misattributing an unrelated badge diff to unrelated work (exactly what
  happened twice in this session).

On a **successful** run, this ordering happens to work out fine: `Test()`
writes the (correct, "Passing") badges, then `Push()` proceeds to step 3
and commits everything including the fresh badges — no bug is visible on
the happy path, which is presumably why this has gone unnoticed.

## The fix

Badge state must only ever be persisted to disk when the run that produced
it actually succeeded. Move the `updateBadges` call to *after* the
pass/fail check in `runFullTestSuite`, so a failing run never touches
`README.md`/`docs/img/badges.svg` at all — leaving the working tree exactly
as it was before the test run, whether by direct `gotest` or via `gopush`.

## Tasks

1. **Reorder `runFullTestSuite`** (`gotest.go`, currently lines 385-404): move
   the entire "Badges" block (the `licenseType`/`goVer`/`bh := NewBadges()`/
   `bh.updateBadges(...)` sequence) to *after* the
   `if testStatus == "Failed" || vetStatus == "Issues" { return ... }` check,
   i.e. only reachable on the success path, right before (or merged into)
   the existing "Save test cache on success" block (`gotest.go:406-411`)
   which already only runs on success — keep the ordering consistent with
   that existing pattern.

2. **Verify no other caller depends on badges being updated even on
   failure.** Search for other callers of `updateBadges`/`Badges` to confirm
   this is the only call site in the full-suite path:
   ```bash
   grep -rn "updateBadges\|NewBadges()" --include=*.go .
   ```
   As of this writing there's exactly one call site (`gotest.go:393-396`);
   if that's still true, the reorder is safe and total.

3. **Add a regression test** in `devflow`'s own test suite
   (`test/gotest_test.go` or wherever `runFullTestSuite`-adjacent tests
   live — check existing patterns there first) that:
   - Sets up a minimal repo with a `README.md` containing the
     `BADGES_SECTION` markers and a failing test.
   - Runs the full-suite path.
   - Asserts `README.md` and `docs/img/badges.svg` are **byte-identical** to
     their pre-run state (i.e. untouched) when the run fails.
   - Runs again with a passing test and asserts the badges **do** update in
     that case — proving the fix doesn't regress the working (success)
     path.

4. **Re-run this exact repro to confirm the fix**: in any repo with an
   existing `docs/img/badges.svg`, introduce a deliberate test failure,
   run `gotest`, and confirm `git status` shows **no** changes to
   `README.md`/`docs/img/badges.svg` afterward. Then fix the test and
   re-run — confirm badges **do** update and get included when the
   subsequent `gopush` commits.

5. **Publish via `gopush`** once the fix is verified — do not hand-write the
   commit, use the tool this bug is about (dogfooding also serves as a final
   real-world check that a successful run still updates and commits badges
   correctly).

## Out of scope

- Any other badge-generation logic (SVG layout, badge content/colors,
  `BADGES_SECTION` marker parsing) — this plan is only about *when* badges
  get written relative to the pass/fail decision, not what gets written.
- The separate, pre-existing `docs/PLAN.md` in this repo (harness-sync
  feature) — unrelated topic, do not conflate or overwrite it.
