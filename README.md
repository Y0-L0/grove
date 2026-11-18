<p align="center">
  <img src="assets/logo-light.png" alt="Grove logo" width="120">
</p>

<h1 align="center">Grove (<code>gw</code>)</h1>

<p align="center"><b>grove</b> /ɡrōv/ <i>noun</i> — a small group of trees growing together.</p>

<p align="center">
  <a href="https://github.com/nicksenap/grove/actions/workflows/ci.yml"><img src="https://github.com/nicksenap/grove/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/nicksenap/grove/releases/latest"><img src="https://img.shields.io/github/v/release/nicksenap/grove" alt="Release"></a>
  <a href="https://github.com/nicksenap/grove/blob/master/LICENSE"><img src="https://img.shields.io/github/license/nicksenap/grove" alt="License"></a>
</p>

<p align="center">
  <a href="assets/grove-brag.mp4"><img src="assets/grove-brag.gif" alt="Grove turns one feature across three repositories into one worktree workspace" width="900"></a>
</p>

## Why?

Monorepos solve cross-project work, but not everyone has one. You've got separate repos, separate CI, separate deploys — and that's fine until you need to work across them.

One feature across three services means `git worktree add` three times, tracking three branches, jumping between three directories, cleaning up three worktrees when you're done. It's annoying.

Grove gives you the multi-repo worktree workflow that monorepos get for free. One command creates a workspace with every repo on the same branch, and `gw status` shows all of them in one table.

## Getting Started

Grove is a single static binary with no dependencies. It requires `git` on `PATH`.

### 1. Install Grove

Pick **any one** of the following methods:

**Homebrew** (macOS / Linux)

```bash
brew install nicksenap/grove/grove
```

**Install script** (macOS / Linux, no Homebrew needed)

```bash
curl -fsSL https://raw.githubusercontent.com/nicksenap/grove/master/scripts/install.sh | sh
```

Verifies the release checksum and installs to `/usr/local/bin` (or `~/.local/bin` if that is not writable). Override with `GW_INSTALL_DIR` and `GW_VERSION`.

**Go install** (if you already have a Go toolchain)

```bash
go install github.com/nicksenap/grove/cmd/gw@latest
```

### 2. Add shell integration

This enables `gw go` to change your working directory and auto-cds into new workspaces after `gw create`.

**Bash / Zsh** — add to `.zshrc` or `.bashrc`:

```bash
eval "$(gw shell-init)"
```

**Nushell** — generate and source the init file:

```nu
gw shell-init --shell nu | save -f ~/.config/nushell/grove.nu
# then add to config.nu:
source grove.nu
```

### 3. Create your first workspace

```bash
gw init ~/dev                          # register the directory that holds your repos
gw create -b feat/login -r svc-a,svc-b # create the feat-login workspace across both repos
gw status feat-login                   # see every repo's branch and git status in one table
```

### Let your coding agent drive it

Grove ships an [agent skill](skills/grove/SKILL.md) that teaches Claude Code, Codex, Cursor, pi, and other skill-aware agents to set up Grove and run `gw` safely: explicit flags instead of pickers, JSON output, and which commands destroy work.

```bash
npx skills add nicksenap/grove --skill grove
```

No skills installer? Paste this prompt into your agent instead:

```text
Read https://raw.githubusercontent.com/nicksenap/grove/master/skills/grove/SKILL.md and set up Grove for me.
```

## Usage

```bash
# Register your repo directories (one-time)
gw init ~/dev ~/work/microservices

# Create a workspace: name the branch, pull repos from a preset or a list
gw create -b feat/login -p backend        # repos from a saved preset
gw create -b feat/login -r svc-a,svc-b    # …or an ad-hoc repo list

# Work across the whole workspace
gw go my-feature       # cd into the workspace
gw status my-feature   # git status across every repo
gw sync my-feature     # rebase all repos onto their base branch
gw reset my-feature    # switch every repo back to the workspace branch, then sync

# Clean up after committing and pushing code changes
gw delete my-feature   # removes worktrees, branches, and workspace files
gw prune               # list workspaces older than 7 days, plus stale records whose directory is gone
gw prune --yes         # delete them (same two-phase cleanup as gw delete)
gw prune branches      # list local branches in main clones that are merged and unused
gw prune branches --gone --fetch --yes   # also branches whose upstream was deleted; delete
```

### Pipelines and machine-readable output

List-style commands support `--output` (`-o`) with `table`, `json`, `jsonl`, `tsv`, `name`, and `path` formats. `table` remains the default. Existing `--json` / `-j` flags remain supported as compatibility aliases for `--output json`.

```bash
# Select a workspace without parsing a table
gw list -o name | fzf

# Feed workspace paths into another command
gw list -o path | while IFS= read -r path; do
  printf '%s\n' "$path"
done

# Process one complete object per line
gw status my-feature -o jsonl |
  jq -r 'select(.changed > 0) | .repo'

# Delete a reviewed set of workspaces from newline-delimited stdin
gw list -o name | grep '^old-' | gw delete --stdin --yes
```

The output formats are available on `gw list`, `gw repos`, `gw status`, and `gw ws show`. Data is written to stdout; progress and diagnostics are written to stderr. ANSI styling is disabled when output is redirected or `NO_COLOR` is set.

Interactive menus support search (press `/`, then type), arrow-key or `j`/`k` navigation (single-select), or arrow + tab (multi-select) with an `(all)` shortcut.

Repos can carry a `.grove.toml` so every new worktree is ready to work in:

```toml
# svc-a/.grove.toml
base_branch = "stage"   # branch from origin/stage instead of origin/main
setup = "pnpm install"  # run after the worktree is created
```

This fork creates workspaces at `<workspace_dir>/<type>/<name>/`.
The type defaults to `work`; use `--type priv` for a private workspace.
Names are unique across types, and existing workspaces keep their paths.
Set `owner_prefix` in `~/.grove/config.toml` to derive the branch from the name:

```toml
owner_prefix = "alice"

[types.work]
context = "/home/alice/contexts/work"

[types.priv]
context = "/home/alice/contexts/private"
```

`gw create login -r api` creates a `work` workspace on branch `alice/login`.
An explicit `--branch` overrides the owner-prefix default.
Without `owner_prefix`, supply `--branch`; `--track` always requires it.
`--no-hooks` also skips owner-prefix derivation.
Without `--branch`, it uses the workspace name as the branch name.
Shell resolvers still run after Go resolvers and can change the resulting branch.
See [docs/hooks.md](docs/hooks.md#resolving-the-branch-name).

Create and add-repo reuse an existing local branch first.
Otherwise, they adopt `origin/<branch>` with its upstream when it exists,
or create a branch from base without an upstream.
`--track` skips branch resolvers and warns when the remote branch is missing.
Adoption also works with `--no-hooks`.

For HTTPS origins, the Go lifecycle hooks set an SSH `remote.origin.pushurl`
when none exists, preserving the fetch URL and existing push URLs.
Context repositories are excluded, and `--no-hooks` skips this configuration.
Configure HTTPS credential helpers through Git, including inside the sandbox.
Grove preserves credential configuration.

When the type has a configured context repository,
create adds its worktree on the workspace branch, named after its source clone.
The context path supports `~/` and may sit outside `repo_dirs`.
The workspace's relative `records` symlink points to `<clone>/records/<name>/`.
Create commits `TASK.md` and `workspace.toml` there, unsigned.
The goal belongs in `TASK.md`;
active repositories and the context flag stay in Grove's state.
`--no-hooks` skips context worktree creation and records as well.

Context repositories may have no remote.
Their status and sync use the local base branch,
with `base_branch` in the source clone's `.grove.toml` overriding the default.
For a local repository without `main` or `master`, configure that base explicitly.
`remove-repo` refuses the context worktree, including with `--force`.

To generate workspace agent rules, configure the template at the top level of
`~/.grove/config.toml`, using an absolute path or a path beginning with `~/`:

```toml
agent_rules_template = "~/src/grove/templates/AGENTS.md"
```

Create, add-repo and sync replace the workspace's `AGENTS.md` with this template
followed by `agent-rules.md` from its context worktree, when that file exists.
The generated disclaimer names both source paths.
`CLAUDE.md` contains `@AGENTS.md`.
Before generation, the context worktree rebases onto its local base branch,
normally `main`, including any workspace commits.
Dirty or unexpected context branches and rebase failures stop generation
and preserve the existing agent files.
Context rebases keep commits unsigned, including during sync.
Edit the sources, since regeneration replaces local edits to the generated files.
An unset `agent_rules_template` disables generation.
`--no-hooks` skips generation and the additional context rebase;
sync still performs its normal repository rebases.

Delete reports dirty or unregistered worktrees, unexpected branches,
unpushed code commits and temporary worktrees before changing anything.
Plain directories such as `vmtest/` go with the workspace.
The context clone must have its local base branch checked out.
Delete commits the deletion date and code repository names in `workspace.toml`,
rebases the context branch onto its base and fast-forwards the clone before removal.
These commits remain unsigned.
`--force` skips checks and discards uncommitted work, but still merges committed records.
`--no-hooks` skips optional lifecycle hooks; delete checks and records merging still run.
Delete repeats validation after optional hooks and repository teardown commands.
If a hook commits more context records, deletion stops until those commits reach the base branch,
including with `--force`.
Disable the mutating hook before retrying deletion.
If it changed the context checkout's branch, merge that checkout's commits into the workspace branch first.
Records-aware rename, the index command and reopen wait for slice 9.
Do not rename context-bearing workspaces until records-aware rename is implemented.
`doctor --fix` retains missing context entries for path repair.
A failed `post_create` leaves the created workspace in state;
an attached context worktree remains tracked when records initialization or its commit fails.

To store `state.json` and `state.lock` separately from the configuration,
set `state_dir` at the top level of `~/.grove/config.toml`.
It defaults to `~/.grove` and supports paths beginning with `~/`.

```toml
state_dir = "~/grove-state"
```

Presets, plugins, hooks, and the full command reference are covered in [Workflows](openwiki/workflows.md) and [Operations](openwiki/operations.md).

### Integrations

- [grove-herdr](https://github.com/nicksenap/grove-herdr) — [Herdr](https://herdr.dev) plugin: `prefix+shift+g` prompts for a name, runs `gw create`, and opens the workspace in Herdr; hooks close it on `gw delete`.
- [grove-rex](https://github.com/nicksenap/grove-rex) — [Rex](https://www.superlogical.com/rex) popup: `cmd+g` asks for a name, a preset or repos, and a branch, runs `gw create`, and opens the workspace as a Rex session; a `pre_delete` hook closes it on `gw delete`.
- Plugins such as [gw-dispatch](https://github.com/nicksenap/gw-dispatch), [gw-run](https://github.com/nicksenap/gw-run), and [gw-recipe](https://github.com/nicksenap/gw-recipe) — see [docs/plugins.md](docs/plugins.md).

## Upgrading

If you installed with Homebrew:

```bash
brew update && brew upgrade grove
```

If you used the install script, re-run it to get the latest release.

## Documentation

- [Quickstart](openwiki/quickstart.md) — install, first commands, and key concepts
- [Workflows](openwiki/workflows.md) — creating, syncing, resetting, and deleting workspaces
- [Operations](openwiki/operations.md) — configuration, hooks, state, and troubleshooting
- [Hooks](docs/hooks.md) — global hooks and per-repo `.grove.toml` hooks
- [Plugins](docs/plugins.md) — extend `gw` with external commands
- [AI coding tools](docs/ai-tools.md) — vendor-neutral agent workflows
- [Agent skill](skills/grove/SKILL.md) — teach a coding agent to use `gw`

Contributors can start with the [Architecture](openwiki/architecture.md) and [Integrations](openwiki/integrations.md) notes.
