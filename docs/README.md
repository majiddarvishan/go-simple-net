# neta-go documentation

Use this directory for the **current public contracts** and release/qualification status. Engineering snapshots and historical decisions are retained separately under [`.codex/`](../.codex/README.md).

## Use the library

- [Quick start and overview](../README.md#quick-start)
- [Public API](API.md) — Client, Server, Session, callbacks and diagnostics
- [Configuration and defaults](CONFIGURATION.md) — typed Functional Options, worker, timeout and resource limits
- [Lifecycle, ownership, callback and error semantics](LIFECYCLE.md) — including the merged QA-007 behavior
- [Wire protocol and V1/V2 Bind](PROTOCOL.md) — fixed Header, version differences and limits
- [Twelve runnable Client/Server examples](../examples/README.md) — CLI, JSON, concurrency, reconnect, V1 and diagnostics
- [Toolchain and platform support](SUPPORT.md) — verified versus unverified targets

## Qualification and release

- [Current release readiness](RELEASE_READINESS.md) — **the authoritative release-gate summary**, including the reviewed `main` SHA, completed Development, open QA gates, required owner decisions and publication checks.
- [Performance qualification method and historical evidence](PERFORMANCE_QUALIFICATION.md) — QA-004 P99, QA-005 Core/Full matrix, A/A versus cross-version comparisons, retained-memory exclusions and sampling requirements.
- [Unreleased changelog](../CHANGELOG.md) — draft release notes; owner approval of the current wording is separate from any release authorization.

**At the 2026-11-20 documentation review:** Development is merged via [PR #24](https://github.com/majiddarvishan/neta-go/pull/24) (16/16 scoped tasks done); **QA-007 is implemented/merged**, while QA-004/005 and MEM-004/005/006 reference qualification are still open; the mandatory Performance Gate is **DISABLED** and `v1.0.0` is **not authorized** for publication. Check [Release readiness](RELEASE_READINESS.md) for the actual evidence and remaining preflight.

## Engineering records (dated snapshots)

- [Task tracker](../.codex/TASKS.md)
- [Development closeout (historical feature-branch snapshot)](../.codex/NETA_GO_DEV_READY_FOR_QUALIFICATION_2026-11-19.md)
- [Decisions and handoff history](../.codex/README.md)

Older documents may say “feature-only,” “not merged,” or “PR not authorized” because those were correct **when written**. Do not treat those dated restrictions as an up-to-date post-merge release status; likewise, a merge does **not** authorize benchmarks, gate activation, tags or releases.
