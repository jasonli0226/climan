# climan

A catalog of the CLI tools installed on your machine, and of how your AI agents
should use them.

> **Status:** early. All v1 commands work on Linux and Windows. See
> [docs/design.md](docs/design.md) for the full design.

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

## Install

```sh
go install github.com/jasonli0226/climan@latest
```

## Quickstart

```sh
climan sync                       # scan and pick tools to keep
climan list                       # show kept tools; ? marks weak descriptions
climan enable zoxide --desc "jump to frequent dirs"   # fix a description
climan list --pending             # tools you have not decided on yet
climan context                    # the tool list agents read
climan describe uv                # everything known about one tool
climan register alias://gs --desc "git status"   # add an alias by hand
climan skill --all --out ~/agent-skills          # one SKILL.md per enabled tool
```

## Using with agents

[`skills/climan/SKILL.md`](skills/climan/SKILL.md) teaches an agent to run
`climan context` once per session and to use an installed tool
before writing its own script. Copy it into your agent's skills directory.
`context` lists enabled tools only, one line each, so it stays short enough to
read on every task.

## Commands

| Command | What it does |
|---|---|
| `sync [--batch \| --auto]` | Scan and pick new tools. `--batch` runs without prompts (for cron); `--auto` applies `rules.yaml`. |
| `enable <tool>... [--desc "text"]` / `ignore <tool>...` | Keep tools, or stop being asked about them. `--desc` sets your own description, which agents see instead of the probed one. |
| `register <id> [--desc] [--tags a,b]` | Add an entry by hand: a script (`file://...`), alias (`alias://...`), or MCP server (`mcp://...`). |
| `list [--all \| --pending \| --ignored \| --stale]` | Show kept tools, or other groups. |
| `context` | Print the short tool list agents read on every task. |
| `describe <tool> [--probe]` | Show everything known about a tool; `--probe` reruns its `--version` and `--help`. |
| `skill <tool> \| --all [--out dir]` | Write `<dir>/<category>/<tool>/SKILL.md` for an enabled, installed tool. Never overwrites a SKILL.md you wrote yourself or one written for another tool. |

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
| `~/.config/climan/config.yaml` | Settings: `skills_dir: ~/agent-skills` sets the default `skill --out` |
