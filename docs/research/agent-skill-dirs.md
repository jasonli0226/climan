# Agent skill directories: Claude Code, pi, opencode

Research for `climan install`. Researched 2026-10-27 against primary sources only:
official docs, shipped source code, and the Agent Skills spec. Anything marked
**unconfirmed** could not be pinned to a primary source.

Versions checked:

- Claude Code 2.1.285 (closed source). Sources: docs at code.claude.com, plus strings in the shipped binary.
- pi 1.3.0. The npm package is now `@earendil-works/pi-coding-agent` (repo `github.com/earendil-works/pi`, formerly badlogic/pi-mono). Sources: the installed package's `dist/` and `docs/` under `~/.pi/agent/install/releases/1.3.0/node_modules/@earendil-works/pi-coding-agent/`, written `PI/` below.
- opencode 1.19.0. Repo `github.com/anomalyco/opencode` (`sst/opencode` redirects there), tag `v1.19.0`, commit `733b2c627e5aec39cb33cc22373bdce33053ef2c`, written `OC@733b2c6` below.

## Summary

| Agent | Global dir | Project dir | Nesting | Name rules | Env override |
|---|---|---|---|---|---|
| Claude Code | `~/.claude/skills/<name>/SKILL.md` | `.claude/skills/<name>/SKILL.md` in cwd and every parent up to the repo root, plus nested `<subdir>/.claude/skills` loaded lazily | Flat: direct children only | `name` is optional (falls back to the dir name) and may differ from the dir. Only some characters are banned. No documented length limit. | `CLAUDE_CONFIG_DIR` (replaces `~/.claude`) |
| pi | `~/.pi/agent/skills/` and `~/.agents/skills/` | `.pi/skills/` in cwd only, and `.agents/skills/` in cwd and every ancestor up to the git root. Both need project trust. | Recursive, no depth limit. Stops at the first dir containing `SKILL.md`. | Follows the spec (64 chars, `[a-z0-9-]`), but violations only **warn**. Name does **not** need to match the dir. A missing description means the skill is skipped. | `PI_CODING_AGENT_DIR` (replaces `~/.pi/agent`) |
| opencode | `~/.config/opencode/skill(s)/`, `~/.opencode/skill(s)/`, `~/.claude/skills/`, `~/.agents/skills/` | `.opencode/skill(s)/`, `.claude/skills/`, `.agents/skills/` from cwd up to the git worktree | Recursive `**/SKILL.md`, unlimited depth | Docs say spec plus "must match directory". The v1.19.0 code checks **neither**. It only requires `name` to be a string. | `XDG_CONFIG_HOME` moves the global dir. `OPENCODE_CONFIG_DIR` **adds** a directory. |

## Claude Code

### Directories
- Personal skills live at `~/.claude/skills/<skill-name>/SKILL.md`. Project skills live at `.claude/skills/<skill-name>/SKILL.md`. Enterprise skills live at `.claude/skills/` inside the managed settings dir, for example `/etc/claude-code/.claude/skills/` on Linux. Plugin skills live at `<plugin>/skills/<skill-name>/SKILL.md`, and `--add-dir` directories load their `.claude/skills/`. Source: https://code.claude.com/docs/en/skills ("Choose where skills load").
- Project discovery walks from cwd "in every parent directory up to the repository root". `.claude/skills` folders *below* cwd load only "the first time Claude reads or edits a file in that subdirectory". Source: same page, "Load skills in monorepos and subdirectories".
- Windows: "`~/.claude` means `%USERPROFILE%\.claude`". Source: https://code.claude.com/docs/en/settings. macOS is the same as Linux.
- Reserved folder names:
  - `~/.claude/skills/synced/` is used for claude.ai-synced skills. A user skill named `synced` (any case) is skipped.
  - Folders named `anthropic-skills` or `anthropic-skills:*` don't load.
  - Source: skills docs.

### Env and config
- `CLAUDE_CONFIG_DIR` overrides `~/.claude`: "All settings, session history, and plugins are stored under this path". It is ignored in project and local settings. Source: https://code.claude.com/docs/en/env-vars.
- That personal *skills* move with it is implied (they live under the config dir) but not stated explicitly. The binary reads personal skills through a `userConfigDir("skills", ...)` helper, which supports this. Treat as **likely, not doc-confirmed**.

### Layout and nesting
- Every documented example is `<skills-dir>/<skill-name>/SKILL.md`. Category subfolders are not documented for skills. They are documented only for the legacy `.claude/commands/<dir>/<file>.md` form, which yields `/dir:file`.
- Binary inspection (v2.1.285) supports flat-only. The loader lists the skills dir, and for each entry `F` reads `join(dir, F, "SKILL.md")`. If that file is not found it returns `null`, with no recursion. Treat `<category>/<name>/SKILL.md` as **not discovered**. This is unconfirmed by the docs, but the binary evidence points that way.
- Claude Code does not read `.agents/skills` or other agents' folders (none are documented).

### Frontmatter
- All fields are optional. `name` "Defaults to the directory name". `description` is "Recommended". If it is omitted, Claude Code uses the first non-empty line of the body. Source: skills docs, frontmatter table.
- `name` may differ from the folder. The `/` menu shows `name`, and "The directory name also invokes the skill".
- Banned characters in names: control or zero-width characters, private-use characters, bidi characters, irregular blanks, and `<` `>`. A folder whose name has one of these "doesn't load". No length limit is documented.
- The combined `description` + `when_to_use` is truncated at 1,536 chars in the listing. The cap is configurable via `skillListingMaxDescChars`.
- A skill that uses only the six spec fields also loads on claude.ai and in the API. Extra keys can error there, but not in Claude Code. Source: skills docs, "Restricting frontmatter to the spec's six fields".

### Symlinks
- "a `<skill-name>` entry in the enterprise, personal, or project location can be a symlink to a directory elsewhere on disk ... loads the skill once even if several locations point at the same target." Source: skills docs.

### Context loading
- The prompt always lists every skill name. Descriptions share a budget of 1% of the context window (fallback 8,000 chars). When the listing overflows, descriptions are dropped starting with the least-invoked skills.
- The budget can be changed with `skillListingBudgetFraction` or `SLASH_COMMAND_TOOL_CHAR_BUDGET`.
- The full body loads only on invocation. After compaction, Claude Code keeps 5k tokens per skill and 25k in total.
- Live reload watches `SKILL.md` in the personal, project and `--add-dir` locations.
- Sources: skills docs, plus the `SLASH_COMMAND_TOOL_CHAR_BUDGET` row on the env-vars page.

### Collisions
- Enterprise beats personal, and personal beats project. A user skill replaces a bundled or built-in command of the same name.
- A root skill and a nested skill with the same name both load, and the nested one becomes `/apps/web:deploy`.
- Source: skills docs, collision table.

## pi (`@earendil-works/pi-coding-agent` 1.3.0)

### Directories
- User skills:
  - `<agentDir>/skills`, where `agentDir` is `~/.pi/agent` (`getAgentDir()` = `join(homedir(), ".pi", "agent")`). Source: `PI/dist/config.js:491-497`.
  - `~/.agents/skills`. Source: `PI/dist/core/package-manager.js:2023,2056-2064`.
- Project skills:
  - `<cwd>/.pi/skills`. It is not walked up, since `projectBaseDir = join(cwd, ".pi")`. Source: `package-manager.js:720,2039`.
  - `.agents/skills` in cwd and every ancestor, stopping at the dir containing `.git`. Source: `collectAncestorAgentsSkillDirs`, `package-manager.js:283-304`.
  - Both project sources load only when the project is trusted (`isProjectTrusted()`). Source: `package-manager.js:2024-2048`.
- Extra paths: the `skills: string[]` setting, plus Pi packages. Source: `PI/docs/settings.md:156`, `PI/docs/packages.md`.
- Windows and macOS use the same paths under `os.homedir()`, so on Windows that is `%USERPROFILE%\.pi\agent\skills`. `~/.agents` uses `process.env.HOME || homedir()` (`package-manager.js:82-84`).

### Env and config
- `PI_CODING_AGENT_DIR` replaces `~/.pi/agent` (tilde is expanded). Sources: `config.js:476,491-495` and `PI/docs/environment-variables.md:81`.
- The `.pi` name comes from `package.json` `piConfig.configDir` (`config.js:473`). Rebranded builds may use a different dir name and `<APP>_CODING_AGENT_DIR`.

### Layout and nesting
- Discovery is recursive with no depth limit. A dir containing `SKILL.md` "is a skill root and do[es] not recurse further". Sources: `PI/dist/core/skills.js` (`loadSkillsFromDirInternal`) and `package-manager.js:205-262` (`collectSkillEntries`).
- So `<dir>/<category>/<name>/SKILL.md` **works**.
- Skipped during the walk: dot-entries, `node_modules`, and paths matched by `.gitignore`, `.ignore` or `.fdignore`.
- Loose `.md` files are also treated as skills:
  - in `~/.pi/agent/skills` and `.pi/skills`, only those at the root of the dir;
  - in `.agents/skills`, only those *below* the root (`mode === "agents" && dir !== root`). Source: `package-manager.js:254-258`.
  - The docs prefer the directory form (`PI/docs/skills.md`).
- pi reads `.agents/skills` (Agent Skills locations). It does **not** read `.claude/skills`.

### Frontmatter
- pi "implements the Agent Skills specification". Most invalid fields produce **warnings** rather than errors. Source: `PI/docs/skills.md`.
- Name rules: at most 64 chars, `^[a-z0-9-]+$`, no leading or trailing hyphen, and no `--`. Violations only push a warning, and the skill still loads. Source: `skills.js` `validateName`, and `loadSkillFromFile` ("Still load the skill even with warnings").
- `name` falls back to the parent dir name. "Pi neither requires nor warns when the declared name differs from the parent directory." Source: `PI/docs/skills.md`.
- `description` is required and limited to 1024 chars. If it is missing or empty the skill is **not loaded**. If it is too long, pi warns and still loads it.
- Malformed YAML gives a warning and the skill is skipped.
- `disable-model-invocation: true` hides the skill from the prompt, but `/skill:name` still works.

### Symlinks
- Symlinked dirs and `SKILL.md` files are followed (`statSync`), and broken links are skipped. Duplicates reached via symlinks are deduped by real path (`canonicalizePath`). Source: `skills.js` `loadSkillsFromDirInternal` / `loadSkills`.

### Context loading and collisions
- Each skill's name, description and location go into the system prompt as XML `<available_skills>`. The body is read by the model on demand. Source: `skills.js` `formatSkillsForPrompt`.
- No count or size limit was found in the code.
- On a name collision, the first discovered skill wins and pi emits a `collision` diagnostic. Source: `skills.js` `loadSkills`, `PI/docs/skills.md`.
- In the `loadSkills` defaults, the user dir is added before the project dir. Order in `package-manager.js`: project `.pi`, then project `.agents`, then user `.pi`, then user `.agents`. Which list feeds the final merge was not traced, so **the effective precedence between user and project is unconfirmed**.

## opencode (v1.19.0)

### Directories
Docs (https://opencode.ai/docs/skills) list these locations:

- `.opencode/skills/<name>/SKILL.md`
- `~/.config/opencode/skills/<name>/SKILL.md`
- `.claude/skills/<name>/SKILL.md` and `~/.claude/skills/<name>/SKILL.md`
- `.agents/skills/<name>/SKILL.md` and `~/.agents/skills/<name>/SKILL.md`

Project paths are walked "up from your current working directory until it reaches the git worktree".

The code adds detail:
- Config dirs come from `ConfigPaths.directories`: `Global.Path.config`, then `.opencode` from cwd up to the worktree, then `~/.opencode`, then `OPENCODE_CONFIG_DIR` if set. Source: `OC@733b2c6 packages/opencode/src/config/paths.ts:23-41`.
- Each config dir is scanned with `{skill,skills}/**/SKILL.md`, so singular `skill/` also works. Source: `packages/opencode/src/skill/index.ts:24,205-208`.
- `~/.claude` and `~/.agents`, plus their project-level copies walked up to the worktree, are scanned with `skills/**/SKILL.md`. Source: `skill/index.ts:21-23,185-203`.
- `skills.paths` (`**/SKILL.md`) and `skills.urls` in `opencode.json` add more locations. Source: `skill/index.ts:210-227`.
- `Global.Path.config` = `join(xdgConfig, "opencode")`. Source: `packages/core/src/global.ts:13`. The xdg-basedir library resolves `xdgConfig` to `XDG_CONFIG_HOME || ~/.config` on **every OS** (https://unpkg.com/xdg-basedir@5.1.0/index.js). So the Windows path is `%USERPROFILE%\.config\opencode\skills` and macOS uses `~/.config/opencode/skills`, not `~/Library`.

### Env and config
- `XDG_CONFIG_HOME` moves the global config dir.
- `OPENCODE_CONFIG_DIR` is **appended** as an extra config dir and does not replace `~/.config/opencode` (`paths.ts:26,39`). The docs call it a "custom config directory" (https://opencode.ai/docs/config).
- `OPENCODE_DISABLE_EXTERNAL_SKILLS` skips the `.claude` and `.agents` scans.
- `OPENCODE_DISABLE_CLAUDE_CODE_SKILLS` (or `OPENCODE_DISABLE_CLAUDE_CODE`) skips `.claude` only. Source: `packages/opencode/src/effect/runtime-flags.ts:21-29`.
- `OPENCODE_DISABLE_PROJECT_CONFIG` skips project `.opencode` dirs (`paths.ts:27`).

### Layout and nesting
- The globs use `**`, so nested `<category>/<name>/SKILL.md` **is discovered at any depth**.
- Unlike pi, opencode keeps descending after it finds a `SKILL.md`, so a `SKILL.md` inside another skill's subfolder becomes a separate skill.
- `.claude` and `.agents` scans use `dot: true`.
- Consequence on this machine: opencode will pick up Claude's `~/.claude/skills/synced/<uuid>/*/SKILL.md` too.

### Frontmatter
- Docs: `name` and `description` are both required. `name` must be 1-64 chars, match `^[a-z0-9]+(-[a-z0-9]+)*$` and "Match the directory name". `description` must be 1-1024 chars. Unknown fields are ignored.
- **The code does not enforce this.** `isSkillFrontmatter` only checks that `name` is a string and that `description` is a string or undefined (`skill/index.ts:53-59`).
- `NameMismatchError` is declared (`index.ts:67`) but never used anywhere in the repo.
- If `name` is missing, the skill is skipped silently.
- Unparseable YAML is logged and also published as a session error event (`index.ts:105-121`).
- A skill with no description still loads, but it is left out of the prompt listing (`fmt`, `index.ts:321-323`).

### Symlinks
- Followed. The scan uses `Glob.scan(..., { symlink: true })`, which maps to `follow: true` (`skill/index.ts:150-156`, `packages/core/src/util/glob.ts:18`).
- The file list shown when the skill tool loads a skill uses `follow: false` (`tool/skill.ts:36-43`).

### Context loading and collisions
- The system prompt gets a verbose `<available_skills>` listing (name, description, location) for every skill the agent is not `deny`-ed. Source: `session/system.ts:107-118`.
- The body loads when the model calls the `skill` tool, together with up to 10 sampled file paths (`tool/skill.ts`).
- No count or size limit was found.
- On a duplicate name, opencode logs `"duplicate skill name"` and the **later write overwrites** the earlier one (`index.ts:125-139`). Files are parsed with `concurrency: "unbounded"`, so which file wins is **not deterministic**. Inferred from the code, not tested.
- The built-in `customize-opencode` skill can be overridden by a skill of the same name on disk.

## This machine (read-only check)

- Installed binaries: `claude` → `~/.local/bin/claude` (2.1.285), `pi` → `~/.local/bin/pi` (1.3.0, managed install under `~/.pi/agent/install`), `opencode` → `~/.opencode/bin/opencode` (1.19.0).
- `~/.claude/skills` exists and holds only `synced/` (claude.ai-managed).
- `~/.pi/agent` exists, but `~/.pi/agent/skills` does **not**. pi gets skills from the `git:github.com/obra/superpowers` package in its settings.
- `~/.config/opencode/skills` exists and holds 7 flat `caveman*` skill dirs.

## Implications for `climan install`

1. **Use a flat layout: `<dir>/<skill-name>/SKILL.md`.** Claude Code only finds direct children, while pi and opencode find flat skills as well as nested ones. A category layout would silently break Claude Code.
2. **Make the folder name equal to `name`, and validate it against the spec**: 1-64 chars, `^[a-z0-9]+(-[a-z0-9]+)*$`, description 1-1024 chars. Validate in climan, at generation time. opencode's docs require it and the spec requires it, while pi only warns and opencode's code doesn't check, so climan is the only place errors will surface.
3. **Watch for double-loading through shared folders.** opencode reads `~/.claude/skills` and `~/.agents/skills`, and pi reads `~/.agents/skills`. If climan installs the same skill into both `~/.claude/skills` and `~/.config/opencode/skills`, opencode sees two copies. It keeps one at random and logs a duplicate warning.
   - Option A: install opencode skills only to `~/.claude/skills`, or skip opencode when the Claude target is also selected.
   - Option B: target `~/.agents/skills` once to cover pi and opencode, and add `~/.claude/skills` for Claude Code. opencode will still see both, so the copies must be identical, or Claude must be disabled for opencode via `OPENCODE_DISABLE_CLAUDE_CODE_SKILLS`.
4. **Avoid name collisions with existing skills.** pi keeps the first skill found and opencode keeps an arbitrary one. In Claude Code, personal skills shadow project skills and can replace built-in commands of the same name.
   - Consider a prefix such as `<tool>-cli` or a namespaced name.
   - Refuse to overwrite a dir climan didn't create, for example by tracking ownership with a marker file.
5. **Never write into `~/.claude/skills/synced/`, and never name a skill `synced` or `anthropic-skills*`.**
6. **Resolve target dirs from the env vars:**
   - Claude: `$CLAUDE_CONFIG_DIR/skills`, else `~/.claude/skills`. On Windows, `~` is `%USERPROFILE%`.
   - pi: `$PI_CODING_AGENT_DIR/skills`, else `~/.pi/agent/skills`.
   - opencode: `${XDG_CONFIG_HOME:-~/.config}/opencode/skills` on all OSes. `OPENCODE_CONFIG_DIR` is additive, so it is an optional extra target and not a replacement.
7. **Project scope maps to:** `.claude/skills/`, `.pi/skills/` (pi needs project trust) or `.agents/skills/`, and `.opencode/skills/`. `.agents/skills/` is the one location that pi and opencode share.
8. **Symlinks are viable.** All three agents follow symlinked skill dirs, so `--link` mode could symlink to one canonical copy. Claude Code and pi dedupe by target, but opencode does not dedupe a skill reached through two symlinks into scanned dirs.
9. **Description budget:** Claude Code truncates description + `when_to_use` at 1,536 chars and may drop descriptions when many skills are installed. Keep generated descriptions short (well under 1024 chars), with the use case first.
