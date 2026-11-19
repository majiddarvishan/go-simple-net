# DEV-04 / DEV-05 and EX-C01..05 / EX-S01..05 — completed lightweight acceptance evidence
Date: 2026-11-19
Source Feature after code and verified formatting: `807f60968e2ef48c07c64820ae6972bde82e417b`
Status: **DEV-04 DONE, DEV-05 DONE, ten EX tasks DONE**. Phase D DEV-06 remains next.

## 1. Scope and traceability

| Task | Independent implementation commit(s) | Actual acceptance evidence |
| --- | --- | --- |
| EX-C01 client-config | `89f5706` | strict CLI>JSON>defaults, unknown-key/duration/identity unit tests, config and config-json live pair |
| EX-S01 server-config | `8fb0278` | public Server/Session options, max Sessions/worker/queue/timeouts, strict JSON/CLI, config and config-json live pair |
| EX-C02 client-concurrent | `4b1742e` | capped goroutines, Request credits and correlated Completion, 8-of-8 bounded loopback, no delivery ACK claim |
| EX-S03 server-concurrency | `24da6fb` | bounded owner+Session worker/queue, echo and orderly Stop, live concurrency pair |
| EX-C03 client-reconnect | `d16cb83`, `0cca0d2` harness | intentional abrupt server-process termination, fresh Server on SAME port, observed `OnBound generation-observation=2`, no outgoing messages/replay, 20s cap and cleanup |
| EX-S02 server-bind-validator | `cdbd0dc`, `1999765` | positive allowlist, explicit unknown refusal logs, negative Bind smoke, unit-level concurrent duplicate session rejection |
| EX-C04 client-v1 | `00ef943` | explicit V1, <=19-byte SystemID unit boundary, connected to V1-only Server |
| EX-S04 server-v1 | `cd1d2c9`, `d017b59` | V1 accepted, default V2 rejected as separate non-colliding identity, live positive/negative pairs |
| EX-C05 client-diagnostics | `67bb2ba`, `6ea06c7` | Completion Response/RequestTimeout, local Completion panic and peer-request panic opt-in, typed panic snapshots, bounded Stop |
| EX-S05 server-diagnostics | `bea3532` | safe default, bounded panic ring/text budget, explicit opt-in OnRequest panic recorded by OnClose, no raw graph traversal |
| DEV-04 fast smoke | `7bfc043`, `0cca0d2`, `1d5b71b` | 12 package builds, focused unit, 12 pos/neg paired loopbacks + real Server restart; ephemeral port readiness and process cleanup; not heavy |
| DEV-05 guide | `b5ed030`, `ea0958f` | examples/README scenario matrix/commands/options/shutdown, sample JSON, root README, docs/API and docs/CONFIGURATION corrections |

Additional separate format commits `cd190814` and `807f609` restore exact Go formatting from light CI-tested source. These do not represent extra task completion.

## 2. Evidence from three actual lightweight GitHub Actions runs

Workflow `.github/workflows/examples-dev-light.yml` runs only on explicitly isolated validation branch `ci/neta-go-examples-fast-2026-11-19`. This route is necessary because the normal Feature branch `.github/workflows/qa-final-validation.yml` contains **heavy** benchmark advisory jobs and Phase D forbids intentionally triggering them. Feature branch commits use `[skip ci]`. We did NOT modify the QA Final workflow, reference benchmark or performance gate.

- [Run 44540355649](https://github.com/majiddarvishan/neta-go/actions/runs/44540355649): PASS format, go vet, focused `go test -count=1 ./examples/...`, build all 12 and nine bounded loopback pairs.
- [Run 44541733512](https://github.com/majiddarvishan/neta-go/actions/runs/44541733512): PASS including **actual** second Bind generation after abrupt Server kill/restart; no guessed static wait.
- [Run 44542311954](https://github.com/majiddarvishan/neta-go/actions/runs/44542311954): PASS all actions: gofmt, go vet, build 12, example unit tests (new unknown/duplicate Bind and V1/V2 mismatch), twelve positive/negative Client/Server loopback pairs and real restart+reconnect. Job log explicitly includes:
`PASS: config`, `config-json`, `concurrent`, `bind`, `bind-reject`, `reconnect`, `v1`, `v2-reject`, `diagnostics`, `timeout`, `panic`, `server-panic`, and `PASS: real reconnect after abrupt Server loss and new Server on same port`.
- CI formatting is committed on the disposable validation branch and the formatted Go files were copied back to Feature via separate GitHub commits. At Feature SHA `807f60968e2ef48c07c64820ae6972bde82e417b` and validation SHA `af013c9c1fefd4fbab937d8e9178f00dd9d9cd0a`, **all 71 relevant root production Go, 12 examples' Go files, go.mod and `scripts/examples-dev-smoke.sh` have IDENTICAL Git blob SHA** (recursive tree comparison). Thus tests cover the exact source content now on Feature, even though their commit SHAs differ due docs and isolated CI marker.

## 3. No false expansion of acceptance

- These were Linux/amd64 Go 1.23.x focused commands; did **not** run `make verify`, -race full matrix, C++ wire suite anew, fuzz/stress, 30-min soak, 8/24 workload qualification or 10 paired performance comparisons.
- The original `scripts/example-smoke.sh` and basic Client/Server source remain unchanged. Historic prior Go verify+paired smoke [run 44529571646](https://github.com/majiddarvishan/neta-go/actions/runs/44529571646) is only historical coverage for unchanged basic runtime.
- No production API/Wire change, new dependency or required config. New code uses public Functional Options, Go stdlib and opt-in failure scenarios only.
- GitHub action automatically writes gofmt to the **validation branch only**. No PR, Merge, Tag, Release, baseline update or gate activation.
- Canonical MEM-004 `retained-attributed-B` remains PARTIAL_INCONCLUSIVE despite DEV-02 documentation closure. This tool/result cannot be substituted for a Performance Qualification metric.

## 4. Next

Prepare DEV-06 `DEV_READY_FOR_QUALIFICATION` report: reconcile all 16 authorized task statuses with evidence, enumerate separately owner-blocked decisions, Phase Q test matrix and Phase R dependencies. Do not run the heavy matrix without new owner permission.
