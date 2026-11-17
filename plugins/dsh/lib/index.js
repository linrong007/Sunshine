/**
 * sunshine-dsh-plugin — Sunshine workflow hooks for DeepSeek Harness (DSH).
 *
 * Step A of the third tier (see `PLAN.md`, "DSH 功能补齐"): the hook
 * equivalents. Slash commands (step B) are not in this package.
 *
 * What it does:
 *
 * 1. `agent/created` — session-start context injection. For `startup` /
 *    `clear` / `compact` it runs the existing `sunshine hook session-start`
 *    command and injects its output as a source-attributed user message.
 *    `resume` is skipped: a resumed session already carries its context.
 *    The Go hook resolves the active task from
 *    `.sunshine/.runtime/sessions/dsh_<session-id>.json`; DSH itself never
 *    writes that file (its bash tool does not export a session id), so the
 *    plugin mirrors `.current-task` into it first — same file format as
 *    `sunshine task start` writes.
 * 2. `agent/pre-step` — fallback injection, same pattern as
 *    `dsh-time-context`: only on turn 1 / step 1, only if no Sunshine
 *    context message is in the session yet (tracked by a session
 *    projection that replays from the event log, so resumes stay skipped).
 * 3. `tools/post-execute` — after an Edit/Write/Bash that touches
 *    `.sunshine/spec/`, run `sunshine spec index` fire-and-forget and
 *    report the outcome (only when files changed, or on failure) to the
 *    next request via `additionalContexts`. The tool pipeline is never
 *    blocked: the sync runs in the background and a failed sync never
 *    changes the tool result.
 *
 * Nothing here ever throws into DSH: a listener failure is swallowed and
 * the session continues without Sunshine context, exactly like the Go
 * hooks' "exit 0, stay silent" rule.
 *
 * Typing note: DSH's own type graph (merged `Events` interface, projection
 * generics, zod v4) makes `tsc` run out of memory in this environment, so
 * the plugin talks to the host through small local structural interfaces
 * (`SunshineContext` and friends) instead of importing DSH's generic
 * types. The runtime contract is the same; only the compile-time surface
 * is narrowed. The one value import from DSH is `createUserMessage`.
 */
import { execFile } from 'node:child_process';
import { createHash } from 'node:crypto';
import { mkdirSync, readFileSync, rmSync, statSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve, sep } from 'node:path';
import { createUserMessage } from '@deepseek-ai/dsh-llm';
import z from '@deepseek-ai/schemastery';
import { z as zod } from 'zod';
/** Cordis plugin name used by loader diagnostics. */
export const name = 'sunshine-hooks';
/** Services this plugin needs: agent registry, session projections, tools. */
export const inject = ['agents', 'sessionProjections', 'tools'];
/** Schemastery validation for {@link Config}. */
export const Config = z.object({
    sunshineBin: z.string().default('sunshine'),
    specDir: z.string().default('.sunshine/spec'),
});
// ---------------------------------------------------------------------------
// Small pure helpers (mirroring the Go hook code where noted).
// ---------------------------------------------------------------------------
const SANITIZE_RE = /[^A-Za-z0-9._-]+/g;
/**
 * Mirror of Go `sanitizeKey` (internal/hook/active_task.go): replace
 * runs of disallowed characters with `_`, trim `._-` from both ends,
 * cap at 160 chars; empty result falls back to a sha256 prefix.
 */
export function sanitizeKey(raw) {
    let safe = raw.trim().replace(SANITIZE_RE, '_').replace(/^[._-]+|[._-]+$/g, '');
    if (safe.length > 160)
        safe = safe.slice(0, 160);
    if (safe === '')
        safe = createHash('sha256').update(raw).digest('hex').slice(0, 24);
    return safe;
}
/** Session-file key the Go hook looks up: `dsh_<sanitized session id>`. */
export function sessionFileKey(sessionId) {
    return `dsh_${sanitizeKey(sessionId)}`;
}
/** Walk up from `start` looking for a `.sunshine/` directory. */
export function findSunshineRoot(start) {
    let cur = resolve(start);
    for (;;) {
        try {
            if (statSync(join(cur, '.sunshine')).isDirectory())
                return cur;
        }
        catch {
            // Not here — keep walking up.
        }
        const parent = dirname(cur);
        if (parent === cur)
            return null;
        cur = parent;
    }
}
/** Build one source-attributed Sunshine context message. */
function sunshineMessage(text) {
    return createUserMessage({
        content: [{ type: 'text', text }],
        source: {
            kind: name,
            form: 'snapshot',
            sections: [{ name, text }],
        },
    });
}
const sessions = new Map();
function entryFor(sessionId, cwd) {
    const key = sessionId || `cwd:${cwd}`;
    let entry = sessions.get(key);
    if (!entry) {
        entry = {
            root: findSunshineRoot(cwd),
            cwd,
            context: undefined,
            injectedInProcess: false,
            sync: { pending: null, rerun: false, lastReport: null },
        };
        sessions.set(key, entry);
    }
    return entry;
}
// ---------------------------------------------------------------------------
// Session-start context: mirror `.current-task`, then run the Go hook.
// ---------------------------------------------------------------------------
/**
 * Bridge the DSH session to the Go hook's active-task lookup.
 *
 * The hook reads `.sunshine/.runtime/sessions/dsh_<session-id>.json`, but
 * nothing on DSH writes it: `sunshine task start` only writes that file
 * when a context key resolves, and DSH's bash tool exports no session id
 * (there is no `DSH_SESSION_ID` in its environment). So the plugin mirrors
 * the `.current-task` pointer — which `task start` always writes — into
 * the session file, in the same format. When there is no current task the
 * mirror file is removed so a finished task cannot linger as "active".
 * Never throws.
 */
function syncSessionFile(root, sessionId) {
    try {
        let currentTask = null;
        try {
            const ref = readFileSync(join(root, '.current-task'), 'utf8').trim();
            if (ref !== '')
                currentTask = ref;
        }
        catch {
            currentTask = null;
        }
        const dir = join(root, '.sunshine', '.runtime', 'sessions');
        const path = join(dir, `${sessionFileKey(sessionId)}.json`);
        if (currentTask === null) {
            rmSync(path, { force: true });
            return;
        }
        let data = {};
        try {
            data = JSON.parse(readFileSync(path, 'utf8'));
        }
        catch {
            data = {};
        }
        if (data['current_task'] === currentTask)
            return;
        data['current_task'] = currentTask;
        data['platform'] = 'dsh';
        data['last_seen_at'] = new Date().toISOString().replace(/\.\d+Z$/, 'Z');
        mkdirSync(dir, { recursive: true });
        writeFileSync(path, `${JSON.stringify(data, null, 2)}\n`);
    }
    catch {
        // A stale mirror is better than a broken session start.
    }
}
/** Run `sunshine hook session-start` and extract its context text. */
function runHookSessionStart(root, sessionId, sunshineBin) {
    return new Promise((resolvePromise) => {
        const child = execFile(sunshineBin, ['hook', 'session-start'], {
            cwd: root,
            timeout: 10_000,
            maxBuffer: 4 * 1024 * 1024,
            env: {
                ...process.env,
                SUNSHINE_HOOK_PLATFORM: 'dsh',
                CLAUDE_PROJECT_DIR: root,
            },
        }, (error, stdout) => {
            if (error || !stdout) {
                resolvePromise(null);
                return;
            }
            try {
                const parsed = JSON.parse(stdout);
                const hookOutput = parsed['hookSpecificOutput'];
                const text = (typeof hookOutput?.['additionalContext'] === 'string' ? hookOutput['additionalContext'] : '') ||
                    (typeof parsed['additional_context'] === 'string' ? parsed['additional_context'] : '');
                resolvePromise(text.trim() !== '' ? text : null);
            }
            catch {
                // Non-JSON stdout (e.g. a raw-text platform branch): use it as-is.
                resolvePromise(stdout.trim() !== '' ? stdout.trim() : null);
            }
        });
        // `input` is not on this @types/node's ExecFileOptions — feed stdin directly.
        child.stdin?.write(JSON.stringify({ session_id: sessionId, cwd: root }));
        child.stdin?.end();
    });
}
async function getContext(entry, sessionId, sunshineBin) {
    if (entry.context !== undefined)
        return entry.context;
    let text = null;
    if (entry.root !== null && sessionId !== '') {
        syncSessionFile(entry.root, sessionId);
        text = await runHookSessionStart(entry.root, sessionId, sunshineBin);
    }
    entry.context = text;
    return text;
}
// ---------------------------------------------------------------------------
// Spec-index sync (tools/post-execute).
// ---------------------------------------------------------------------------
/** Tool names whose arguments can change a spec file. */
const SPEC_TOUCH_TOOLS = new Set(['edit', 'write', 'bash', 'apply_patch']);
/**
 * Does this tool call touch a file under the project's spec directory?
 * Edit/Write carry `file_path` (dsh-tool-fs) or `path`
 * (dsh-tool-str-replace-editor); Bash carries `command`. A JSON scan is
 * the fallback for patch-style arguments.
 *
 * Exported for testing (the end-to-end path needs a live model call).
 */
export function touchesSpec(exec, entry, specDirRel) {
    if (entry.root === null || !SPEC_TOUCH_TOOLS.has(exec.name))
        return false;
    const specAbs = resolve(entry.root, specDirRel);
    const isInsideSpec = (p) => {
        const abs = resolve(entry.cwd, p);
        return abs === specAbs || abs.startsWith(specAbs + sep);
    };
    const args = (exec.arguments ?? {});
    if (typeof args['file_path'] === 'string' && isInsideSpec(args['file_path']))
        return true;
    if (typeof args['path'] === 'string' && isInsideSpec(args['path']))
        return true;
    if (exec.name === 'bash' && typeof args['command'] === 'string' && args['command'].includes('.sunshine/spec/')) {
        return true;
    }
    try {
        if (JSON.stringify(args).includes('.sunshine/spec/'))
            return true;
    }
    catch {
        // Unserializable arguments — fall through to "no".
    }
    return false;
}
/** One-line report for a finished sync; null when nothing is worth saying. */
function reportFor(outcome) {
    if (!outcome.ok) {
        return `Sunshine spec 索引同步失败：${outcome.detail}（不影响本次工具调用；可手动跑 sunshine spec index）`;
    }
    if (outcome.detail.includes('无需改动'))
        return null;
    const firstLine = outcome.detail.split('\n')[0] ?? '';
    return firstLine !== '' ? `Sunshine spec 索引${firstLine}` : null;
}
/**
 * Start (or coalesce) a background `sunshine spec index` for this
 * session. Never awaits: the caller returns to the tool pipeline at once.
 */
function requestSpecIndex(entry, sunshineBin) {
    const sync = entry.sync;
    if (sync.pending !== null) {
        sync.rerun = true;
        return;
    }
    const root = entry.root;
    if (root === null)
        return;
    const run = () => {
        sync.pending = new Promise((resolvePromise) => {
            execFile(sunshineBin, ['spec', 'index'], { cwd: root, timeout: 30_000 }, (error, stdout) => {
                if (error)
                    resolvePromise({ ok: false, detail: error.message });
                else
                    resolvePromise({ ok: true, detail: stdout.trim() });
            });
        });
        void sync.pending.then((outcome) => {
            sync.pending = null;
            sync.lastReport = reportFor(outcome);
            if (sync.rerun) {
                sync.rerun = false;
                run();
            }
        });
    };
    run();
}
/** Take a pending sync report, if one finished since the last tool call. */
function takeSyncReport(entry) {
    const report = entry.sync.lastReport;
    entry.sync.lastReport = null;
    return report;
}
// ---------------------------------------------------------------------------
// Plugin entry point.
// ---------------------------------------------------------------------------
/**
 * Register the three listeners for the lifetime of `ctx`.
 * Session-start injection is best-effort: any failure leaves the session
 * running without Sunshine context, never failing agent creation.
 */
export async function apply(ctx, config) {
    const sctx = ctx;
    const sunshineBin = config.sunshineBin ?? 'sunshine';
    const specDirRel = config.specDir ?? '.sunshine/spec';
    // Projection tracking whether this session already received Sunshine
    // context (a message with our source kind in the event log). Host-only
    // unit, no wire view — the state never leaves the process.
    const projectionReady = sctx.sessionProjections.register({
        key: 'sunshineHooks',
        stateVersion: 1,
        stateSchema: zod.object({ injected: zod.boolean() }),
        init: () => ({ injected: false }),
        apply: (state, event) => {
            if (event.type === 'user/message') {
                const source = event.data?.source;
                if (source?.kind === name) {
                    return state.injected ? state : { injected: true };
                }
            }
            return state;
        },
    });
    // 1. Session-start injection.
    sctx.on('agent/created', async ({ agent, source }) => {
        try {
            if (source === 'resume')
                return;
            const cwd = agent.session.header.cwd ?? process.cwd();
            const entry = entryFor(agent.session.id, cwd);
            if (entry.root === null)
                return;
            const text = await getContext(entry, agent.session.id, sunshineBin);
            if (text === null)
                return;
            agent.inject(sunshineMessage(text));
            entry.injectedInProcess = true;
        }
        catch {
            // Never fail agent creation because Sunshine context failed.
        }
    });
    // 2. Per-step fallback (same pattern as dsh-time-context): only the
    // first step of the first turn, only when no Sunshine context landed.
    sctx.on('agent/pre-step', async ({ agent, turn, step, signal }, next) => {
        const handle = await projectionReady;
        sctx.fiber.assertActive();
        const decision = await next();
        try {
            if (decision.kind === 'reject' || signal.aborted)
                return decision;
            if (turn !== 1 || step !== 1)
                return decision;
            const cwd = agent.session.header.cwd ?? process.cwd();
            const entry = entryFor(agent.session.id, cwd);
            if (entry.root === null)
                return decision;
            // Three guards against double injection (see `injectedInProcess`):
            // this process already delivered it, or it is already in the
            // proposed batch (inbox, not yet committed), or the event log
            // already contains it (e.g. a resumed session in a new process).
            if (entry.injectedInProcess)
                return decision;
            if (decision.messages.some((m) => m?.source?.kind === name)) {
                return decision;
            }
            if (handle.stateOf(agent.session).injected)
                return decision;
            const text = await getContext(entry, agent.session.id, sunshineBin);
            if (text === null)
                return decision;
            entry.injectedInProcess = true;
            return { ...decision, messages: [...decision.messages, sunshineMessage(text)] };
        }
        catch {
            return decision;
        }
    }, { prepend: true });
    // 3. Post-tool-use: sync the spec index after spec edits.
    sctx.on('tools/post-execute', async (exec, _result, next) => {
        const decision = await next();
        try {
            const cwd = exec.agent?.session.header.cwd ?? process.cwd();
            const entry = entryFor(exec.agent?.session.id ?? '', cwd);
            if (entry.root === null)
                return decision;
            if (!touchesSpec(exec, entry, specDirRel))
                return decision;
            requestSpecIndex(entry, sunshineBin);
            const report = takeSyncReport(entry);
            if (report === null)
                return decision;
            return {
                ...decision,
                additionalContexts: [...(decision.additionalContexts ?? []), sunshineMessage(report)],
            };
        }
        catch {
            return decision;
        }
    });
    await projectionReady;
}
//# sourceMappingURL=index.js.map