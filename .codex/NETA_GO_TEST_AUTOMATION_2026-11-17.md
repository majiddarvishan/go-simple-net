# neta-go Test and Performance Automation

**Date:** 2026-11-17  
**Repository:** github.com/majiddarvishan/neta-go  
**Branch:** main  
**Purpose:** One repeatable local entry point and the same scripts in GitHub Actions.

## User-facing commands

| Command | Meaning |
|---|---|
| `make check` | gofmt, go vet and uncached unit tests |
| `make race` | uncached race detector |
| `make verify` | all correctness checks: format, vet, unit, race |
| `make perf-smoke` | short Phase 8 benchmark smoke |
| `make soak` | mixed-payload memory soak (300 seconds by default) |
| `make benchmark` | full benchmark matrix, 1–512 Session scaling, soak and profiles |

The same scripts are used by GitHub Actions. CI pins Go `1.23.x`; local runs record the actual Go version. For comparable performance numbers, use the same Go toolchain and equivalent hardware/runner conditions.

## Safe execution model

- Each script runs as a child Bash process. It does not source settings into, or change options in, the user's interactive shell.
- Run one command such as `make verify`; do not paste `set -e` into an interactive terminal.
- Every run writes a unique directory under `artifacts/ci/` or `artifacts/performance/`.
- Each run records the source SHA, branch, worktree status, Go version, GOOS/GOARCH and environment metadata.
- Standard output and errors are captured in `run.log`; a `status.txt` records PASS/FAIL and the exit code.
- Performance runs preserve benchmark output, timing information, pprof profiles and pprof summaries.
- `artifacts/` is ignored by Git and is uploaded by CI as a workflow artifact.

## Reproducibility

- Correctness tests use `-count=1` so Go's test cache cannot turn a repeat into a no-op.
- Race runs remain separate from performance measurements.
- Memory soak duration is configurable through `NETA_SOAK_SECONDS` and is constrained to 300–600 seconds.
- `ARTIFACT_DIR` can override the artifact root. The script creates a unique run subdirectory beneath it.
- Benchmark commands, source SHA and environment details are retained with each result.

## Current validation status

Automation files are committed with the Phase 8 work. GitHub Actions execution is the validation gate. Do not call P8-01 complete or claim a performance baseline until the full Phase 8 workflow succeeds and its artifacts are reviewed.
