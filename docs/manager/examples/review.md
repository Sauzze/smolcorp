# Example brief: a review

**Role:** reviewer · **Model:** Opus · **Folder:** a read-only worktree of `shop` on `fix/cart-total`

## The job
Review the branch `fix/cart-total` against `main`. You get the diff, not the builder's explanation: work out for yourself whether the fix is right.

## Done means
A verdict in `STATUS.md`: `PASS`, or `FAIL` with a numbered list. Each finding names the file and line, what goes wrong, and an input that shows it.

## Off-limits
- You are read-only: don't edit, commit, check out or stash anything.
- Style nits come last and short. Don't rewrite the builder's approach.
- Don't run anything that needs a key or touches a real account.

## Proof I want
The commands you ran (tests, the reproduction) and their results. A finding you couldn't check says so.
