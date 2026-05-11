# ClassGo ↔ ycode Integration Plan

## Status

**Planning document.** Defines how ClassGo will consume [ycode](https://github.com/qiangli/ycode) as its AI provider, and surfaces gaps in ycode's public API that need resolution before — or during — the first feature build. Companion to [`ai-opportunity-map.md`](./ai-opportunity-map.md).

## What ycode actually provides (confirmed)

ycode is a **pure-Go CLI agent harness** with the following relevant capabilities (from `README.md`, `docs/architecture.md`, and `pkg/ycode/ycode.go`):

| Capability | Where | Useful for |
|---|---|---|
| Multi-provider LLM access (Anthropic native, OpenAI-compat covering OpenAI/xAI/Qwen/Ollama/OpenRouter) | `internal/api/` | All AI features |
| Embedded Ollama inference runner | `internal/inference/ollama.go` | Local-only model execution |
| Streaming agent loop with tool calls | `internal/runtime/conversation/runtime.go` | Chat features, agentic flows |
| MCP client + `ycode mcp serve` | `internal/tools/mcp_tools.go` | Exposing classgo data as tools to the agent |
| Five-layer memory + RRF retrieval (vector / Bleve FTS / keyword / entity) | `pkg/memex/` | RAG over Memos & tracker notes |
| Bonsai graph (DQL queryable) | `pkg/memex/graph` | Entity-relationship queries |
| **Embeddable Go API** in `pkg/ycode/` | `pkg/ycode/ycode.go` | Direct import — no IPC |
| Mountable `http.Handler` for REST+WS chat API | `Agent.Handler()` | Drop-in chat backend |
| Custom tool registration | `Agent.Registry()` | classgo-specific tools (`query_attendance`, etc.) |
| `serve` mode HTTP/WS endpoints | `cmd/ycode/serve.go` | Out-of-process alternative |
| Permission tiers: ReadOnly / WorkspaceWrite / DangerFullAccess | `internal/runtime/permission/` | Coarse safety lever |

## Recommended integration model: **embed `pkg/ycode/` as a library**

Three integration surfaces are possible:

| Option | What it looks like | Pros | Cons |
|---|---|---|---|
| **A. Embed `pkg/ycode/`** ✅ | `import "github.com/qiangli/ycode/pkg/ycode"`, call `ycode.NewAgent(...)`, mount `Agent.Handler()` on classgo's mux for chat, call `Agent.Chat()` for in-process invocations | No IPC; one binary; type-safe; provider/memory directly accessible | Tight version coupling; pulls all of ycode's deps into classgo's build |
| **B. Talk to `ycode serve` over HTTP/WS** | Run ycode as a sibling process; classgo's `internal/ai/client.go` is a thin REST/WS client | Decoupled lifecycle; can upgrade ycode independently | Two processes to manage; latency on the hot path; auth across the boundary |
| **C. Use MCP** | classgo exposes data via an MCP server; ycode is the MCP client | Standard protocol; reusable from other tools | Doesn't solve "where does the model run" — orthogonal to A/B |

**Recommendation: A**, with MCP (C) as the mechanism by which classgo's domain tools are registered into the embedded ycode agent. Rationale:

- `pkg/ycode.Agent.Handler()` returns a ready-to-mount `http.Handler`. The chat features (parent "Ask about my child", admin "Ask anything") drop in behind `RequireAuth`/`RequireAdmin` middleware with minimal new code.
- `pkg/ycode.WithOllama("")` auto-points at local Ollama — zero classgo config for the happy path.
- `Agent.Registry()` lets classgo register domain tools (`summarize_student`, `query_attendance`) without running a separate MCP server.
- One-shot extraction/summarization can either go through `Agent.Run()` (heavyweight — full agent loop) or directly call `api.Provider` (lighter — see gap #3 below).

## Sketch: classgo-side architecture

```
internal/ai/
  client.go        // Thin wrapper around ycode.Agent — lifecycle, health, role-aware request builder
  tools.go         // Register classgo-domain tools into agent.Registry(): query_attendance, summarize_student, list_audit_flags
  extract.go       // Helper for one-shot structured extraction (uses provider directly to avoid agent loop)
  summarize.go     // Helper for batch/one-shot summarization (same)
  prompts/         // Per-feature system prompts and JSON schemas
    signoff.go     // Schema for {sentiment, concern_flags[], suggested_followup}
    teacher_oneliner.go
    parent_digest.go
config.json (new ai block):
  ai:
    enabled: bool
    transport: "embedded" | "http"      // future-proof for option B
    ollama_url: ""                       // "" = ycode auto-detect
    model: "qwen2.5:7b-instruct"
    embed_model: "nomic-embed-text"
    features: { signoff_notes: true, teacher_oneliner: false, ... }
```

All AI-using handlers depend on a single `*ai.Client` injected at startup. When `ai.enabled=false`, the client returns no-op stubs and AI-gated UI hides itself.

## Gap analysis — what's missing or unclear in ycode for this use case

Severity legend: **🛑 blocker** (must resolve before any feature ships), **⚠️ important** (must resolve before chat/multi-user features), **ℹ️ nice** (workaround exists).

### 1. 🛑 Tool-set bleed-over / sandbox escape

`pkg/ycode.NewAgent()` unconditionally calls `tools.RegisterBuiltins(a.registry)` which registers `bash`, `read_file`, `write_file`, `edit_file`, `grep_search`, etc. **If we mount `Agent.Handler()` behind `RequireAdmin`, an authenticated admin chatting could ask the LLM to run shell commands on the classgo host.** That's a critical privilege escalation — admin = "manage student records" ≠ admin = "run bash on the server".

**Resolution options** (require coordination with ycode):
- Upstream a `pkg/ycode.WithoutBuiltinTools()` option, or `WithToolRegistry(custom)` that bypasses `RegisterBuiltins`.
- Until that lands: post-construction `Agent.Registry().Unregister(...)` for every builtin — fragile, depends on internal naming staying stable.
- Pin permission mode hard to `read-only` AND verify in tests that none of the readwrite tools are reachable. This still leaves `read_file` exposed.

This is the single most important gap to close.

### 2. 🛑 Multi-tenant session model

`NewAgent()` creates one `*session.Session` per `Agent` instance. ClassGo has many concurrent users (teachers viewing dashboards, parents asking chat questions). Options:

- **One Agent per classgo user** — leaks memory; ycode's persona/dreaming would conflate users.
- **One Agent total, sessions multiplexed by `Agent.Handler()`** — what does the REST API actually do? The `Agent.Handler()` returns the same Mux that `ycode serve` uses, which has `/api/sessions` endpoints. Need to confirm: does each `POST /api/sessions` create an isolated state, or does it share memory/persona with siblings?
- **One Agent per feature, stateless** — for one-shot extraction/summarization this works (no session needed). For chat it doesn't.

Until we read `internal/server/` we won't know the multi-tenant story. **Action**: verify `Agent.Handler()` session isolation before designing parent/admin chat.

### 3. ⚠️ No one-shot structured-output API exposed

For smart signoff notes / profile normalization, we want **one LLM call returning JSON-schema-constrained output** — no tool loop, no memory injection, no session. `pkg/ycode` exposes `Agent.Run()` (full agent loop) and `Agent.Chat()` (full agent loop). The underlying `api.Provider` does support structured output (`StructuredOutput` is listed as a tool in `docs/usage.md`), but the public `pkg/ycode` doesn't expose a `func (a *Agent) Extract(ctx, schema, prompt) (json.RawMessage, error)`.

**Resolution options**:
- Upstream `Agent.Extract(...)` helper.
- Reach for `a.provider` via reflection — not viable, it's unexported.
- Until then: implement extraction by calling Ollama directly via `net/http` against `http://localhost:11434/api/chat` with `format: <jsonschema>`. ycode still "owns" the Ollama install and model pull; classgo bypasses ycode for these calls. Acceptable workaround; technical debt small.

### 4. ⚠️ Embeddings API not exposed in `pkg/ycode`

ycode's memex uses vector retrieval internally but `pkg/ycode` only exposes `Storage() *store.Manager` and `Memory() *memory.Manager`. Whether either offers a clean `Embed(text) []float32` is unconfirmed from the docs.

**Resolution options**:
- Upstream `Agent.Embed(text)`.
- Until then: call Ollama `/api/embeddings` directly with `nomic-embed-text`. Same shape of workaround as gap #3.

### 5. ⚠️ Role / actor propagation to custom tools

When classgo registers `query_attendance(student_id)` and the agent decides to call it, the tool needs to know **who** the requesting user is so it can enforce role scoping (parent can only query their own child). The standard `tools.Tool` interface takes execution context but not "the classgo user behind this turn".

**Resolution options**:
- Per-Agent factory: stamp the requesting user into a `context.Context` value before `Agent.Chat()`, and tools read it. Works if `Agent.Chat()` propagates the context down to tool execution (likely; need to verify).
- If not propagated: classgo must close over the user identity at tool registration time, meaning a fresh `*ai.Client` per request. Expensive.

**Action**: verify context propagation via a small spike test.

### 6. ⚠️ Permission-mode mismatch

ycode's modes are `ask` / `read-only` / `workspace-write` / `danger-full-access`. None of these map onto "parent can ask about own child only". Permission enforcement has to happen **inside each custom tool**, not at ycode's permission layer. Acceptable but worth being explicit about.

### 7. ℹ️ Persona / memory tenancy

`_persona_{id}.md` is currently a single file in `~/.agents/ycode/memory/`. For embedded use we want per-classgo-user (or fully disabled) personas. The architecture doc says `PersonaEnabled` config exists. **Action**: confirm we can disable it via `pkg/ycode` options; if not, post-construction config mutation.

### 8. ℹ️ Build-size and dependency surface

`pkg/ycode/ycode.go` imports `internal/api`, `internal/bus`, `internal/cli`, `internal/inference`, `internal/runtime/config|session`, `internal/server`, `internal/service`, `internal/tools`, `pkg/memex/*`. That's a lot. Expect the classgo binary to grow significantly (estimated 30–80 MB on top of current size depending on which features compile in). Mitigations: build tags, vendoring only-what-we-use. Tolerable for v1 of the integration.

### 9. ℹ️ Versioning / stability

ycode is at `v0.1.0-139-g00a5102-dirty`. Pre-1.0; surface may break. Pin to a tag or commit SHA, treat upgrades as project work.

### 10. ℹ️ Tools list is developer-coded

Of ycode's 50+ builtin tools, almost all are dev-tooling (bash, glob, LSP, etc.). The general-purpose ones we'd want (text summarization, classification) are *not* tools — they're just LLM calls. So most of ycode's tool surface is irrelevant or harmful (#1) to classgo's use. Reinforces gap #1's "strip the builtins" requirement.

## Open questions for the user

These determine the shape of the first PR. Recommend resolving before starting:

1. **Embed vs serve-mode?** Plan recommends embed (A). Confirm or pick B.
2. **Strip builtins or upstream `WithoutBuiltinTools()`?** Stripping is faster; upstreaming is correct. Either way, gap #1 must be closed before any chat endpoint ships.
3. **Anonymize student names before sending to the LLM?** Even with local Ollama, pseudonymizing (`Alice` → `Student_A`) limits prompt-injection blast radius and makes logs safer.
4. **Where is the model pulled?** ycode can manage `qwen2.5:7b` etc. Should classgo's first-run setup auto-pull the model, or document `ollama pull` as a prereq?

## Phasing

This plan does not commit code. When approved, the build order is:

1. **Spike: confirm gap #1, #2, #5 unknowns** (~half-day). Read `internal/server/`, run a minimal `pkg/ycode.NewAgent()` + custom-tool test, verify context propagation. **No production code yet.**
2. **`internal/ai/` skeleton with stubbed `Client`** (~half-day). Interface, config block, disabled-by-default. Tests against a fake provider — no ycode dep yet.
3. **Wire ycode** (~1 day). Embed `pkg/ycode`, set up Ollama auto-detect, strip/upstream builtin tools per #1 resolution, expose `Client.ExtractJSON(ctx, schema, prompt)` and `Client.Summarize(ctx, prompt)`.
4. **First feature: Smart signoff notes** (~1 day) — opportunity-map item #2. Adds DB column on `TrackerResponse`, calls `Client.ExtractJSON` in `HandleTrackerRespond`, renders concern flags on teacher dashboard.

Stop after step 4 and evaluate. The remaining shortlist (teacher one-liners, parent digest, chat features) builds on the same plumbing.

## Verification (when work moves to implementation)

- `make test` — all existing tests pass; AI features additive and config-gated.
- New `internal/ai/` package: unit tests using a fake `api.Provider` implementation; golden-file tests for each prompt + schema pair.
- Integration test that confirms gap #1 is closed: spawn `*ai.Client`, attempt to invoke `bash` tool via Chat, assert it's not available.
- For chat features: integration test that confirms role scoping — parent A cannot retrieve parent B's child data via chat or tool call.
- Latency budgets: extract/classify p95 < 1s; summarize p95 < 3s; chat first-token < 1.5s. All on local Ollama + Mac mini-class hardware.
- Document required `ollama pull <model>` step in CLAUDE.md once model choice is locked.

## Critical files referenced

- `pkg/ycode/ycode.go` (ycode) — embedding API surface. Confirms `Chat`, `Run`, `Handler`, `Registry`, `Memory`, `Storage`, `Graph`.
- `docs/architecture.md` (ycode) — conversation runtime, memory hierarchy, permission modes.
- `docs/usage.md` (ycode) — tools list, including `StructuredOutput`.
- `internal/auth/` (classgo) — session/identity that must be propagated into every AI call.
- `internal/handlers/tracker.go::HandleTrackerRespond` (classgo) — site of the first feature build.
- `config.json` (classgo) — destination for the new `ai` block.
