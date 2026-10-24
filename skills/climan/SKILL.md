---
name: climan
description: Use at the start of every task, before writing any script or reaching for an unfamiliar command. Lists the CLI tools installed and approved on this machine so you can use an existing tool instead of reinventing one.
---

# climan: check installed tools first

The user curates the command-line tools on this machine with climan.
`climan context` prints the approved ones, one line each.

## Every task

1. Run `climan context` before you start.
2. If a listed tool fits the task, run `<tool> --help` (or `<tool> -h`) to
   learn its flags, then use it.
3. Write your own script only when no listed tool fits.
4. Do not run a tool that is not in the list unless the user asks for it;
   it may not be installed.

Names in parentheses are other commands of the same tool, for example
`python (also python3, python3.12)`. A line with no description is still a
usable tool; check its `--help`.

## For the user

If a description is wrong or missing, set your own:

```sh
climan enable <tool> --desc "one line saying what it does"
```

`climan list` shows enabled tools; a `?` marks descriptions worth fixing.
