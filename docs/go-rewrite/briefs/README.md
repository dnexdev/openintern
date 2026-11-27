# Workstream briefs

Each file is the full brief for one workstream. Any agent (re)starting a
workstream reads `../PLAN.md`, its brief here, and its status file in
`../status/`, then continues from where the status file says work stopped.

Common to every brief (also PLAN.md "Rules for every agent"):

- Commits are allowed (maintainer, 2026-11-26): commit only paths your brief
  owns, with `git add <your paths>` then `git commit` (never `git add -A`/`.`,
  never `commit -a`). Plain messages, no AI credit or trailers. Never push,
  and never run stash, checkout, switch, restore, reset, clean, rebase or
  merge: other agents share this checkout. If the index lock is busy, retry.
- Do NOT credit yourself or any AI, model, agent or tool anywhere: no
  Co-Authored-By, no "Generated with", no model or agent names in code,
  comments, docs or status files. Write as the project.
- Edit only the paths your brief lists. Other agents share this checkout: never
  touch, reformat, revert or delete their files, never kill processes you did
  not start.
- Go standard library only; `go.mod` stays free of `require` lines.
- Usage limits can stop any run without warning. Update your status file after
  every package or milestone so a successor can resume exactly where you
  stopped (what is done, what is half-done, what is next).
- `~` is mirrored to other machines by Syncthing. If `*.sync-conflict-*` files
  or long-deleted files appear in the repo, leave them alone and mention them
  in your status file; the lead cleans them up.
- No push and no GitHub write actions (PRs, issues, comments, reviews, reactions). Read-only git and gh are fine.
