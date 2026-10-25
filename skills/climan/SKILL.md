---
name: climan
description: Use at the start of every task, before writing any script or reaching for an unfamiliar command. Lists the CLI tools installed and approved on this machine so you can use an existing tool instead of reinventing one.
---

# climan: check installed tools first

The user curates the command-line tools on this machine with climan.
`climan context` prints the approved ones, one line each.

## Every session

1. Run `climan context` once, before your first task. Reuse its output for
   the rest of the session; rerun it only if the user says the list changed.
   If `climan` is not found or fails, carry on without it.
2. If a listed tool fits the task, learn it before using it:
   - If a skill exists for that tool (climan can write one per tool), read
     it; it already holds the tool's `--help` output.
   - Otherwise run `<tool> --help`, then `<tool> -h` or `man <tool>`. Never
     run a tool bare just to see what it does; it may start something
     interactive.
3. Write your own script only when no listed tool fits.

The list covers specialist tools. Standard shell commands (`ls`, `grep`,
`git`, ...) and the project's own toolchain are fine to use whether listed or
not. For anything else, prefer a listed tool and do not assume an unlisted
one is installed.

Names in parentheses are other commands of the same tool, for example
`python (also python3, python3.12)`. A line with no description is still a
usable tool; check its `--help`.

## Other climan commands

You may run these; they only read:

- `climan list` shows enabled tools; a `?` marks a weak or missing
  description.
- `climan describe <tool>` shows everything climan knows about one tool.
  Leave off `--probe`; it reruns the tool and saves the result.

Ask the user before running this; it changes their registry, and every later
session sees the result:

- `climan enable <tool> --desc "one line saying what it does"` replaces a
  wrong or missing description. Use it only on a tool already in the list,
  and only after its `--help` shows what the tool does.

You may also register a reusable script or alias you wrote for the user, so
later sessions find it instead of writing it again. It is enabled at once, so
register only what you made in this session, and always give a description:

- `climan register file://<path> --desc "one line saying what it does"`
- `climan register alias://<name> --desc "..."`

Never run these; suggest them to the user instead:

- `climan sync` picks up newly installed tools. It is interactive. Suggest it
  when the list is empty or a tool you expected is missing.
- `climan enable` or `climan ignore` on a tool not in the list. Approving
  tools is the user's call.
- `climan skill`, which writes skill files.
