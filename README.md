# neta-go

A high-performance, Pinex-compatible TCP messaging library written in Go.

## Project status

The implementation is tracked phase by phase. The root task tracker is the source of truth for completed work, open tasks, acceptance criteria and CI evidence.

**[View implementation tasks and progress →](TASKS.md)**

Current phase: **Phase 6 — Client/Server orchestration (COMPLETE / CI PASS)**. Client Connect/Bind, generation fencing, reconnect, Server lifecycle/admission and Client shutdown are implemented and verified. Phase 7 live TCP interoperability and Phases 8–9 remain gated pending separate owner authorization.

Architecture decisions and detailed design records are maintained under [`.codex/`](.codex/).
