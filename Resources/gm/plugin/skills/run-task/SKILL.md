---
name: run-task
description: Run one task end to end - hire a writer, check the tests, get an independent review, one fix round, a fresh re-check, then report. Use for every task in the plan.
---

# Run a task
1. **Brief.** Write the brief in your folder: job, "done", proof (commands and results), off-limits. Say the writer reports in its `STATUS.md` and ends with `NEEDS-REVIEW:` when ready.
2. **Hire the writer** on Sonnet (`hire`, role fitting the task, its own worktree and branch). Within a minute check in `team` that it started and shows a prompt.
3. **Wait** for `inbox` to tell you of `NEEDS-REVIEW:`. Don't poll; check on the team every 10 to 20 minutes.
4. **Tests.** Read the writer's proof. Run the tests yourself in its worktree (the real path, not a copy). Failing: `brief` it back with the exact failure.
5. **Review.** Hire a *reviewer* (a different hire, read-only) and give it the diff and the brief, not your conclusion. It writes a verdict file: PASS or FAIL, with exact fixes.
6. **One fix round.** On FAIL, send the fixes to the writer once. Then hire a *fresh* reviewer for a re-check; the first one has seen too much.
7. **Still FAIL:** stop. Put it in the report under "What needs you" with both verdicts.
8. **PASS:** report to the owner (what changed, proof, verdict) and use `fire-and-recap`. Merging, pushing and deploying stay the owner's.
