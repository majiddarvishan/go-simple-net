#!/usr/bin/env bash
# Correctness checks shared by local Makefile and GitHub Actions.
# This script runs as a child process and does not change the caller's shell.
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "$BASH_SOURCE")/.." && pwd)"
cd "$ROOT_DIR"

GO_BIN="$(printenv GO 2>/dev/null || true)"
if [[ -z "$GO_BIN" ]]; then GO_BIN=go; fi
MODE=verify
if [[ "$#" -gt 0 ]]; then MODE="$1"; fi
ARTIFACT_BASE="$(printenv ARTIFACT_DIR 2>/dev/null || true)"
if [[ -z "$ARTIFACT_BASE" ]]; then ARTIFACT_BASE="$ROOT_DIR/artifacts/ci"; fi
if [[ "$ARTIFACT_BASE" != /* ]]; then ARTIFACT_BASE="$ROOT_DIR/$ARTIFACT_BASE"; fi

command -v "$GO_BIN" >/dev/null 2>&1 || {
    echo "ERROR: Go executable not found: $GO_BIN" >&2
    exit 127
}

RUN_STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
SOURCE_SHA="$(git rev-parse --short=12 HEAD)"
RUN_DIR="$ARTIFACT_BASE/$RUN_STAMP-$SOURCE_SHA-$MODE-$$"
mkdir -p "$RUN_DIR"
LOG_FILE="$RUN_DIR/run.log"

exec > >(tee -a "$LOG_FILE") 2>&1

finish() {
    local rc=$?
    trap - EXIT
    {
        if (( rc == 0 )); then echo "status=PASS"; else echo "status=FAIL"; fi
        echo "mode=$MODE"
        echo "source_sha=$(git rev-parse HEAD)"
        echo "exit_code=$rc"
        echo "finished_utc=$(date -u +%FT%TZ)"
    } > "$RUN_DIR/status.txt"
    echo
    if (( rc == 0 )); then echo "Correctness result: PASS"; else echo "Correctness result: FAIL"; fi
    echo "Artifacts: $RUN_DIR"
    exit "$rc"
}
trap finish EXIT

capture_metadata() {
    {
        echo "started_utc=$(date -u +%FT%TZ)"
        echo "source_sha=$(git rev-parse HEAD)"
        echo "branch=$(git branch --show-current)"
        echo "go_version=$("$GO_BIN" version)"
        echo "GOOS=$("$GO_BIN" env GOOS)"
        echo "GOARCH=$("$GO_BIN" env GOARCH)"
        echo "GOMAXPROCS=$(printenv GOMAXPROCS 2>/dev/null || echo default)"
        echo "GO_ENV:"
        "$GO_BIN" env
        echo "SYSTEM:"
        uname -a
        echo "WORKTREE:"
        git status --short --branch
        git diff --binary HEAD > "$RUN_DIR/worktree.patch"
    } > "$RUN_DIR/environment.txt" 2>&1
}

format_check() {
    echo
    echo "========== GOFMT =========="
    local unformatted
    unformatted="$(find . -type f -name '*.go' \
        -not -path './.git/*' \
        -not -path './vendor/*' \
        -not -path './artifacts/*' \
        -exec gofmt -l {} +)"
    if [[ -n "$unformatted" ]]; then
        echo "$unformatted"
        while IFS= read -r file; do
            [[ -n "$file" ]] && gofmt -d "$file"
        done <<< "$unformatted"
        return 1
    fi
    echo "PASS"
}

vet_check() {
    echo
    echo "========== GO VET =========="
    "$GO_BIN" vet ./...
}

unit_check() {
    echo
    echo "========== UNIT TESTS (UNCACHED) =========="
    "$GO_BIN" test -count=1 ./...
}

race_check() {
    echo
    echo "========== RACE TESTS (UNCACHED) =========="
    "$GO_BIN" test -race -count=1 ./...
}

capture_metadata

case "$MODE" in
    format) format_check ;;
    vet) vet_check ;;
    test) unit_check ;;
    race) race_check ;;
    check)
        format_check
        vet_check
        unit_check
        ;;
    verify)
        format_check
        vet_check
        unit_check
        race_check
        ;;
    *)
        echo "Usage: bash scripts/ci-check.sh {format|vet|test|race|check|verify}"
        exit 2
        ;;
esac

echo
echo "========== ALL REQUESTED CHECKS PASSED =========="
