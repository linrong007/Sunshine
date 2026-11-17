import type { Context } from '@deepseek-ai/cordis';
import { type ContextFormed } from '@deepseek-ai/dsh-llm';
import z from '@deepseek-ai/schemastery';
/** Cordis plugin name used by loader diagnostics. */
export declare const name = "sunshine-hooks";
/** Services this plugin needs: agent registry, session projections, tools. */
export declare const inject: string[];
declare module '@deepseek-ai/dsh-llm' {
    interface MessageSourceMap {
        'sunshine-hooks': {
            kind: 'sunshine-hooks';
        } & ContextFormed;
    }
}
export interface Config {
    /** Sunshine CLI binary to invoke. Defaults to `sunshine`. */
    sunshineBin?: string;
    /** Spec directory relative to the project root. Defaults to `.sunshine/spec`. */
    specDir?: string;
}
/** Schemastery validation for {@link Config}. */
export declare const Config: z<Schemastery.ObjectS<NoInfer<{
    sunshineBin: z<string, string, "defined">;
    specDir: z<string, string, "defined">;
}>>, Schemastery.ObjectT<NoInfer<{
    sunshineBin: z<string, string, "defined">;
    specDir: z<string, string, "defined">;
}>>, "plain">;
/** The fields of an agent this plugin reads. */
interface AgentLike {
    session: {
        id: string;
        header: {
            cwd?: string;
        };
    };
    inject(message: unknown): void;
}
/** A tool call as seen by `tools/post-execute`. */
interface ToolExecutionLike {
    name: string;
    arguments: unknown;
    agent?: AgentLike;
}
/**
 * Mirror of Go `sanitizeKey` (internal/hook/active_task.go): replace
 * runs of disallowed characters with `_`, trim `._-` from both ends,
 * cap at 160 chars; empty result falls back to a sha256 prefix.
 */
export declare function sanitizeKey(raw: string): string;
/** Session-file key the Go hook looks up: `dsh_<sanitized session id>`. */
export declare function sessionFileKey(sessionId: string): string;
/** Walk up from `start` looking for a `.sunshine/` directory. */
export declare function findSunshineRoot(start: string): string | null;
interface SyncOutcome {
    ok: boolean;
    /** First line of the command's stdout, or the error message. */
    detail: string;
}
interface SessionSync {
    /** In-flight `sunshine spec index`, if any. */
    pending: Promise<SyncOutcome> | null;
    /** A spec touch arrived while a sync was running; run once more after. */
    rerun: boolean;
    /** Outcome line waiting to be reported via `additionalContexts`. */
    lastReport: string | null;
}
interface SessionEntry {
    /** Project root, or null when this session is not in a Sunshine project. */
    root: string | null;
    /** Session working directory (for resolving relative tool paths). */
    cwd: string;
    /** Cached session-start context; `undefined` means not computed yet. */
    context: string | null | undefined;
    /**
     * Set when this process delivered Sunshine context (via `agent/created`
     * or the pre-step fallback). The projection alone cannot guard the
     * fallback: at pre-step time the `agent/created` message is still in
     * the inbox batch, not yet committed to the event log, so the projection
     * still reads `injected: false` and the fallback would inject twice.
     */
    injectedInProcess: boolean;
    sync: SessionSync;
}
/**
 * Does this tool call touch a file under the project's spec directory?
 * Edit/Write carry `file_path` (dsh-tool-fs) or `path`
 * (dsh-tool-str-replace-editor); Bash carries `command`. A JSON scan is
 * the fallback for patch-style arguments.
 *
 * Exported for testing (the end-to-end path needs a live model call).
 */
export declare function touchesSpec(exec: ToolExecutionLike, entry: SessionEntry, specDirRel: string): boolean;
/**
 * Register the three listeners for the lifetime of `ctx`.
 * Session-start injection is best-effort: any failure leaves the session
 * running without Sunshine context, never failing agent creation.
 */
export declare function apply(ctx: Context, config: Config): Promise<void>;
export {};
//# sourceMappingURL=index.d.ts.map