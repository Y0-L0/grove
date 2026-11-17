---
name: grove
description: Create, inspect, sync, and delete multi-repo git worktree workspaces with the Grove CLI (`gw`). Use when the user mentions gw or Grove, wants one branch across several repos, asks for an isolated workspace or worktree for a task or PR, or wants to set up Grove. Covers setup, non-interactive flags, JSON output, and which commands destroy work.
---

# Grove (`gw`)

Grove creates a **workspace**: a directory holding one git worktree per selected repo, all on the same branch.
This fork creates workspaces under `<workspace_dir>/<type>/<name>/<repo>`,
with type `work` by default or `priv` through `--type priv`.
Existing workspaces keep their recorded paths.

## Rules (apply to every command)

- **Always pass explicit names and flags.** Without a TTY, pickers exit 1 with `interactive selection requires a terminal`. Never rely on a picker or prompt.
- **stdout is data, stderr is chatter.** Parse only `-o json|jsonl|name|path` or `--json` output. Never parse tables or stderr.
- **`gw delete` does not ask.** It checks every worktree, merges context records, then removes the workspace and its branches. Use `--force` only when the user authorizes discarding uncommitted work and unpushed code commits. Committed context records still merge, including with `--no-hooks`.
- **Confirmation prompts silently decline without input.** `gw remove-repo -r X` (without `-f`) and `gw create --replace` (without `-f`) print a prompt, do nothing, and exit 0. Check the result; do not assume success from the exit code.
- **Do not pass `-n`/`--no-hooks`** unless the user asks. Hooks may enforce the user's policy (for example a `pre_delete` that blocks deletion).
- **Do not run `prune --yes`, `prune branches --yes`, `reset --discard`, `doctor --fix`, or `create --replace -f`** without the user's explicit go-ahead. Run the read-only form first and show the result.
- **Do not edit the user's shell rc or `~/.grove/config.toml`** unprompted. Print the exact line or snippet and let the user apply it.

## 1. Check Grove is ready

```bash
gw --version
gw repos -o name
```

Done when `gw repos` lists the repos the user cares about.

- `gw: command not found` → install (see Setup below).
- `Grove not initialized` → `gw init <dir-containing-repos>` (the dirs must exist), then rerun `gw repos -o name`.
- A repo is missing → `gw add-dir <dir>`; discovery scans each registered dir for git repos.

## 2. Create a workspace

Pick repos by name, preset, or `--all`:

```bash
gw preset list --json                      # optional: named repo groups
gw create fix-auth -r api,web              # branch from configured owner_prefix
gw create home --type priv -r api          # private workspace
gw create fix-auth -b fix/auth-expiry -r api,web
gw create -b feat/login -p backend         # name derived from branch: feat-login
gw create pr-123 -b their-branch -r api --track   # check out an existing remote branch
```

Without `--branch`, configure `owner_prefix` in `~/.grove/config.toml` first.
The default branch is `<owner_prefix>/<name>`.
When the user requests `--no-hooks`, the default branch is the workspace name alone,
and `owner_prefix` is not required.
An explicit `--branch` overrides that default; `--track` requires it.
A `resolve_branch` hook can change the branch name,
so read back what was created:

```bash
gw ws show fix-auth -o json
```

Done when every repo in the output has the expected `branch` and a `worktree_path`.

## 3. Work in it

```bash
cd "$(gw ws show fix-auth -o path)"        # workspace root; each repo is a subdirectory
gw status fix-auth -o jsonl                # one object per repo: branch, changed, ahead, behind
gw add-repo fix-auth -r shared-lib         # add repos later; also accepts git URLs
gw sync fix-auth                           # rebase every repo onto its base branch
```

- Inside a workspace directory, `NAME` can be omitted for `status`, `sync`, `reset`, `add-repo`, and `remove-repo`. Passing it is still safer.
- `gw sync` skips dirty repos and aborts a failed rebase. It rewrites local branch history, so warn the user first if the branch is shared or already pushed.
- `gw go NAME` also prints the path, but without a trailing newline and meant for the user's shell function. Prefer `gw ws show NAME -o path`.

## 4. Clean up

Before deleting, confirm nothing will be lost:

```bash
gw status fix-auth -o jsonl
```

Commit all changes and push code commits before deletion.
Delete checks commits against every remote-tracking branch without fetching.
It reports every problem, including temporary worktrees, before changing anything.
Context records merge locally and need no push.
Then:

```bash
gw delete fix-auth
```

For many workspaces: `gw prune --min-age 14d --json` lists old candidates. Show the list, and only after the user agrees run `gw prune --min-age 14d --yes`.

## Setup on a fresh machine

Install one way (the user may prefer one):

```bash
brew install nicksenap/grove/grove
curl -fsSL https://raw.githubusercontent.com/nicksenap/grove/master/scripts/install.sh | sh
go install github.com/nicksenap/grove/cmd/gw@latest
```

Then:

```bash
gw init ~/dev ~/work          # directories that contain git repos
gw repos -o name              # confirm discovery
gw preset add backend -r api,worker   # optional
gw doctor --json              # `null` means healthy
```

Ask the user to add shell integration to their rc file (needed only for `gw go` to change *their* shell's directory). The agent does not need it:

```bash
eval "$(gw shell-init)"       # bash/zsh; `gw shell-init --help` for others
```

## More

- Every flag, output format, and JSON field: [references/commands.md](references/commands.md)
- Lifecycle hooks (`resolve_branch`, `post_create`, `post_add_repo`, `pre_delete`, `on_close`): https://github.com/nicksenap/grove/blob/master/docs/hooks.md
- Per-repo `.grove.toml` (`base_branch`, `setup`) and the rest: `gw <command> --help`
