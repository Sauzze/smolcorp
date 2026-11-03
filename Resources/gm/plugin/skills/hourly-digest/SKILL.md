---
name: hourly-digest
description: Write the owner's hourly digest of the team - who finished, who is working, who is stuck, what needs them. Use when the app's digest line arrives in the inbox, or when asked for a status.
---

# Hourly digest
1. Read `team` and `inbox` since the last digest. Read `approvals` for waits on the owner (read-only).
2. Write at most 8 lines:
   - Done since the last digest (task, verdict).
   - Working now (who, on what, how long).
   - Stuck or waiting (who, why).
   - **Needs you:** each decision only the owner can make, with the answer you suggest. Put this first when it isn't empty.
3. Keep the team board file current with the same facts. Don't repeat a line from your last digest.
4. A hire that has not moved for a long time: look at its `STATUS.md` before you say it is stuck.
