# neta-go — DEV-01 production/API completeness audit
Date: 2026-11-19
Status: DEV-01 STATIC REVIEW COMPLETE; Phase D continues at DEV-02
Repository: github.com/majiddarvishan/neta-go
Feature inspected: `a58d2bb50fe5f8d09d1a700f50f5b37d512b5ed1`
Protected main inspected: `24b7c5f6eb4e001f30dd8a564a337ca1f506eac8`
Source-anchor (unchanged): `9b7b2318f73d173a99d22489042f84f8ce58b745`
Decision authority: `.codex/DECISION_LOG.md` §§75–78; approved QA contracts; owner development-first request.

## 1. Verified comparison and review scope

GitHub compare `main...feature/qa004-qa005-qa007-implementation-2026-11-18` returned **ahead 119, behind 0** at the stated refs. The feature has 13 modified/added *production* Go files vs main:

`config.go`, `diagnostics.go` (new), `errors.go`, `options.go`, `resource_charges.go`, `resource_ledger.go`, `server_lifecycle.go`, `session.go`, `session_control.go`, `session_internal.go`, `session_transport.go`, `types.go`, `worker_scheduler.go`.

Reviewed public-facing `docs/API.md`, `docs/CONFIGURATION.md`, `docs/PROTOCOL.md`, `README.md`, `examples/client/main.go`, `examples/server/main.go`, `scripts/example-smoke.sh`; production paths for Session admission/Completion, worker dispatch/cancel, callback panic capture, Client reconnect generations, Server Bind admission/Stop and protocol options. This is a **static source review**, not a fresh race/performance qualification or proof no bugs exist.

## 2. Production code status and actionable findings

| Area | Source-grounded observation | Classification and concrete next action |
| --- | --- | --- |
| QA-007: forced-close Completion | `session_internal.go` retains accepted Request terminal tasks, cancels only unstarted Application work; `session.go` reserves Request credits; `worker_scheduler.go` isolates Application/Completion/OnClose domains. | Implementation already in feature. Historic focused Go/race CI cited in `docs/LIFECYCLE.md`; **no new production edit justified by this audit**. Recheck during Phase Q. |
| QA-007: diagnostics and OnClose | `diagnostics.go` exposes typed `Session.Diagnostics()` and panic-safe snapshots; `resource_ledger.go`/`server_lifecycle.go` reserve OnClose credit at admission; callback releases credit. | Implementation present. No newly confirmed code bug; targeted example demonstration EX-C05/EX-S05 and Q verification remain. |
| QA-004: Deadline scan | `session_control.go` iterates `s.pending` for expired Request settlement and next deadline; no heap/wheel replacement. Deadline observer is internal and opt-in. | Linear scan matches approved policy; heavy P99/delay baseline and absolute limit are **QUALIFICATION**, not new production implementation. |
| QA-005: retained memory | Existing test-only `scripts/qa005_owned_buffers_overlay.py` and `qa005_qualification_benchmark_test.go` experiments do not establish canonical `retained-attributed-B`; source-ledger counts are distinct from allocated/owned backing. | **DEV-02** must produce bounded F1–F5/M1/D1 coverage matrix and honestly classify measurable, excluded and unproved cases. Phase Q owns peak/sampler/overhead qualification. No implicit production hot-path instrumentation. |
| Protocol & public options | `config.go` defaults V2; `options.go` exposes Client/Server/Session Functional Options; `docs/PROTOCOL.md` states V1=19+NUL and V2=20/no NUL with fixed 10-byte header. | Sufficient public API surface for 10 requested examples; **no new public API required for examples**. PV-02/PV-04/PV-05 remain owner-gated. |
| Existing example compatibility | Client flags `-addr`, `-system-id`; Server flag `-listen`. Smoke expects `listening on ...;`, `Completion sequence=`, `one-way sequence=`, `one-way event sequence=`, `Server stopped`. | **DEV-03** freezes commands/logs; EX tasks introduce separate directories rather than rewriting these two programs. |
| Public-doc drift | `docs/API.md` lists Session methods without `Diagnostics()` and omits `Server.AcceptDone()/AcceptErr()`; `docs/CONFIGURATION.md` omits exported `WithCallbackPanicRingCapacity` and `WithCallbackPanicByteBudget`. Documentation still describes Client/Server OnClose as best-effort although feature has pre-admission credit guarantee. | **DEV-05 documentation corrections** alongside guide. Not a missing production symbol or authorization to alter runtime policy. |

**Confirmed new production-code defects requiring an independent DEV implementation ticket: none from this audit.** This is a bounded result of inspecting the requested files; discovery through focused examples or subsequent validation must be logged as a new in-scope defect, not silently folded into Performance Qualification. No unreviewed runtime behavior is declared fully correct.

## 3. Remaining work classified without conflation

- **DEVELOPMENT (authorized, OPEN after this audit):** DEV-02 through DEV-06; EX-C01..EX-C05; EX-S01..EX-S05. Scope, acceptance and order: `.codex/NETA_GO_DEVELOPMENT_FIRST_EXAMPLES_BACKLOG_2026-11-19.md`. Existing two executable examples remain; target twelve.
- **QUALIFICATION (deferred; not implementation blockers for demos):** QA-004 representative Pending P99 and absolute threshold, QA-005 eight Core/twenty-four Full workloads, ten paired/95% comparisons, MEM-004 owned attribution with temporal peak validity, MEM-005 measurement overhead, 30-minute soak, final race/interop/fuzz/stress. Missing proof must stay INCONCLUSIVE.
- **OWNER DECISION (BLOCKED, outside Phase D):** previously unapproved protocol capability/negotiation PV-02/PV-04/PV-05, numerical reference-host/baseline/absolute SLA and mandatory Performance Gate activation, required-check naming/settings and amended release note approvals.
- **RELEASE (separately blocked):** PR, Merge, Tag and GitHub Release. None authorized.
- **NO ACTION:** protected `main`, `QA004-QA005-SOURCE-V1` immutable source SHA, historical benchmark baselines and gate setting; leave untouched.

## 4. Evidence and verification limitations

Verification for DEV-01 is GitHub branch and compare response plus direct inspection of the named source/docs at the identified feature commit. **No source file was changed; no Go command, smoke, race suite, benchmark or fresh CI was executed for this documentation-only audit.** Historic green CI remains historical; it is not new Phase-Q sign-off. DEV-01 closes a concrete *inventory/review* task only. All ten NEW examples and five remaining DEV tasks stay unchecked, with DEV-02 next.

## 5. Precise next action

Conduct **DEV-02** category-by-category Hybrid-B F1–F5/M1/D1 coverage assessment. Distinguish observed owned backing capacity and test-only logical structs from policy charges, allocator graph and a simultaneous in-workload retained peak. Close bounded implementation accounting honestly without fabricating unknown bytes or changing a numerical gate. Continue to DEV-03 and requested examples afterward.
