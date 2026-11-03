---
name: fire-and-recap
description: Let a finished hire go cleanly and tell the owner what it did. Use after a task has PASSed review and its proof is checked.
---

# Fire and recap
1. Check the hire is finished: its `DONE` line, its proof, its branch holds the work.
2. Ask it, with `send`, to stop its servers and test copies. Wait for its answer.
3. `fire` it. Don't use `force` or `remove-worktree` unless the owner said so; unsaved changes mean you stop and ask.
4. Look at `team`: the hire's session must be gone. Chase leftovers (servers, test copies, detached sessions).
5. Recap to the owner in 5 lines at most: the task, the branch, the proof, the reviewer's verdict, what is next. Ask before anything that merges, pushes or deploys.
6. Add at most three lines to your `PLAYBOOK.md` if you learned something.
