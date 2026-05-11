# ClassGo — AI Opportunity Map

## Status

**Reference document.** Catalogs viable AI integration points across roles, the SOTA pattern each maps to, and a prioritized shortlist. **No implementation is committed by this document.**

## Architecture note: ycode as the AI provider

ClassGo will be a **client/consumer** of [ycode](https://github.com/qiangli/ycode), which provides Ollama support, a chat service, and a broader agentic toolset (treesitter symbol search, code knowledge graph, memex, browser fetch, sandboxed execution, etc.) behind both MCP servers and `yc <verb>` shell built-ins.

Implication for everything below:

- The original draft of this map assumed ClassGo would ship its own `internal/ai/` Ollama HTTP client. That assumption is **superseded** — model access, embeddings, tool-calling, **conversational chat**, and (potentially) RAG infrastructure are delegated to ycode.
- The "Recommended stack" section below documents what an in-process Ollama integration *would* look like for reference; in practice ClassGo will instead call ycode (MCP or `yc` CLI) and ycode will own the model runtime, chat orchestration, prompt scaffolding, and provider abstraction.
- This means pattern #7 (role-scoped conversational chat) and the chat-shaped opportunities below (parent "Ask about my child", admin "Ask anything") become **thin proxies** over ycode's chat service rather than full builds — ClassGo's job narrows to: auth/role gating, per-role system prompt + tool allowlist, retrieval scope enforcement, and rendering.
- **The classgo ↔ ycode integration plan is TBD** — surface choice (MCP client vs. CLI shell-out vs. HTTP), auth boundary, latency budget, streaming transport for chat, and feature-flag mechanism will be specified in a separate document when the work is scheduled.
- The opportunity catalog and prioritization below remain valid regardless of provider — they describe *where* AI helps, not *how* the model is reached.

## Context (original framing)

ClassGo is a tutoring/check-in/attendance app with embedded Memos notes, serving five roles: **student, parent, teacher, admin, superadmin** (plus unauth kiosk). There is **zero existing LLM integration** today — verified by grepping for `ollama|openai|anthropic|llm|gpt|claude|gemini` across `internal/`, `main.go`, and `go.mod`.

Privacy framing: student PII (names, grades, attendance, parent contacts, behavioral notes) makes external LLM providers a hard sell. A local Ollama runtime — now via ycode — keeps data on-prem, which is the natural fit. All recommendations assume that constraint.

---

## SOTA patterns this app can use

Integration patterns currently mainstream in production LLM apps (late 2025/early 2026), ordered by maturity/ease:

| # | Pattern | What it is | Best for | Risk |
|---|---|---|---|---|
| 1 | **Structured extraction (JSON mode / constrained decoding)** | LLM returns typed JSON conforming to a schema. Modern Ollama supports `format: json` and `format: <json-schema>` natively. | Classifying free-text into enums, extracting entities from notes, parsing addresses, normalizing school names. | Lowest. Output is validated; failures are caught at decode. |
| 2 | **Summarization / report narratives** | LLM converts structured rows into prose. Often paired with a Jinja-like prompt template + retrieval of recent records. | Parent digests, teacher class summaries, audit log narratives, weekly check-in reports. | Low. Hallucinations possible — mitigate by quoting source numbers verbatim and only narrating *interpretation*. |
| 3 | **Retrieval-Augmented Generation (RAG)** | Embed Memos content / profile notes / past responses; retrieve top-k at query time and stuff into context. Common stack: `nomic-embed-text` (Ollama) → SQLite-vec or in-memory FAISS → small chat model. | "How is Alice doing?" cross-corpus questions; teacher Q&A over their student history; admin search over Memos. | Medium. Need to enforce role scoping at retrieval (parent can only retrieve own child's data). |
| 4 | **Tool-using / function-calling agents** | Model calls typed functions (Ollama supports OpenAI-style tool calls on Llama 3.1+, Qwen 2.5, Mistral). Agent reads DB, drafts changes, asks for human approval. | Admin: "draft a tracker item for students missing Q2 enrollment forms"; teacher: "assign weekly reading log to my Tuesday class". | Higher. Needs allowlisted tools, dry-run/confirm UI, audit log. Don't let an agent write directly. |
| 5 | **Inline assist (autocomplete-style)** | LLM completes a free-text field as the user types, with a Tab-to-accept ghost text. | Tracker item descriptions, criteria fields, teacher notes, admin profile notes. | Low–medium. Latency-sensitive; local model must be small (≤7B) and quantized. |
| 6 | **Anomaly explanation** | LLM doesn't *detect* the anomaly (rules do that) — it *explains* it in human terms and proposes next steps. | Audit flags ("why was this checkin suspicious?"), schedule conflicts, attendance dips. | Low. Deterministic detection + LLM narration is a robust split. |
| 7 | **Conversational chat (role-scoped)** | A `/assistant` chat surface that uses RAG + tool calls, scoped to the user's permissions. The "AI sidekick" pattern (Notion AI, Linear Asks, Intercom Fin). | Each role gets a different system prompt + tool allowlist. | Highest UX complexity; lowest payoff per dev-day. |

**Reference stack (superseded — ycode now owns this layer):**
- ~~Runtime: Ollama HTTP API at `http://localhost:11434` (config-driven URL).~~
- Models for reference: `qwen2.5:7b-instruct` or `llama3.1:8b-instruct` for chat/tool use; `nomic-embed-text` for embeddings; `qwen2.5:1.5b` for low-latency extraction tasks.
- ~~Go client: thin `internal/ai/` wrapper over the Ollama REST API.~~
- Storage for embeddings (if classgo holds an index rather than ycode): reuse existing SQLite via `sqlite-vec`, or a single `embeddings` table with cosine-sim computed in Go (fine up to ~100k vectors at this scale). **Likely delegated to ycode.**
- Config: a single `ai` block in `config.json` — `enabled`, ycode endpoint/transport, per-feature toggles. Exact shape TBD with the integration plan.

---

## Per-role opportunity catalog

References below use `file.go:HandlerName` to point at concrete integration sites. Volume estimates assume a center with ~100 students, ~20 teachers, ~150 parents.

### 1. Student (and parent — shared `/dashboard`)

**Touchpoints**: tracker signoff at checkout, viewing assigned tasks, viewing own attendance, Memos.

| Idea | Pattern | Integration site | Value |
|---|---|---|---|
| Smart signoff notes | Structured extraction (#1) | `handlers/tracker.go::HandleTrackerRespond` — after submit, classify `notes` into `{sentiment, concern_flags[], suggested_followup}` and stash on `TrackerResponse`. | Surfaces "struggling", "sick", "family event" signals to teachers without extra UX. |
| Plain-language attendance summary | Summarization (#2) | `handlers/dashboard.go` student view. Convert attendance rows + task completion into a 2–3 sentence narrative ("You came in Mon/Wed/Fri this week and finished 6 of 8 tasks…"). | Friendlier UX; transparent to parents. |
| Memos auto-tagging | Structured extraction (#1) | `internal/memos/sync.go` — on note write, extract `{themes[], people_referenced[], action_items[]}`. | Powers downstream search & report enrichment. |

### 2. Parent

**Touchpoints**: view child progress, scheduled email digests (`reports/parent.go`), Memos read access.

| Idea | Pattern | Integration site | Value |
|---|---|---|---|
| Weekly activity digest narrative | Summarization (#2) | `reports/parent.go::ReportParentChildActivity`. Wrap existing data with a friendly 1-paragraph summary + 1 suggested talking point. | High — parents are the audience least likely to read tabular data. |
| "Ask about my child" chat | RAG + chat (#3 + #7) | New endpoint `/api/ai/ask` gated by `RequireAuth` + parent-child relationship check. Retrieval scope: only own child's attendance, tasks, teacher notes flagged shareable. | Medium — high-touch UX but high engagement. Defer until extraction layer is built. |

### 3. Teacher

**Touchpoints**: `My Classes`, `My Students`, tracker assignment, weekly timesheet, time-off, Memos.

| Idea | Pattern | Integration site | Value |
|---|---|---|---|
| Per-student status one-liners | Summarization (#2) | `handlers/dashboard.go::HandleDashboardAllTasks` teacher view. For each of 20–50 students, render a one-sentence status drawn from last 4 weeks of attendance + signoff notes + flagged TrackerResponse extractions. | **High.** Highest dev-effort-to-value ratio. Teachers triage at a glance. |
| Inline assist on tracker item creation | Inline assist (#5) | `handlers/tracker.go::HandleTrackerCreate` UI. Ghost-text completion for `Name`, `Notes`, `Criteria`. | Medium. Lowers friction on a 4–6 field form. |
| Class-level intervention suggestions | Tool-using agent (#4), human-in-the-loop | New admin/teacher panel. Agent reads class roster + recent tasks, drafts up to 3 candidate tracker items, presents as "Review & assign" — never auto-executes. | Medium-high; needs careful guardrails. |
| Memos draft starter | Inline assist (#5) | Within embedded Memos. Hard because Memos is upstream-vendored React — would need a sidecar UI overlay rather than fork. | Low priority. |

### 4. Admin

**Touchpoints**: directory CRUD, attendance review, scheduled reports, data import, tracker library, audit, PIN/config.

| Idea | Pattern | Integration site | Value |
|---|---|---|---|
| Import conflict resolver | Tool-using agent (#4), human-in-the-loop | `handlers/import.go::HandleImportUpload`. For each `NamelistConflict`, agent computes match confidence over name/grade/school/parent fields, proposes `merge` vs `create-new` with a reason, admin clicks to confirm. | **High.** Bulk import is the most painful admin workflow. |
| Audit log narratives | Anomaly explanation (#6) | `handlers/audit*` or `reports/admin.go` weekly summary. Rules detect; LLM narrates ("3 students checked in within 4 seconds from the same device — likely shared kiosk, not fraud"). | High. Turns alerts from noise into prioritized triage. |
| Profile normalization on save | Structured extraction (#1) | `handlers/profile.go::HandleStudentProfile`. Validate/standardize: phone format, address components, school name against registry, grade vs. major sanity check. Save canonical + raw. | Medium. Cumulative win across hundreds of records. |
| Cross-corpus admin chat | RAG + tools (#3 + #4 + #7) | New `/admin/ai` page. Tools: `query_attendance`, `list_audit_flags`, `summarize_student`. RAG over Memos + tracker notes. | Medium. The "Ask anything" headline feature — high marketing value, requires the foundation work first. |
| Report narrative augmentation | Summarization (#2) | `reports/admin.go`, `reports/teacher.go`, `reports/timesheet.go`. Append a 2–3 sentence executive summary to each scheduled report. | Medium. |

### 5. SuperAdmin

**Touchpoints**: scheduler, tunnel, system config. Mostly operational.

| Idea | Pattern | Integration site | Value |
|---|---|---|---|
| Schedule conflict explainer | Anomaly explanation (#6) | `internal/scheduling/engine.go` — wrap `DetectConflicts` results with LLM narrative + proposed resolutions ("Move section B to room 3 — only conflict is one student also in section A"). | Medium, niche audience. |
| AI provider health & feature toggle UI | Plumbing | `handlers/app.go` admin settings page. Show ycode reachability, active model(s), last ping latency, per-feature toggle. | Required if any AI ships. |

### 6. Kiosk / unauthenticated checkin

Deliberately excluded. Adding LLM-in-the-loop on the checkin hot path costs latency and risks denial-of-service via slow model. Keep this path pure-Go.

---

## Prioritized shortlist (build order)

Ranked by **value × ease**, given that the AI provider integration needs to be built once and amortized:

1. **Foundation: classgo↔ycode integration layer** — replaces the original "Ollama client" item. Owns: transport (MCP or `yc` CLI), feature-flag config, health check, role-aware request envelope. Effort TBD pending the integration plan; treat as a prerequisite for everything below.
2. **Smart signoff notes** (#1 pattern, `tracker.go::HandleTrackerRespond`) — touches every checkout, demonstrates the extraction pattern, easy to A/B against the raw notes field. ~1 day on top of foundation.
3. **Per-student status one-liners for teachers** (#2 pattern, dashboard teacher view) — the single highest-leverage feature. Daily-use surface, 20–50 summaries per teacher per view. ~2 days.
4. **Weekly parent digest narrative** (#2 pattern, `reports/parent.go`) — reuses #2's prompt scaffolding, fires on existing scheduler, observable improvement to a low-engagement channel. ~1 day.
5. **Import conflict resolver** (#4 pattern, `handlers/import.go`) — biggest admin pain point. Human-in-loop keeps risk low. ~2–3 days.
6. **Audit log narratives** (#6 pattern, `handlers/audit*` / `reports/admin.go`). ~1 day once #2 is in.
7. **Profile normalization on save** (#1 pattern, `profile.go::HandleStudentProfile`). ~1–2 days.
8. **Admin "Ask anything" chat** (#3 + #4 + #7) — thin proxy over ycode's chat service: auth gating, admin-scoped tool allowlist, retrieval scope, rendering. Effort drops significantly versus a from-scratch build; sequencing depends on whether chat is desired before or after the extraction/summary wins of items 2–7. RAG corpus likely via ycode memex.
9. **Class-level intervention suggestions for teachers** (#4) — gated behind admin opt-in. ~3 days.
10. **Schedule conflict explainer** (#6, superadmin). ~1 day.

Items 1–4 form a coherent first slice: one shared integration layer plus three user-visible features spanning student, teacher, and parent roles. Items 5–10 are independent and can be sequenced by demand.

---

## Critical files referenced

- `main.go` — route registration (middleware tiers: public / `RequireAuth` / `RequireAdmin`).
- `internal/auth/` — Identity, Session, `IsSuperAdmin` — gates for role-scoped AI features.
- `internal/models/models.go` — Student (20+ text fields), TrackerItem, TrackerResponse, Attendance.
- `internal/handlers/tracker.go` — primary signoff/response surface.
- `internal/handlers/dashboard.go` — teacher/student/parent dashboard views.
- `internal/handlers/profile.go` — profile save flow.
- `internal/handlers/import.go` — namelist upload + conflict detection.
- `internal/handlers/reports.go`, `internal/reports/*.go` — report generation per role.
- `internal/memos/sync.go` — Memos content sync (RAG source).
- `internal/scheduling/engine.go` — schedule materialization & conflict detection.
- `config.json` — where the new `ai` config block goes.

---

## Verification (when an idea is picked up for implementation)

This document does not commit code. When any opportunity moves to implementation, baseline checks before merging:

- `make test` — all existing tests pass (AI features should be additive, gated by config).
- The classgo↔ycode integration layer needs unit tests against a fake ycode endpoint (`httptest.NewServer` or a stub MCP server) and golden-file tests for structured-extraction prompts.
- Manual end-to-end with ycode running locally and a small model pulled (e.g. `qwen2.5:1.5b` for tests). Document the required ycode setup in CLAUDE.md when integration ships.
- For any RAG/agent feature, integration test that confirms **role scoping**: a parent's query cannot retrieve another family's child data. This is non-negotiable.
- Latency budget for inline/interactive features: p95 < 1.5s on a Mac mini. Use streaming + small models on the hot path; reserve 7B+ for background jobs (digests, reports).
