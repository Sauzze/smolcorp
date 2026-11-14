# Tasks

Three small tasks, each one an agent can finish and prove in one sitting.

1. **The shares don't add up.** `python3 bill.py 100 --people 3` prints three
   shares of $33.33 and a total of $99.99: a cent is lost. Make the shares add
   up to the bill exactly (the first people pay a cent more), with a test that
   proves it.

2. **`parse_amount` has no tests.** Add tests for "12", "12.5", "$12.50" and
   " 12.50 ". Then make a bad amount ("abc", "-5") end with a clear message
   instead of a traceback, with a test for each.

3. **The README doesn't say how to use it.** Add a Usage section with two
   examples, each with the output the program really prints.

Proof for each: `python3 -m unittest` passes, and the change is on its own
branch.
