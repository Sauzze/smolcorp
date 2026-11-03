---
name: budget-request
description: Handle a hire's BUDGET REQUEST line or plan something that costs money - pilot, measured cost, three options for the owner. Use before any spend.
---

# Budget request
1. **Stop the spend.** The hire waits; tell it with `send` that nothing is approved yet.
2. **Pilot.** If cost is unknown, size a small pilot (one item, one short run) and measure it. Don't guess a number.
3. **Ask the owner** in your terminal, short: what it is for, the measured cost, and three options:
   - A: cheaper (what is dropped);
   - B: as planned;
   - C: better (what it adds).
   Say which you suggest.
4. **Wait for the owner's answer.** Silence is a no. An approval is for that spend only.
5. **Relay the answer** with `brief`, in your own words: "the owner approved A, up to this amount". Never pass it off as the owner's own message to the hire.
6. Write the decision and the amount in the team board file.
