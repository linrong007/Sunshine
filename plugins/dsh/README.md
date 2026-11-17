# sunshine-dsh-plugin

Sunshine workflow hooks for DeepSeek Harness (DSH), as a Cordis plugin.

Scope (third tier, step A only — see `PLAN.md` "DSH 功能补齐"):

- `agent/created`: session-start context injection (startup/clear/compact;
  resume is skipped — the resumed session already has its context), by
  delegating to the existing `sunshine hook session-start` command.
- `agent/pre-step`: fallback injection if the `agent/created` path missed.
- `tools/post-execute`: after an Edit/Write/Bash that touches
  `.sunshine/spec/`, run `sunshine spec index` (fire-and-forget) and report
  the outcome to the next request via `additionalContexts`.

Not in this package: slash commands (step B, interactive UI only), headless
slash-command panel (DSH does not provide one), ACP/SDK profiles.

## Install

Per profile (a plugin is installed per profile, not globally):

```sh
dsh plugin --profile headless add file:/path/to/sunshine/plugins/dsh
dsh plugin --profile web add file:/path/to/sunshine/plugins/dsh
```

`package.json` declares `engines.dsh` (`^0.2.2-rc.1`); installing under an
incompatible DSH runtime is refused before anything is downloaded.

`dsh plugin add` needs `pnpm` on `PATH` (DSH shells out to it); this
sandbox had none, so pnpm 11 was installed locally for the install step.

Rebuilding: `pnpm file:` installs hardlink the package files. Editing a
file in place propagates, but `rm -rf lib && tsc` creates new inodes, so
the installed copy keeps the old `lib/`. After a rebuild, reinstall with
remove + add (a plain re-add is not enough — pnpm considers the same
version unchanged):

```sh
dsh plugin --profile headless remove sunshine-dsh-plugin
dsh plugin --profile headless add file:/path/to/sunshine/plugins/dsh
```

Verify the installed copy matches the build with
`md5sum plugins/dsh/lib/index.js ~/.dsh/profiles/headless/node_modules/sunshine-dsh-plugin/lib/index.js`.

## Design notes

- Session-file bridge: the Go hook resolves the active task from
  `.sunshine/.runtime/sessions/dsh_<session-id>.json`, but nothing on DSH
  writes that file (`sunshine task start` only writes it when a context
  key resolves, and DSH's bash tool exports no session id — there is no
  `DSH_SESSION_ID` in its environment). The plugin mirrors the
  `.current-task` pointer into that file (same JSON format as
  `task start` writes) before invoking the hook, and removes the mirror
  when there is no current task.
- Double-injection guard: the plugin injects via `agent/created` first
  and keeps `agent/pre-step` as a fallback. The fallback must not fire
  when the primary already delivered: at pre-step time the primary
  message is still in the inbox batch, not yet committed to the event
  log, so the projection alone still reads `injected: false`. The
  fallback therefore checks three things — an in-process flag, whether
  the proposed batch already contains a `sunshine-hooks` message, and
  the projection state (which covers resumes in a new process).
- `resume` is skipped in `agent/created` (a resumed session already
  carries its context); the projection replay keeps the pre-step
  fallback skipped as well.
- The plugin shells out to the `sunshine` binary on `PATH` (overridable
  with the `sunshineBin` config). If the binary is missing, injection
  silently yields nothing and spec sync reports a failure once — the
  session never breaks because of this plugin.

## Verify

```sh
dsh --profile headless --dump-config | grep -A 4 sunshine-hooks
```

## Verification status (2026-11-17, DSH 0.2.2-rc.1, sandbox)

Verified without a model key (headless runs fail at the model call with
`MISSING_CREDENTIAL`, which is expected — the plugin runs before that):

- `tsc` clean build; module exports `Config/apply/inject/name`.
- Plugin row present in `dsh --profile headless --dump-config`.
- Third-party bundle loads under headless: `apply()` ran and
  `agent/created` fired (`source=startup`, real session id and cwd).
- Session-start context lands in the session log as exactly one
  `user/message` with source `sunshine-hooks` (an early version injected
  twice — primary plus fallback — and was fixed with a three-part guard).
- Session-file bridge works: with `.current-task` set, the plugin wrote
  `.sunshine/.runtime/sessions/dsh_<session-id>.json`, and the injected
  text resolved the task (status COMPLETED shown).
- Resume adds no second injection (log grew, `sunshine-hooks` count
  stayed at 1).
- Spec-touch detection unit-checked 9/9 (edit/write `file_path` and
  `path`, relative paths, bash command text, patch text; `read` and
  unrelated calls correctly ignored).

Still needs a DeepSeek key (not available in this sandbox; not requested
from the user):

- End-to-end `tools/post-execute`: a real model-driven edit of a spec
  file, then confirm `sunshine spec index` ran in the background and the
  outcome surfaced via `additionalContexts` on the next request.
- Web profile install/load (the web profile is not initialized in this
  sandbox; install there with `dsh plugin --profile web add …` on a
  machine that has it).

## Maintenance (every DSH upgrade)

DSH is 0.2.x RC and its event API has been deprecated before
(`agent/model-select`). On each DSH upgrade: check `dsh --version`, watch
whether `engines.dsh` still passes on install, re-run `--dump-config` to
confirm the plugin row is still present, and smoke-test a headless run
(new-session injection appears; editing a spec file re-syncs the index).
