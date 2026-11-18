# QA-009 — Real Mutation Fuzzing and Live C++ TCP Continuous Verification

**Date:** 2026-11-18  
**Public repository:** `github.com/majiddarvishan/neta-go`  
**Change:** [PR #20](https://github.com/majiddarvishan/neta-go/pull/20)

## The public fuzz campaign

`.github/workflows/wire-fuzz.yml` installs Go 1.23.x and uses `go test -fuzz` (mutation engine, not seed-only unit testing). Each target is a separate package invocation to satisfy the single-target Go fuzz contract:

1. `FuzzParseHeader`
2. `FuzzDecodeFrame`
3. `FuzzDecodeBindRequest` (legacy V1)
4. `FuzzDecodeBindResponse` (legacy V1)
5. `FuzzDecodeBindRequestV2` (new)
6. `FuzzDecodeBindResponseV2` (new)
7. `FuzzDecodeStreamRequest`

The V2-specific targets seed a valid 20-byte SystemID, legacy NUL-terminated V1 input and malformed/truncated cases. They check accepted BindType, maximum V2 SystemID size, lack of NUL and correct command for both Request and Response, including the supported empty-Response-body case.

**Execution policy:** 3 seconds per target in public PRs, 12 seconds per target on weekly Monday 03:23 UTC and manual dispatch; `-parallel=2`. This is a bounded mutation campaign; it is **not** an exhaustive proof against every malformed frame.

**Verified first PR run:** [44266748134](https://github.com/majiddarvishan/neta-go/actions/runs/44266748134) **SUCCESS** with 7 target-level PASS results and 1,618,844 mutation executions cumulatively (234,674 + 230,265 + 231,481 + 233,355 + 230,084 + 239,380 + 219,605). No crash or fuzz counterexample was reported. All targets run on the same CI runner but in separate Go invocations. Fuzz-discovered interesting inputs are not presented as committed permanent corpus; crashes in any later run fail CI and need to be retained as regression seeds.

## Real TCP across languages

The separate **private** C++ 1.5.0 repository runs `.github/workflows/neta-go-live-interop.yml` (merged in [pinex PR #29](https://github.com/majiddarvishan/pinex/pull/29)), with weekly, manual, private PR/push triggers. It compiles the original pinned C++ peer and checks both TCP directions in ordinary and Go Race mode. [Private postmerge run 44260379077](https://github.com/majiddarvishan/pinex/actions/runs/44260379077) was **SUCCESS** on public neta-go `0824a70127594df72f78d1048cc360e33f57bd5b`; see [QA-001 evidence](NETA_GO_QA001_REAL_TCP_VERIFICATION_2026-11-18.md).

**The private C++ workflow is not a required status context on each public neta-go PR.** Public PRs always retain the independent golden wire workflow and now additionally run bounded parser mutation fuzz.

## Scope and follow-up

QA-009 is **closed for availability and actual execution of scheduled/authorized mutation fuzz and live C++ verification**. Longer fuzz periods, cross-version negotiation, real-world network security, and performance SLAs are separate qualification efforts. No runtime implementation, wire bytes, public V2 default, tag, Release or release approval changed in this task. Preserve failure inputs as permanent seeds if a future fuzz run finds a bug.
