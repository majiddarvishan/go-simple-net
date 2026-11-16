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
| RequestTimeout | **30s (API-002-01 approved)** | Client + Server owner; inherited by Sessions | positive finite duration; starts after successful local admission; zero invalid |
| FrameAssemblyTimeout | **30s (API-002-02 approved)** | Client + Server owner; inherited by Sessions | positive finite duration; absolute from first Frame Header byte on Bound Session; zero invalid |
| ConnectTimeout | **30s (API-002-03 approved)** | Client Dial attempt only | positive finite duration; absolute from Dial start; includes DNS + TCP; earlier caller Context wins; zero invalid |

Additional C++-observed reference values (Bind 30s, reconnect delay 5s, Unbind 10s) are documented elsewhere; **they are not promoted here to approved Go defaults**. ConnectTimeout=30s is separately owner-approved for Go under API-002-03. RequestTimeout=30s and FrameAssemblyTimeout=30s are separately owner-approved Go defaults under API-002-01/02; C++ Pinex does not define a RequestTimeout default and has no equivalent bound-session Frame Assembly deadline. The max protocol PDU length remains separate from all queue byte limits and must retain Pinex C++ 1.5.0 wire compatibility.

### Cross-field validation — draft

- Validate all settings **after** overrides are applied, not each default independently, to avoid false conflicts during construction.
- Prevent integer overflow when converting MiB to byte counts or accumulating queued frame charges. Include the 10-byte Pinex frame header where the applicable byte budget specifies full serialized frames.
- Do not require `MaxAggregateQueuedFrameBytes >= MaxBoundSessions * MaxQueuedIncomingBytes`: that would effectively negate the approved dynamic global budget. Permitting a global cap smaller than the theoretical sum is intentional.
- A configured incoming or outgoing queue bound **does not increase maximum PDU size**. Oversized PDU handling stays with protocol validation.
- For a frame larger than the configured queue capacity, apply the approved admission result (outgoing Queue Full; incoming close offender) without unbounded temporary allocation.
- Any user-supplied byte cap smaller than the minimum supported frame size should be rejected during configuration validation where practical. The minimum valid bound and accounting for an executing callback are **technical details for API-001/GO-PINEX-010**.
- The 4 KiB Control allowance must remain separate/bounded and preserve single-writer semantics and Pinex Unbind ordering/10s deadline. Do not interpret its presence as a priority queue or permission to overtake application frames.
- Include a clear field-specific error category (e.g. `invalid_config`) for invalid overrides. Exact exported error names are not decided.

### WORKER-009B2 approved count/event defaults (2026-11-17)

The following six count limits are part of the effective Client/Server configuration. Each is an independent typed Functional Option over its approved default; none is preallocated. Exact exported option names remain API-001 work.

| Semantic setting | Approved default | Scope |
|---|---:|---|
| OnClose event credits | 1,024 | Per Client/Server owner; queued + running |
| Incoming Application frames | 256 | Per Session; Reader reservation + waiting Request callbacks |
| Outgoing Application frames | 256 | Per Session; includes Writer-owned frame until full Write/discard |
| Aggregate queued Application frames | 65,536 | Per Server; incoming + outgoing |
| Outstanding Requests | 128 | Per Session; through Completion callback return/cancellation |
| Aggregate outstanding Requests | 32,768 | Per Client/Server owner; shared across Sessions/reconnect generations |

Incoming count saturation closes only the offending Session; outgoing count saturation returns Queue Full and keeps the Session open. Server aggregate frame-count saturation follows the same direction-specific behavior. OnClose saturation permanently skips the new event. Request-credit saturation rejects the new callback-based SendRequest before enqueue/transmission with a typed capacity error and leaves the Session open. SendNoWait and SendResponse do not consume Request credits. These limits are distinct from worker permits, retained-payload bytes and queue-byte ledgers; no physical buffer is double-counted. Explicit zero/negative values are invalid. The aggregate frame-count and Request-credit defaults are intentionally lower than the sum of their theoretical per-Session ceilings; cross-field validation must not force aggregate limits to equal or exceed those products.

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
- Independently override each Q-008/Q-010 setting and each WORKER-009B2 count limit; omitted values remain unchanged.
- Verify each B2 count limit's exact-at-limit and non-fitting behavior, including direction-specific Session/Server saturation and no leaked Request credits.
- Verify OnClose queued+running saturation skips permanently, and Request-credit saturation rejects before enqueue/transmission without closing Session.
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

**WORKER-008 Completion-byte exhaustion is also CLOSED / OWNER APPROVED, OPTION A (2026-11-17):** if a valid matching Response cannot reserve its declared full frame against the Session or owner Completion retained-byte budget, do not allocate its Body. Drain exactly the Body through a small fixed-size scratch buffer; if successful and no competing terminal event has won, invoke the existing Completion callback once with no Response payload and a typed terminal Resource Capacity error, keeping the Session open. Keep the existing Request credit through callback return/cancellation; do not create another Completion event credit. A drain/transport failure closes the Session under the existing transport-error/Forced Close policy. Malformed or over-maximum frames follow protocol-error handling. Exact read-timeout behavior remains API-002 work.

WORKER-009A, WORKER-009B1, WORKER-009B2 and WORKER-009B3 are CLOSED / OWNER APPROVED. WORKER-009A resolves owner worker ceilings once from GOMAXPROCS. WORKER-009B1 sets Application/Completion concurrency=1 per Session and independent 30s idle grace for Application, Completion and OnClose. WORKER-009B2 approves OnClose event credits=1,024 per owner; incoming/outgoing Application frame counts=256 per Session; aggregate queued Application frame count=65,536 per Server; outstanding Request credits=128 per Session and 32,768 per Client/Server owner. WORKER-009B3 approves retained Application bytes=16 MiB per Session and 512 MiB per owner; retained Completion bytes=16 MiB per Session and 256 MiB per owner. All are independently configurable finite defaults, not preallocations or hard maxima. Saturation and ledger semantics are specified above and in Worker Capacity Design §17. WORKER-009C hard maxima, valid ranges, cross-field rules and semantic Functional-Option mapping are CLOSED / OWNER APPROVED / OPTION A.

The exact public option names remain API-001 work. Omitted values use defaults; explicit zero is invalid for required positive resource limits, except the already-approved EnquireLinkIdleInterval=0 disable behavior. No implementation or tests authorized.


## 11. WORKER-009B3 approval and WORKER-009C validation proposal (2026-11-17)

### 11.1 WORKER-009B3 — approved retained-payload byte defaults

The owner approved Option A. The four values are official V1 defaults:

| Semantic setting | Approved default | Scope |
|---|---:|---|
| Retained Application bytes | 16 MiB | Per Session |
| Aggregate retained Application bytes | 512 MiB | Per Client/Server owner |
| Retained Completion bytes | 16 MiB | Per Session |
| Aggregate retained Completion bytes | 256 MiB | Per Client/Server owner |

Each value is an independent typed Functional Option over a positive finite default. Client owner budgets are shared across reconnect generations; Server owner budgets cover all Sessions of that Server. These are lazy admission ceilings, not preallocation or process RSS limits. Queue byte/count limits remain independent admission sub-limits on the same frame and are not summed as additional physical memory. Started callbacks retain their byte charge until actual callback return; user-retained inbound payload after callback return is outside the library-owned ledger. The exact Pinex maximum frame remains a separate protocol-compatibility decision.

### 11.2 WORKER-009C — approved hard maxima (CLOSED / OWNER APPROVED / OPTION A, 2026-11-17)

The owner approved Option A. The approved validation ranges and maxima are:

| Semantic setting | Approved default | Approved allowed range / hard maximum |
|---|---:|---:|
| Application active workers per owner | CPU-derived, cap 128 | 1..512 |
| Completion active workers per owner | CPU-derived, cap 64 | 1..256 |
| OnClose active workers per owner | CPU-derived, cap 32 | 1..128 |
| Application callback concurrency per Session | 1 | 1..32 |
| Completion callback concurrency per Session | 1 | 1..32 |
| Application/Completion/OnClose worker idle grace | 30s | 1ms..10m |
| OnClose event credits per owner | 1,024 | 1..4,096 |
| Incoming Application frames per Session | 256 | 1..1,024 |
| Outgoing Application frames per Session | 256 | 1..1,024 |
| Aggregate queued Application frames per Server | 65,536 | 1..262,144 |
| Outstanding Request credits per Session | 128 | 1..512 |
| Aggregate outstanding Request credits per owner | 32,768 | 1..131,072 |
| Retained Application bytes per Session | 16 MiB | 10 bytes..64 MiB |
| Aggregate retained Application bytes per owner | 512 MiB | 10 bytes..2 GiB |
| Retained Completion bytes per Session | 16 MiB | 10 bytes..64 MiB |
| Aggregate retained Completion bytes per owner | 256 MiB | 10 bytes..1 GiB |

Binding validation invariants:
- Validate the completed effective configuration only after all Functional Options have been applied, and before socket creation, goroutine startup, or other active resource creation.
- Count and non-retained-byte limits have minimum 1; retained-byte limits have minimum 10 bytes. Worker idle grace is 1ms..10m. Explicit zero is invalid except `EnquireLinkIdleInterval=0`, which disables automatic outgoing Enquire Link only.
- Owner and per-Session limits are independent. Aggregate capacity may be below a per-Session ceiling or below the theoretical sum across Sessions; admission must satisfy all applicable limits.
- Per-Session Application/Completion concurrency is intersected with the owner/class active-worker ceiling at dispatch; no cross-field rejection is applied solely because the Session value is larger.
- Queue bytes/counts, retained bytes, Request credits, OnClose credits and worker permits are independent ledgers; do not double-count a payload during ownership transitions.
- Byte conversions and counter arithmetic are overflow/underflow checked. Owner caps are per Client/Server instance, not process-wide.
- A 10-byte retained-byte minimum admits only a header-sized frame. The 1 MiB total-frame maximum is source-verified against pinned Pinex C++ 1.5.0 and is separate from WORKER-009C; cross-language confirmation remains pending.

### 11.3 Approved Functional-Option mapping

Use one typed, independent Functional Option per semantic setting. Omitted options preserve defaults; explicit options override only their own setting. Owner worker ceilings, OnClose credits, owner Request credits and owner retained-byte ceilings follow Client/Server ownership; aggregate queued Application frame count is Server-only; per-Session concurrency and per-Session frame/Request/retained-byte limits attach to Session configuration/inheritance; worker idle grace is independently configurable per class. Do not introduce a generic map, mutable live config or hot reload. Exact exported identifiers and final constructor/Session option placement remain API-001 public-surface work.

**Decision:** `CLOSED / OWNER APPROVED / OPTION A`. Implementation remains NOT AUTHORIZED.


### 11.4 API-002-01 — approved RequestTimeout configuration (CLOSED / OWNER APPROVED / OPTION A, 2026-11-17)

- **Default:** 30 seconds.
- **Scope:** independently configured on each Client or Server owner using a typed Functional Option; accepted Sessions inherit their owner's immutable effective setting. There is no per-call timeout override in V1.
- **Validation:** strictly positive finite duration; explicit zero is invalid and does not disable Request timeout. No additional upper hard maximum has been approved.
- **Start point:** immediately after successful atomic local Request admission (Sequence/Request credit/queue frame+bytes/retained-payload reservation and FIFO insertion). Queue wait, Writer wait/Write and peer response time all consume the same deadline.
- **Timeout before Writer claim:** cancel the queued frame so it cannot be transmitted; release queue and queued-payload accounting exactly once. Deliver one timeout Completion; Request credit and Sequence protection remain until Completion callback return or permitted cancellation.
- **Timeout after Writer claim:** send ownership cannot be revoked. Ignore a later Response for settlement purposes; keep Writer-owned payload memory accounted until Write completion or transport teardown.
- Timeout alone does not close Session. Timeout/Response/Close race has one terminal settlement winner. B (start after full Write) and C (mandatory per-call timeout) were not selected.

API-002-02 Frame Assembly Timeout is CLOSED / OWNER APPROVED / OPTION A; see §11.5. API-002-03 TCP Connect Timeout is the next owner decision. No implementation or tests are authorized.


### 11.5 API-002-02 — approved FrameAssemblyTimeout configuration (CLOSED / OWNER APPROVED / OPTION A, 2026-11-17)

- **Default:** 30 seconds absolute per inbound Frame on a Bound Session.
- **Scope:** independently configured on Client and Server owners using typed Functional Options; accepted Server Sessions inherit the immutable Server effective value.
- **Start/semantics:** starts when Reader first observes the first byte of the current Frame Header; covers remaining Header and full declared Body. Incremental progress does not reset or extend the deadline. Cancel only when the Frame is fully assembled/validated or Session enters terminal close.
- **Expiry:** close only the affected Session under existing terminal/Forced Close policy. No byte skipping, stream resynchronization, or impact on other Sessions.
- **Validation:** positive finite duration; explicit zero invalid. No additional upper hard maximum approved.
- **Resource interaction:** partial Header holds only bounded parser storage. After Header validation, reserve full declared-frame retained bytes and applicable queue/count capacity before Body allocation. Expiry releases partial storage and all applicable reservations exactly once. Completion-capacity scratch-buffer Body drain is subject to the same original deadline; drain expiry closes Session without a Resource Capacity Completion.
- **Pre-Bind:** the approved 30s absolute handshake deadline from TCP acceptance remains independent and unchanged; FrameAssemblyTimeout does not extend it.

No wire change is introduced. Pinex C++ 1.5.0 has no separate equivalent bound-session assembly deadline. API-002-03 TCP Connect Timeout is next. No implementation or tests are authorized.


### 11.6 API-002-03 — approved ConnectTimeout configuration (CLOSED / OWNER APPROVED / OPTION A, 2026-11-17)

- **Default:** 30 seconds absolute per Client Dial attempt.
- **Scope:** Client only; each Dial attempt receives a fresh independent deadline. This is not a whole reconnect-loop deadline.
- **Start/coverage:** starts immediately before Dial and covers DNS/name resolution and TCP establishment. Use a context-aware Dial operation so resolution is also bounded.
- **Context:** caller cancellation always applies. If the caller has an earlier deadline, it takes precedence; otherwise ConnectTimeout supplies the bound.
- **Outcome:** timeout cancels the current attempt and produces no Session, Bind or Session worker/Reader/Writer startup. Close any socket returned after timeout has won. A successful Dial proceeds to the separate BindResponseTimeout phase.
- **Validation:** positive finite duration; explicit zero invalid; no additional upper hard maximum approved.
- This does not enable AutoReconnect or authorize retries. Pinex C++ 1.5.0 also uses a 30s connect reference; this is an explicit Go owner approval, not an inferred default.

API-002-04 Bind response timeout is next. No implementation or tests are authorized.
