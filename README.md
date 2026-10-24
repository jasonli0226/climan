# climan

A catalog of the CLI tools installed on your machine, and of how your AI agents
should use them.

> **Status:** design stage. Nothing is implemented yet. The commands below
> describe the planned v1. See [docs/design.md](docs/design.md) for the full
> design.

## Why

Over time you install dozens of CLIs through cargo, npm, pipx, uv,
`go install`, and brew, then forget what you have. climan scans your machine,
lets you choose which tools matter, and keeps notes on them in one plain YAML
file. It can also hand that list to your agents as a prompt block or as skill
files.

## How it works

1. `climan sync` scans your PATH and common install dirs. New tools show up
   unchecked in a picker; you check the ones you want to keep.
2. Kept tools are stored in `~/.config/climan/registry.yaml`, along with any
   notes, tags, and usage snippets you add. You can edit the file by hand.
3. `climan context` and `climan skill` turn the registry into something an
   agent can read.

Sync never deletes entries. If you uninstall a tool, its notes are kept and
come back when you reinstall it.

## Install (planned)

```sh
go install github.com/jasonli0226/climan@latest
```

## Quickstart (planned)

```sh
climan sync                       # scan and pick tools to keep
climan list                       # show kept tools
climan describe rg                # full metadata for one tool
climan register alias://gs --desc "git status"
climan context                    # inventory block for an agent prompt
climan skill rg --out ~/agent-skills
```

## Commands

| Command | What it does |
|---|---|
| `sync [--batch \| --auto]` | Scan and pick new tools. `--batch` runs without prompts (for cron); `--auto` applies `rules.yaml`. |
| `register <id>` | Add an entry by hand: a script (`file://...`), alias (`alias://...`), or MCP server (`mcp://...`). |
| `enable <tool>` / `ignore <tool>` | Keep a tool, or stop being asked about it. |
| `list [--all \| --pending \| --ignored \| --stale]` | Show kept tools, or other groups. |
| `describe <tool>` | Show everything known about a tool. |
| `skill <tool> \| --all` | Write `SKILL.md` files for agents. |
| `context` | Print a short tool list to paste into an agent prompt. |

## Registry example

```yaml
tools:
  rg:
    status: enabled
    path: ~/.cargo/bin/rg
    version: "14.1.0"
    summary: Recursively search directories for a regex pattern.
    tags: [search]
    notes: |
      Respects .gitignore. Use -uu to include hidden and ignored files.
```

The exact schema is not final.

## Files

| Path | Purpose |
|---|---|
| `~/.config/climan/registry.yaml` | Your tools and notes |
| `~/.config/climan/rules.yaml` | Auto-enable rules for `sync --auto` |
| `~/.config/climan/config.yaml` | Settings, such as `skills_dir` |
