# `gw` command reference

Global flags: `--verbose` (debug log to `~/.grove/grove.log`), `-n`/`--no-hooks` (skip optional lifecycle hooks; delete checks and records merging still run), `-v`/`--version`.

**Interactive?** column: what happens if you omit the listed args. "picker" means exit 1 without a TTY; "prompt" means it prints a question and, with no stdin, silently declines and exits 0.

## Workspaces

| Command | Non-interactive form | Interactive if omitted | Destroys work? |
|---|---|---|---|
| `gw create` | `gw create NAME [--type work\|priv] [-b BRANCH] (-r a,b \| -p PRESET \| --all)` | `-r/-p/--all` → picker; without NAME, supply `-b` or answer the branch prompt | No |
| `gw create --replace` | `gw create NEW -b B -r a --replace -f` (run inside the workspace to replace) | prompt without `-f` | **Yes**: deletes the current workspace first, even if the create then fails |
| `gw ws show` | `gw ws show [NAME] -o json\|path` | — (outside a workspace without NAME → error) | No |
| `gw list` / `gw ws list` | `gw list [NAME] -o jsonl [-s]` (`-s` adds git status; `-o wide` adds repositories, sessions and goals; NAME lists one workspace with its sessions) | — | No |
| `gw status` | `gw status [NAME] -o jsonl [-P]` (`-P` adds PR status via gh/glab) | — | No |
| `gw go` | `gw go NAME` (prints path, no newline) | >1 workspace → picker | `-d` deletes the workspace after navigating away |
| `gw add-repo` | `gw add-repo [NAME] -r a,b` (names or git URLs; URLs clone into the first repo dir) | `-r` → picker | No |
| `gw remove-repo` | `gw remove-repo [NAME] -r a -f`, or `echo y \| gw remove-repo [NAME] -r a` to keep the dirty-worktree check | `-r` → picker; always prompts without `-f` | **Yes**: removes the worktree and its branch; `-f` also skips the dirty check |
| `gw rename` | `gw rename OLD NEW`, or `gw rename OLD --to NEW` | `OLD` → picker | No |
| `gw reopen` | `gw reopen NAME` | None | Recreates only the context worktree; records and the original branch name survive |
| `gw sign` | `gw sign [NAME]` | None | Autosquashes fixups, signs code commits and pushes branches with force-with-lease |
| `gw sync` | `gw sync [NAME]` | — | Rebases local branches onto base (history rewrite); skips dirty repos |
| `gw reset` | `gw reset [NAME] [--discard]` | — | `--discard` drops tracked local changes |
| `gw delete` / `gw ws delete` | `gw delete NAME... [--force]`; or `... \| gw delete --stdin --yes` | no names → picker | Checks all worktrees and merges committed context records before removal. `--force` skips checks and discards uncommitted work and unpushed code. Pre-existing branches are kept. |
| `gw prune` | `gw prune [--min-age 7d] --json` to list; add `--yes` to delete | — | `--yes`: deletes every listed workspace like `gw delete` |
| `gw prune branches` | `gw prune branches [--fetch] [--gone] --json`; `--yes` deletes | — | `--yes` deletes merged branches in source clones; `--gone` force-deletes, may drop unmerged commits |

`NAME` in brackets is detected from the current directory when you are inside a workspace.

New workspaces default to type `work` and live at `<workspace_dir>/<type>/<name>/`.
Names are unique across types.
Without `-b`, creation requires `owner_prefix` in `~/.grove/config.toml`
and derives the branch as `<owner_prefix>/<name>`.
With `--no-hooks`, the default branch is the workspace name alone,
and `owner_prefix` is not required.
`--track` requires an explicit `-b`.
Create and add-repo reuse local branches first, then adopt same-named origin branches,
or create branches from base without an upstream.
`--track` skips branch resolvers and warns when the remote branch is missing.
Adoption works with `--no-hooks`; SSH push-URL configuration requires hooks.
HTTPS credentials come from Git's configuration.
The `[types.work]` and `[types.priv]` tables each support a `context` repository path.
Create adds the configured context worktree and commits the workspace's records.

With context repositories configured, the default list includes active and deleted entries,
active first, then the default type `work`, then by name,
with name, type, status, the number of repositories besides the context repository and branch,
and reads neither goals nor Claude transcripts.
`-o wide` adds repository names, the number of Claude sessions and the first line of each goal.
`gw list NAME` prints one entry, active or deleted, with every column,
followed by its sessions and their resume commands.
JSON entries add `status`, `goal`, `repositories` and `sessions`.
Sessions include `id`, `first_timestamp`, `last_timestamp`, `ai_title`, `cwd`
and `resume_command`.
The `-s` list retains Grove's active Git summaries.
Rename commits the records move without changing branches.
Reopen removes deleted metadata and regenerates agent rules unless hooks are disabled.
Required create-name checks, rename records and reopen metadata run with `--no-hooks`.

## Setup and config

| Command | Form | Notes |
|---|---|---|
| `gw init` | `gw init DIR...` | Dirs must exist. Required before anything else. Idempotent. |
| `gw add-dir` / `gw remove-dir` | `gw add-dir PATH`, `gw remove-dir PATH` | `remove-dir` without PATH → picker |
| `gw repos` | `gw repos -o name\|json` | Discovered repos and remotes |
| `gw preset add` | `gw preset add NAME -r a,b` | No `-r` → picker |
| `gw preset list` / `show` / `remove` | `gw preset list --json`, `gw preset show NAME --json`, `gw preset remove NAME` | `remove` without NAME → picker |
| `gw doctor` | `gw doctor --json`; `--fix` repairs | `--json` prints `null` (not `[]`) when healthy. `--fix` removes stale state and trash; for a workspace whose directory is gone it also prunes worktree registrations and force-deletes the workspace branch, like `gw delete`. |
| `gw shell-init` | `gw shell-init [--shell nu]` | For the user's rc file only; agents never need it |
| `gw plugin` | `install OWNER/REPO`, `list --json`, `search [term] --json`, `upgrade [name]`, `remove NAME` | No prompts |
| `gw stats` | `gw stats` | Human-only output |
| `gw bug-report` | `gw bug-report --print` | Without a TTY it prints instead of opening a browser |

## Output formats

`-o`/`--output` on `list`, `ws list`, `ws show`, `status`, `repos`: `table` (default, do not parse), `json`, `jsonl`, `tsv`, `name`, `path`. `list` and `ws list` also take `wide`, a table with more columns. `-j`/`--json` is an alias for `-o json`.

`--json` only (no `-o`): `doctor`, `prune`, `prune branches`, `preset list`, `preset show`, `plugin list`, `plugin search`.

Human messages (`ok:`, `warn:`, `error:`, progress) go to stderr. `gw create` writes nothing to stdout; read the result with `gw ws show NAME -o json`.

## JSON fields

Workspace (`ws show`, `list`):

```json
{"name": "fix-auth", "type": "work", "path": "...", "branch": "fix/auth-expiry", "created_at": "...",
 "repos": [{"repo_name": "api", "source_repo": "...", "worktree_path": "...", "branch": "fix/auth-expiry"}]}
```

Status (`status -o jsonl`, one line per repo):

```json
{"workspace": "fix-auth", "repo": "api", "path": "...", "branch": "fix/auth-expiry",
 "status": "clean", "changed": 0, "ahead": "-", "behind": "-"}
```

`changed` is a number. `ahead` and `behind` are **strings**; `"-"` means no upstream (the branch was never pushed). `pr` appears with `-P`.
