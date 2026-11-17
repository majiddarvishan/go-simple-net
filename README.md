# neta-go

A high-performance, Pinex-compatible TCP messaging library written in Go.

## Project status

The implementation is tracked phase by phase. The root task tracker is the source of truth for completed work, open tasks, acceptance criteria and CI evidence.

**[View implementation tasks and progress →](TASKS.md)**

Current phase: **Phase 6 — Client/Server orchestration (authorized / in progress)**. Server lifecycle/admission and Client CurrentSession/Stop are CI-green. Client Connect/Bind and reconnect are implemented against the owner-approved Bind identity contract and are awaiting the current CI verification. Phase 7+ remain gated.

Architecture decisions and detailed design records are maintained under [`.codex/`](.codex/).
