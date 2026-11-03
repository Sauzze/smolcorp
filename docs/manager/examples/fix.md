# Example brief: a fix

**Role:** fixer · **Model:** Sonnet · **Folder:** a worktree of `shop`, branch `fix/cart-total`

## The job
The cart total shows the price without the discount when a coupon and a gift card are both used. Reproduce it, fix it, add a test that fails before and passes after.

## Done means
- A test in `tests/cart.test.ts` that fails on the old code and passes on yours.
- The whole test suite passes. Paste the final result line into `STATUS.md`.
- One commit on your branch, with a message that says what was wrong.

## Off-limits
- Don't touch the checkout page or the price rules; if the bug is there, write `NEEDS-REVIEW:` and say so.
- Don't push, merge or deploy.
- Anything else you find goes in `STATUS.md` under "Found, not fixed".

## Proof I want
The reproducing steps, the test's output before and after, the commit hash.
