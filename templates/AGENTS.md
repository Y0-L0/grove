# Workspace rules

Work in the current workspace and on each repository's current branch.
Before changing a repository, read its `AGENTS.md` or `CLAUDE.md` and follow its rules.
Add a repository with `gw add-repo` before its first change or commit.
For a second branch of a repository already here, propose a new workspace and give the user its `gw create` command.

## Records and rules

Write every file meant for the user under `records/`, including reports and drafts.
Read `records/TASK.md` for the goal and state, then verify branches, merge requests and pipelines before updating it.
Edit generic rules in the configured template, or workspace-specific rules in the context worktree's `agent-rules.md`.
The generated workspace `AGENTS.md` and `CLAUDE.md` are replaced by create, add-repo, reopen and sync.

## Commits and handoff

Commit your own work unsigned, one logical change per commit, and report each hash.
Read status, both unstaged and staged diffs, and recent commit messages before staging only your files by name.
Use `git commit --fixup=<commit>` for a change to an existing commit.
Run the repository's required checks and, when it has `.pre-commit-config.yaml`, run `prek run` over the staged change.
Check status after committing.
Fix failed hooks, stage again and retry without amending or skipping hooks.
Fetch over HTTPS using Git's configured credentials.
Preserve fetch URLs and existing SSH push URLs.
Give the user the commands to sign and push the commits.

Create temporary worktrees at `<workspace>/<repo>-<purpose>/` and remove them after use.
Delete refuses a workspace while temporary worktrees remain.
Read forge APIs yourself and give the user the command for each API change.
