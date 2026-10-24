# climan design

## Problem

Developers install dozens of CLIs (cargo, npm, pipx, uv, go install, brew, ...)
and forget what exists. There is no single view of what is installed, what it
does, and how agents should use it.

## Stack

- Go, single static binary.
- Dependencies: `gopkg.in/yaml.v3` for the registry. Read and write it through
  the `yaml.Node` API so hand-written comments and key order survive rewrites.
- `charmbracelet/huh` for the sync TUI, only if hand-rolling selection is too
  costly.

## Core design decisions

### 1. Discovery is passive, curation is explicit

- `sync` scans PATH and known install dirs, then diffs the results against the
  registry. Newly found binaries enter as `pending`.
- Interactive sync opens a TUI listing new tools, **unchecked by default**. The
  user toggles tools on (space / `a` for all / enter to confirm). Checked tools
  become `enabled`; unchecked tools become `ignored`, so they are not offered
  again. Adopting a tool is the act of curation.
- Escape hatches:
  - `sync --batch` (alias `--no-input`): non-interactive. New tools stay
    `pending` and nothing is enabled. Safe for cron.
  - `sync --auto`: non-interactive. Applies the rules file (see
    [Auto rules](#auto-rules)); tools that match no rule stay `pending`.

### 2. Registry is plain YAML

Stored at `~/.config/climan/registry.yaml`. It is grep-able and diff-able, with
no database. Writes are atomic (write a temp file, then rename).

### 3. Sync never destroys

- Entries are never deleted by sync, so curated notes survive uninstall and
  reinstall.
- Entries created with `register` are never modified by sync.

### 4. Filtering keeps the list human-sized

Rules below were checked against a real Linux PATH (see
[Findings](#findings-from-a-real-path)).

- **PATH dirs are deduped first**, by their resolved path. On merged-`/usr`
  systems `/bin` → `/usr/bin` and `/sbin` → `/usr/sbin`, and scanning both
  doubles every core tool.
- **Tags by location** (first match wins):
  - `core`: `/usr/bin`, `/bin`, `/usr/sbin`, `/sbin`, `/usr/games`, and the
    Windows dirs in [Platforms](#platforms). Core tools are scanned but
    hidden from the sync prompt and from the default `list`. They are not
    written to the registry unless enabled by hand, which keeps the file
    human-sized; `list --all` rescans to show them.
  - `installed`: every other dir, such as anything under the user's home,
    `/usr/local/*`, `/opt/*`, `/snap/bin`, and on Windows `C:\Program Files\...`.
    Shown in the sync prompt. A fixed list of package-manager
    dirs is not enough: real PATHs contain per-tool dirs such as
    `~/.opencode/bin` and versioned ones such as
    `~/.nvm/versions/node/<v>/bin`.
  - `/mnt/<drive>/...` under WSL is skipped.
- **Shadowing, not symlink targets, decides duplicates.** When a name appears
  in several PATH dirs, the first one wins, as in the shell; the rest are kept
  as `shadowed` paths on the same entry. Collapsing by resolved symlink target
  is wrong for multi-call dispatchers: every `/snap/bin` command resolves to
  `/usr/lib/snapd/snapd`, and busybox works the same way.
- **Shims** are recorded, not resolved. A file is a shim when it is a short
  script (`#!` first line, under 4 KiB) whose text names a version manager
  (`pyenv`, `asdf`, `mise`, `rbenv`, `nodenv`, `volta`), or lives in a dir
  named `shims`; on Windows, when a sibling `.shim` file exists (scoop) or it
  is a `.cmd` in the npm dir. The entry stores the shim kind. Finding the real
  binary behind a script shim means running the version manager, which is a
  probe and follows [Probe safety](#probe-safety).
- **Family collapse** applies to `installed` tools only. A name joins family
  `<base>` when it is `<base>` followed by a version suffix (`3`, `3.12`,
  `-21`, `5.40.1`) **and** `<base>` or `<base><major>` also exists. For
  example `python`, `python3`, and `python3.12` become one entry, "python (3
  names)", and `-config` variants (`python3.12-config`) join `python-config`.
  The family's registry key is the bare base name. Curated notes live on the
  family, so a new version joins without losing them.
  Restricting collapse to `installed` matters: in system dirs the suffix rule
  wrongly groups unrelated tools (`diff`/`diff3`, `killall`/`killall5`).
- **Noise filters** (hidden from the sync prompt, kept in the registry as
  `ignored`): names starting with `_` (pyenv makes a `__pycache__` shim),
  names ending in `.sh`, `.py`, or `_completer`, and the pyenv-virtualenv
  `activate` / `deactivate` shims.

### 5. Metadata in layers

- **L0, automatic**: name, path, version probe (`--version` with a timeout),
  first line of `--help`.
- **L1, lazy**: a one-paragraph description, filled in batch by an LLM on adopt
  or from the weekly digest prompt. The LLM sits outside the core binary, so
  climan works offline.
- **L2, curated**: usage snippets, gotchas, tags.

### 6. Agent integration is the main feature

- `climan skill <tool>` writes a `SKILL.md` (synopsis, flags, examples) in the
  agent-skills layout: `<category>/<skill-name>/SKILL.md`. The category is the
  tool's first tag, or `cli` if it has none.
- `climan context` prints a compact "installed tooling inventory" block for an
  agent prompt. It includes enabled tools only: about 40 entries, not 3000.

### 7. Weekly digest (out of v1 core)

Cron runs `sync --batch` followed by a small digest script that sends a
Telegram message listing tools that became new, pending, or stale this week,
and prompts the user to describe them. The Telegram bot token comes from an
environment variable, never from the registry.

## Tool lifecycle

Each entry has two independent properties: a **status** chosen by the user,
and a **presence** observed by sync.

| Status    | Meaning                                         | Shown by        |
|-----------|-------------------------------------------------|-----------------|
| `pending` | Found by sync, not yet decided                  | `list --pending`|
| `enabled` | Adopted by the user; included in `context`      | `list` (default)|
| `ignored` | Declined by the user; not offered again         | `list --ignored`|

Transitions:

- Found by sync for the first time → `pending`.
- Checked in the sync TUI, or `climan enable <tool>` → `enabled`.
- Left unchecked in the sync TUI, or `climan ignore <tool>` → `ignored`.
- `climan enable` / `climan ignore` can move any entry between states later.

Presence:

- Every sync records `last_seen` for each binary it finds.
- A discovered entry whose binary was not found in the latest sync is
  **stale**. Its status is kept, so a reinstalled tool returns to the state it
  had before. Shown by `list --stale`.
- A `register`ed entry whose target no longer exists is **missing**. This is
  computed when listing, never written by sync, so registered entries are
  never modified.

## Tool IDs

Every entry has an ID. Discovered binaries use their command name. Manual
entries use a URI scheme:

| Kind           | ID form            | Example                |
|----------------|--------------------|------------------------|
| PATH binary    | `<name>`           | `rg`                   |
| Off-PATH script| `file://<path>`    | `file://~/bin/backup`  |
| Shell alias    | `alias://<name>`   | `alias://gs`           |
| MCP server     | `mcp://<name>`     | `mcp://github`         |

## Auto rules

`sync --auto` reads `~/.config/climan/rules.yaml`. Rules are checked in order
and the first match wins. A rule matches on install dir, a name glob, or both.

```yaml
rules:
  - dir: ~/.cargo/bin
    action: enable
  - name: "*-completion"
    action: ignore
  - dir: ~/.local/share/pnpm
    action: enable
    tags: [js]
```

Tools that match no rule stay `pending`.

## Commands (v1 scope)

- `sync [--batch | --auto]`: scan, diff, interactive adopt.
- `register <id> [--desc ...] [--tags ...]`: manually add one entry (off-PATH
  scripts, aliases, MCP servers).
- `enable <tool>` / `ignore <tool>`: change an entry's status.
- `list [--all | --pending | --ignored | --stale]`: enabled tools by default.
- `describe <tool>`: show full metadata.
- `skill <tool> --out <dir>`: write one tool's skill file under `<dir>`.
- `skill --all --out <dir>`: write skill files for every enabled tool.
  `--out` defaults to `skills_dir` in `~/.config/climan/config.yaml`.
- `context`: print the agent-prompt inventory block.

Deferred: a built-in `digest` command (cron plus a small script first),
`doctor`, export and import.

## Platforms

v1 targets Linux and Windows (amd64 and arm64). Builds use `CGO_ENABLED=0`,
so every target cross-compiles from one Linux machine with `GOOS`/`GOARCH`; no
Docker is needed. Windows behavior is tested on a GitHub Actions
`windows-latest` runner, since Linux hosts cannot run Windows containers.

| Concern          | Linux                               | Windows                                         |
|------------------|-------------------------------------|-------------------------------------------------|
| PATH split       | `filepath.SplitList` (`:`)          | `filepath.SplitList` (`;`)                      |
| Executable test  | regular file with any exec bit      | extension listed in `PATHEXT`                   |
| Config dir       | `os.UserConfigDir()` → `~/.config/climan` | `os.UserConfigDir()` → `%APPDATA%\climan` |
| `core` dirs      | `/usr/bin`, `/bin`, `/usr/sbin`, `/sbin`, `/usr/games` | `%SystemRoot%\System32`, `%SystemRoot%` |
| Interest dirs    | see [Filtering](#4-filtering-keeps-the-list-human-sized) | `%USERPROFILE%\go\bin`, `%USERPROFILE%\.cargo\bin`, `%USERPROFILE%\.local\bin`, `%APPDATA%\npm`, `%USERPROFILE%\scoop\shims`, `%LOCALAPPDATA%\Microsoft\WinGet\Links`, `%ProgramData%\chocolatey\bin` |
| Shims            | script shims (see [Filtering](#4-filtering-keeps-the-list-human-sized)) | scoop: read `path =` from the sibling `.shim` file; npm: `.cmd` wrappers |
| `alias://`       | shell aliases                       | deferred past v1 (PowerShell aliases/functions) |

On Windows, a file name and its extension form one command (`rg.exe` → `rg`).
When the same name appears with several `PATHEXT` extensions in one dir, the
earliest extension in `PATHEXT` wins, matching how the shell resolves it.

**WSL** is treated as Linux. WSL appends Windows paths (`/mnt/<drive>/...`)
to PATH by default; sync skips them, since those tools belong to the Windows
side of the machine.

## Probe safety

L0 probes (`--version`, `--help`) execute binaries climan did not write. Some
ignore unknown flags and run their normal action, some wait on stdin, and on
Windows some GUI programs open a window. To limit this:

- Never probe during the scan. Sync only reads directory listings, file
  metadata, and (for scoop) `.shim` files.
- Probe only when a tool becomes `enabled`, and only tools tagged `installed`.
  `core` tools are never probed automatically; `describe --probe <tool>`
  probes one on request.
- Run each probe with stdin closed (connected to the null device), a 2-second
  timeout that kills the process, output capped at 64 KiB, and a minimal
  environment: `PATH`, `HOME` / `USERPROFILE`, `LANG`, `SystemRoot` (Windows),
  plus `NO_COLOR=1` and `TERM=dumb`.
- Run the probe in a fresh temp directory as working dir, so a tool that
  writes files writes them there.
- On Windows, start probes with the `CREATE_NO_WINDOW` flag.
- Probe output is stored as untrusted text. It is never executed or
  interpreted, and `context` / `skill` output escapes it as plain text.

## Concurrent writes

Interactive sync, cron sync, and other climan commands can overlap.

- Every command that writes the registry takes an exclusive lock on
  `registry.yaml.lock` in the config dir, using `gofrs/flock` (flock on Linux,
  `LockFileEx` on Windows). If the lock is busy for 5 seconds, the command
  exits with an error naming the lock file.
- Writes go to a temp file in the same directory, are flushed with `fsync`,
  then replace `registry.yaml` with `os.Rename`.
- On Windows, the rename fails if another program holds the file open without
  sharing. climan retries for up to 2 seconds, then exits with an error that
  says the registry is open in another program; the temp file is kept so no
  changes are lost.
- Interactive sync reads the registry, waits for user input, then writes. To
  avoid overwriting edits made in the meantime, it records the file's
  modification time and size at read time and re-reads and re-applies its
  changes if they differ when it takes the lock to write.

## Findings from a real PATH

From `prototype/pathdump` on one Linux developer machine (2026-10-25):

| Measure                                   | Count |
|-------------------------------------------|-------|
| Raw PATH entries                          | 4895  |
| After deduping `/bin`=`/usr/bin` dirs     | ~2470 |
| `core`                                    | ~2420 |
| Non-core (`installed` under the new rules)| 60    |
| After noise filters and family collapse   | ~43 (estimated by hand, not run) |

16 of the non-core entries are snap commands, mostly GUI apps (firefox, zoom,
thunderbird). They are CLIs only in name, which is why the sync prompt
defaults to unchecked.

**Default list size:** the default `list` and `context` show enabled tools
only, so their size is what the user adopts. The sync prompt should stay
under 60 entries on a typical machine; the rules above give about 43 here.

## Open questions

- **Description source.** The first line of `--help` is often a usage line.
  Consider falling back to `whatis` or the man page.
- **LLM fill mechanism.** How L1 descriptions are requested: probably an
  external command hook configured in `config.yaml`. Not needed for v1.
- **macOS.** Not a v1 target. brew dirs stay in the interest list for Linux
  (Linuxbrew).
