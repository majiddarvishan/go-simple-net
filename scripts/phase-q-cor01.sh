#!/usr/bin/env bash
# Q-COR-01: exact released v1.0.0 correctness qualification (no benchmarks).
# Execute this harness OUTSIDE the immutable release worktree.
set -Eeuo pipefail
umask 077

if (( $# != 2 )); then
  echo "Usage: bash scripts/phase-q-cor01.sh <v1.0.0-worktree> <artifact-dir>" >&2
  exit 2
fi
ROOT="$(cd -- "$1" && pwd)"
mkdir -p -- "$2"
OUT="$(cd -- "$2" && pwd)"
GO_BIN="${GO:-go}"
EXPECTED="d8437f36ec812a596bada1398c7af9b530a18118"
COMPLETED=0
ACTIVE=""
BUILD_DIR=""
export ARTIFACT_DIR

if [[ ! -d "$ROOT/.git" && ! -f "$ROOT/.git" ]]; then
  echo "Source argument is not a Git worktree" >&2
  exit 2
fi
SOURCE_SHA="$(git -C "$ROOT" rev-parse HEAD)"
if [[ "$SOURCE_SHA" != "$EXPECTED" ]]; then
  echo "Refusing non-GA source SHA: $SOURCE_SHA" >&2
  exit 2
fi
if [[ "$(git -C "$ROOT" rev-parse 'v1.0.0^{commit}')" != "$EXPECTED" ]] ||
   [[ "$(git -C "$ROOT" cat-file -t refs/tags/v1.0.0)" != "tag" ]]; then
  echo "Missing or mismatched immutable annotated v1.0.0 tag" >&2
  exit 2
fi
if [[ -n "$(git -C "$ROOT" status --porcelain --untracked-files=all)" ]]; then
  echo "Release worktree is not clean" >&2
  exit 2
fi
command -v "$GO_BIN" >/dev/null
command -v make >/dev/null
command -v timeout >/dev/null
command -v sha256sum >/dev/null
cd "$ROOT"
BUILD_DIR="$(mktemp -d)"
printf 'stage\tstatus\texit_code\tstarted_utc\tfinished_utc\n' > "$OUT/stages.tsv"

finish() {
  local rc=$?
  trap - EXIT
  if [[ -n "$ACTIVE" ]]; then
    printf '%s\tINTERRUPTED\t%s\t%s\t%s\n' "$ACTIVE" "$rc" "unknown" "$(date -u +%FT%TZ)" >> "$OUT/stages.tsv"
  fi
  local result=FAIL
  if (( rc == 0 && COMPLETED == 5 )) && [[ -z "$ACTIVE" ]]; then result=PASS; fi
  {
    echo "task=Q-COR-01"
    echo "target_release=v1.0.0"
    echo "source_sha=$SOURCE_SHA"
    echo "runner_sha=${GITHUB_SHA:-local}"
    echo "completed_stages=$COMPLETED/5"
    echo "result=$result"
    echo "performance_gate=DISABLED"
    echo "numerical_qualification=NOT_PERFORMED"
    echo "retained-attributed-B=INCONCLUSIVE"
    echo "finished_utc=$(date -u +%FT%TZ)"
  } > "$OUT/status.txt"
  find "$OUT" -type f ! -name SHA256SUMS -print0 | sort -z | xargs -0 -r sha256sum > "$OUT/SHA256SUMS"
  if [[ -n "$BUILD_DIR" ]]; then rm -rf -- "$BUILD_DIR"; fi
  echo "Q-COR-01 $result ($COMPLETED/5 stages); logs: $OUT"
  if [[ "$result" != PASS ]]; then exit 1; fi
}
trap finish EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

{
  echo "utc=$(date -u +%FT%TZ)"
  echo "source_sha=$SOURCE_SHA"
  echo "go_version=$("$GO_BIN" version)"
  echo "goos=$("$GO_BIN" env GOOS)"
  echo "goarch=$("$GO_BIN" env GOARCH)"
  echo "os=$(uname -srm)"
  git status --short --branch
  echo "release_tag=$(git rev-parse 'v1.0.0^{commit}')"
} > "$OUT/environment.txt"

stage() {
  local label="$1"; shift
  local started rc
  started="$(date -u +%FT%TZ)"
  ACTIVE="$label"
  echo "====== Q-COR-01 $label ======"
  set +e
  "$@" 2>&1 | tee "$OUT/$label.log"
  rc=${PIPESTATUS[0]}
  set -e
  printf '%s\t%s\t%s\t%s\t%s\n' "$label" "$([[ "$rc" -eq 0 ]] && echo PASS || echo FAIL)" "$rc" "$started" "$(date -u +%FT%TZ)" >> "$OUT/stages.tsv"
  ACTIVE=""
  if (( rc != 0 )); then return "$rc"; fi
  COMPLETED=$((COMPLETED+1))
}

build_fourteen_examples() {
  "$GO_BIN" list -f '{{if eq .Name "main"}}{{.ImportPath}}{{end}}' ./examples/... |
    sed '/^$/d' > "$OUT/example-main-packages.txt"
  local -a packages=()
  mapfile -t packages < "$OUT/example-main-packages.txt"
  if (( ${#packages[@]} != 14 )); then
    echo "Expected 14 distinct main example commands, got ${#packages[@]}" >&2
    return 1
  fi
  local package name
  declare -A names=()
  for package in "${packages[@]}"; do
    name="${package##*/}"
    if [[ -n "${names[$name]:-}" ]]; then
      echo "Duplicate executable name: $name" >&2
      return 1
    fi
    names[$name]=1
    "$GO_BIN" build -trimpath -o "$BUILD_DIR/$name" "$package"
    test -x "$BUILD_DIR/$name"
    echo "PASS build: $package"
  done
}

stage verify make verify
stage original-smoke make smoke
stage phase-d-examples bash scripts/examples-dev-smoke.sh
stage onrequest-qa007-race "$GO_BIN" test -race -count=2 -timeout=15m -run '^(TestClientInboundHandler|TestQA007)' ./...
stage build-fourteen build_fourteen_examples

echo "Q-COR-01: all five correctness stages completed on immutable v1.0.0 source."
