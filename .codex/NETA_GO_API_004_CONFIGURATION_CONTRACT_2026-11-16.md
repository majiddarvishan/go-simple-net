# neta-go — API-004: Go Configuration Contract (Design Only)

**Date:** 2026-11-16  
**Status:** CLOSED — APPROVED OPTION A (Functional Options), 2026-11-16; precise exported names still under API-001  
**Repository:** `majiddarvishan/neta-go` / `main`  
**Implementation:** **NOT AUTHORIZED** (no application source, Go module, examples, test runs, build or repository rename).

## 1. Goal and approved constraints

Design a small, Go-native mechanism for independently configurable Client/Server/Session defaults without ambiguity between **omitted** settings and **explicitly configured zero**.

Already approved in [Decision Log](DECISION_LOG.md):
- **Q-002/Q-006:** automatic Enquire Link is only sent after no real session traffic, with **30s default idle interval**; an **explicit 0 disables outgoing probes**, while responses to peer probes remain enabled.
- **Q-007:** Enquire Link response deadline **10s default**, separately configurable; on missing matching response, close Session; reconnect Client only if enabled.
- **Q-008:** per-Session outgoing Application FIFO **4 MiB default**, independently configurable; Queue Full returns promptly without closing Session.
- **Q-009:** incoming Application frame that does not fit the per-Session callback backlog closes **only that Session**.
- **Q-010 A:** **all** ingress/global/admission/Control parameters are **independently configurable** with defaults: incoming **4 MiB per Session**, Server queued Application frames **256 MiB**, bound Sessions **1024**, pending Pre-Bind connections **128 per listener**, absolute Pre-Bind deadline **30s**, Control allowance **4 KiB per Session**. On global overload: outgoing fails Queue Full without close; incoming closes offender; new Bind is rejected at bound capacity, without evicting existing peers.
- **Q-003:** concurrent-safe public Client/Server/Session methods and callback isolation; this requires immutable, race-free effective settings.

All numbers are defaults, not fixed capacity, and **256 MiB is only a queued Application frame budget, not total process RSS**. This document proposes *how* to express these settings and validate them, not new numeric defaults.

## 2. Core ambiguity that must be eliminated

A basic Go struct with a `time.Duration` field has a zero value of `0`. It cannot distinguish these two application intentions without a presence mechanism:

| Application intent | Correct effect |
|---|---|
| No idle interval supplied | Use **30s** default |
| Explicitly supply idle interval `0` | Disable autonomous outgoing Enquire Link (still answer received probes) |
| Supply positive idle interval | Send only after that much real inactivity |
| Supply negative idle interval | Return validation error; do not start networking |

Similarly, an **omitted** resource cap means its positive, approved default; an **explicit zero** for a required bounded cap should be **invalid**, not silently treated as "unlimited" or the default. An `EnquireLinkResponseTimeout` of zero/negative should also be rejected unless a later explicit policy defines another meaning.

## 3. Public API configuration strategy — APPROVED OPTION A

### Option A — Functional Options, with internally applied defaults (**OWNER APPROVED**)

- Client and Server constructors each begin with their own **complete internal default set**.
- Every override is an **explicit typed option** applied after defaults, e.g. conceptual `WithEnquireLinkIdleInterval(0)`, `WithMaxQueuedOutgoingBytes(custom)` and server-only `WithMaxBoundSessions(custom)`. These are **proposed names**, not released code.
- Missing options leave defaults untouched; an explicit Enquire Link option with value `0` is distinguishable and **must disable outgoing probes**.
- A default-settings reference, or a `DefaultClientConfig`/`DefaultServerConfig` factory, may be exposed for inspection/advanced use, but callers should **not** need to pass a mutable pre-filled struct just to get defaults. If a struct is exposed, define its normalization and presence rules explicitly.
- Ensure Client-only and Server-only options cannot be confused; common Session options may be reusable, but the surface must remain small.
- Apply all options to a **private effective configuration**, validate it as a unit, and retain an immutable snapshot for the active Client/Server/Session lifecycle.
- Constructors should reject invalid options **before dialing/listening**. A failed constructor must not allocate active sockets, goroutines or partial resources.

**Pros:** explicit intent, clean zero semantics, no proliferation of public pointer fields; good Go user experience.  
**Tradeoff:** more constructor options; can group options by Config domain without adding runtime mutation.

### Option B — Explicit optional/pointer Config fields (**NOT SELECTED**)

- Public `ClientConfig`/`ServerConfig` stores presence (e.g. optional/pointer duration for `EnquireLinkIdleInterval` and optional numeric fields where omission matters).
- `nil`/not-present means default; explicitly present `0` on idle interval disables sending; explicitly present zero for required capacities is invalid.
- Resolver builds a private, fully populated effective Config, validates, and snapshots it at construction/start.
- Never retain references to mutable caller-owned pointer fields.

**Pros:** easy declarative config and mapping from configuration files.  
**Tradeoff:** pointer/presence overhead and more verbose initialization; a plain `time.Duration` without presence is still incorrect.

**Decision:** owner approved **Option A** on 2026-11-16. The typed Functional Options strategy is binding; detailed exported identifier spelling and constructor signatures remain design proposals. Option B is not selected. No implementation has been authorized.

## 4. Config keys and validation proposal

The keys below describe semantic configuration, not final exported Go API identifiers.

| Key | Approved default | Applies | Draft validation |
|---|---:|---|---|
| EnquireLinkIdleInterval | **30s** | Client + Server Session | `>=0`; explicit `0` disables autonomous requests |
| EnquireLinkResponseTimeout | **10s** | Client + Server Session | `>0` even if idle interval disabled; no infinite wait |
| MaxQueuedOutgoingBytes | **4 MiB** | Client + Server Session | finite positive byte count |
| MaxQueuedIncomingBytes | **4 MiB** | Client + Server Session | finite positive byte count |
| MaxQueuedControlBytes | **4 KiB** | Client + Server Session | finite positive byte count; large enough for supported control frames |
| MaxAggregateQueuedFrameBytes | **256 MiB** | Server | finite positive byte count |
| MaxBoundSessions | **1024** | Server | positive integer |
| MaxPendingPreBindSessions | **128** | Server/listener | positive integer |
| PreBindHandshakeTimeout | **30s** | Server/listener | positive finite duration, absolute from accept |

Additional C++-observed reference values (client Connect/Bind 30s, reconnect delay 5s, Unbind 10s) are documented elsewhere; **they are not promoted here to approved Go defaults**. The max protocol PDU length remains separate from all queue byte limits and must retain Pinex C++ 1.5.0 wire compatibility.

### Cross-field validation — draft

- Validate all settings **after** overrides are applied, not each default independently, to avoid false conflicts during construction.
- Prevent integer overflow when converting MiB to byte counts or accumulating queued frame charges. Include the 10-byte Pinex frame header where the applicable byte budget specifies full serialized frames.
- Do not require `MaxAggregateQueuedFrameBytes >= MaxBoundSessions * MaxQueuedIncomingBytes`: that would effectively negate the approved dynamic global budget. Permitting a global cap smaller than the theoretical sum is intentional.
- A configured incoming or outgoing queue bound **does not increase maximum PDU size**. Oversized PDU handling stays with protocol validation.
- For a frame larger than the configured queue capacity, apply the approved admission result (outgoing Queue Full; incoming close offender) without unbounded temporary allocation.
- Any user-supplied byte cap smaller than the minimum supported frame size should be rejected during configuration validation where practical. The minimum valid bound and accounting for an executing callback are **technical details for API-001/GO-PINEX-010**.
- The 4 KiB Control allowance must remain separate/bounded and preserve single-writer semantics and Pinex Unbind ordering/10s deadline. Do not interpret its presence as a priority queue or permission to overtake application frames.
- Include a clear field-specific error category (e.g. `invalid_config`) for invalid overrides. Exact exported error names are not decided.

## 5. Resolution, ownership and lifetime — approved principles, draft API detail

1. **Defaults:** build complete defaults when constructing the Client or Server.
2. **Overrides:** apply only explicitly provided options/fields. Each of the approved Q-008/Q-010 resource limits can be overridden individually.
3. **Validate:** reject invalid values before opening sockets/listeners or starting goroutines; do not silently clamp to approved defaults or turn zero into "unbounded".
4. **Snapshot:** retain an immutable effective configuration with that instance. New Sessions inherit their Server/Client's immutable per-session defaults.
5. **Concurrency:** effective config can be read safely from multiple goroutines; never hold a shared Config mutex across user callbacks.
6. **Observability:** make effective settings inspectable/loggable without exposing mutable internal pointers, credentials, or peer identity material unnecessarily.
7. **Reconfiguration:** **no live hot-reload requirement has been approved**. Changes to a new construction/configuration apply to new instances; modifying caller-owned data later must not change live instance behavior. If runtime updates become necessary, that needs a separate explicit design.

## 6. Interaction with lifecycle and admission

- Failures of config validation occur before opening network resources; they are **not** Send Queue Full or incoming overload events.
- Outgoing local/global quota rejection leaves Session open and has no orphaned Pending Request. Incoming local/global quota rejection closes only the affected Session (Q-009/Q-010).
- At a bound-Session limit, a new Bind is rejected without evicting existing sessions; Pre-Bind sockets above their cap close before full Session allocation.
- Auto Reconnect remains an explicit Client policy, not automatically enabled by Enquire Link timeout or queue limits.
- An application callback currently blocked in user code cannot be forcibly terminated. The exact sequencing of Close callback versus active user callbacks stays open for **API-001**, not resolved by Config design.

## 7. Proposed acceptance checks (NOT executed)

- No options: all approved defaults load, including **30s** idle and **10s** response timeout.
- Explicit idle interval `0`: outgoing Enquire Link disabled; incoming Enquire Link responses still work.
- Explicit positive idle duration: accepted and honored; negative rejected.
- Independently override each of the six Q-010 defaults and the Q-008 outgoing cap; omitted values remain unchanged.
- Explicit zero and negative queue caps, session counts or mandatory timeouts: reject deterministically; no network side effects.
- Concurrent Client/Server creation from the same option/config inputs never shares mutable effective state.
- Mutating source Config inputs after construction cannot change live Session behavior.
- Repeated startup/shutdown and invalid config paths do not leak goroutines/sockets.
- Cross-language Pinex tests verify configuration affects only local behavior, not PDU bytes or command status.

## 8. Open decisions / next steps

| ID | Item | Recommendation | Status |
|---|---|---|---|
| API-004 | Public Config representation | **Option A: typed Functional Options, independently applied over complete defaults, validated before network start and frozen as an effective per-instance snapshot** | **CLOSED — owner approved 2026-11-16** |
| API-001 | Exact Client/Server/Session interface; callbacks, response ownership, Send signatures | Minimal Go-native API, no app-level framework | OPEN |
| API-002 | Remaining timeout/reconnect/Stop defaults, callback termination contract | Stay aligned with Pinex wire and explicitly approve Go defaults | OPEN |
| API-003 | Go package/module path after brand selection | `neta-go` brand approved; actual repo/module rename NOT authorized | OPEN |
| GO-PINEX-001 | Exact C++ Pinex develop HEAD & golden frames | Pin baseline before coding/interoperability tests | OPEN |

**Important:** Functional Options is an **approved design decision** (API-004 closed); it is **not permission to implement**, and it does not freeze the final exported identifier names or signatures. No code/tests/repository or module rename until explicit authorization.

## 9. Owner's final API-004 decision (2026-11-16)

**APPROVED OPTION A.** Client and Server configuration uses **Go typed Functional Options**. Constructors build approved defaults first, apply only explicit options, validate the resulting effective configuration **before Dial/Listen**, and retain a private immutable snapshot for the instance and inherited Session settings. Options independently override every approved numeric cap/deadline; an omitted idle interval remains **30s**, while explicit `WithEnquireLinkIdleInterval(0)` (illustrative name only) **disables only automatic Enquire Link**. Explicit invalid zero/negative resource limits or mandatory positive timeouts fail validation; none silently become unlimited. Client-only and Server-only option domains should be separated. No hot reload is implied.

**Deferred to API-001/API-002:** precise constructor signatures, option identifier spelling, treatment of bind/readyness and reconnection, and callback completion/teardown semantics. **No implementation or tests authorized.**


## 10. WORKER-008 / WORKER-009 — resource accounting and default resolution handoff (2026-11-17)

WORKER-008 Option A is owner approved: separate per-Session and aggregate-owner retained-payload byte ledgers for Application and Completion. Queue byte/count limits are separate admission dimensions, not additive physical-memory totals. Reconnect generations share Client owner accounting; started callbacks retain their charges until actual return.

WORKER-009 is open. The recommended default model is to resolve only owner worker ceilings from GOMAXPROCS at Client/Server configuration construction, with finite caps, while keeping queue/count/Request/Event/retained-byte quotas as fixed finite defaults. The resolved config remains immutable; no live adaptation or hot reload. The candidate numeric matrix is in [Worker Capacity & Resource Accounting Design](NETA_GO_WORKER_CAPACITY_RESOURCE_ACCOUNTING_DESIGN_2026-11-16.md) §17. The current owner decision is WORKER-009A: CPU-derived versus fixed worker defaults.

The exact public option names remain API-001 work. Omitted values use defaults; explicit zero is invalid for required positive resource limits, except the already-approved EnquireLinkIdleInterval=0 disable behavior. No implementation or tests authorized.
