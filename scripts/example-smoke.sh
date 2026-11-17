#!/usr/bin/env bash
set -euo pipefail

GO="${GO:-go}"
ARTIFACT_DIR="${ARTIFACT_DIR:-artifacts/smoke}"
mkdir -p "$ARTIFACT_DIR"

TMP_DIR="$(mktemp -d)"
SERVER_LOG="$TMP_DIR/server.log"
CLIENT_LOG="$TMP_DIR/client.log"
SERVER_PID=""
CLIENT_PID=""

stop_process() {
  local pid="$1"
  if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
    kill -INT "$pid" 2>/dev/null || true
    sleep 1
    if kill -0 "$pid" 2>/dev/null; then
      kill -KILL "$pid" 2>/dev/null || true
    fi
    wait "$pid" 2>/dev/null || true
  fi
}

cleanup() {
  local exit_code="$?"
  trap - EXIT
  stop_process "$CLIENT_PID"
  stop_process "$SERVER_PID"

  if [[ -f "$CLIENT_LOG" ]]; then
    cp "$CLIENT_LOG" "$ARTIFACT_DIR/example-client.log"
  fi
  if [[ -f "$SERVER_LOG" ]]; then
    cp "$SERVER_LOG" "$ARTIFACT_DIR/example-server.log"
  fi
  if [[ "$exit_code" -ne 0 ]]; then
    echo "Example/coverage check failed; captured logs:" >&2
    [[ -f "$CLIENT_LOG" ]] && cat "$CLIENT_LOG" >&2
    [[ -f "$SERVER_LOG" ]] && cat "$SERVER_LOG" >&2
  fi
  rm -rf "$TMP_DIR"
  exit "$exit_code"
}
trap cleanup EXIT

echo "== Coverage snapshot =="
"$GO" test -count=1 -cover ./... | tee "$ARTIFACT_DIR/coverage.txt"

echo "== Build public examples =="
"$GO" build -trimpath -o "$TMP_DIR/server" ./examples/server
"$GO" build -trimpath -o "$TMP_DIR/client" ./examples/client

echo "== Start Server on an ephemeral loopback port =="
"$TMP_DIR/server" -listen 127.0.0.1:0 >"$SERVER_LOG" 2>&1 &
SERVER_PID="$!"

SERVER_ADDR=""
for _ in {1..100}; do
  if grep -q "listening on " "$SERVER_LOG" 2>/dev/null; then
    SERVER_ADDR="$(sed -nE 's/.*listening on ([^;]+);.*/\1/p' "$SERVER_LOG" | head -n 1)"
    if [[ -n "$SERVER_ADDR" ]]; then
      break
    fi
  fi
  if ! kill -0 "$SERVER_PID" 2>/dev/null; then
    echo "Server exited before becoming ready" >&2
    exit 1
  fi
  sleep 0.1
done

if [[ -z "$SERVER_ADDR" ]]; then
  echo "Server did not publish its bound address within 10 seconds" >&2
  exit 1
fi

echo "== Run Client and verify Request/Response + one-way delivery =="
"$TMP_DIR/client" -addr "$SERVER_ADDR" -system-id neta-example-smoke >"$CLIENT_LOG" 2>&1 &
CLIENT_PID="$!"

READY=0
for _ in {1..100}; do
  if grep -q "Completion sequence=" "$CLIENT_LOG" 2>/dev/null \
    && grep -q "one-way sequence=" "$CLIENT_LOG" 2>/dev/null \
    && grep -q "one-way event sequence=" "$SERVER_LOG" 2>/dev/null; then
    READY=1
    break
  fi
  if ! kill -0 "$CLIENT_PID" 2>/dev/null; then
    echo "Client exited before the Request/Response smoke completed" >&2
    exit 1
  fi
  if ! kill -0 "$SERVER_PID" 2>/dev/null; then
    echo "Server exited during the Client smoke" >&2
    exit 1
  fi
  sleep 0.1
done

if [[ "$READY" -ne 1 ]]; then
  echo "Timed out waiting for the Client Completion and Server one-way event" >&2
  exit 1
fi

echo "== Gracefully stop Client and Server =="
kill -INT "$CLIENT_PID"
wait "$CLIENT_PID"
CLIENT_PID=""

if grep -q "Client Stop:" "$CLIENT_LOG"; then
  echo "Client reported an error during graceful Stop" >&2
  exit 1
fi

kill -INT "$SERVER_PID"
wait "$SERVER_PID"
SERVER_PID=""

if ! grep -q "Server stopped" "$SERVER_LOG"; then
  echo "Server did not report graceful Stop completion" >&2
  exit 1
fi

{
  echo "GO_VERSION=$("$GO" version)"
  echo "GOOS=$("$GO" env GOOS)"
  echo "GOARCH=$("$GO" env GOARCH)"
  echo "COVERAGE=$ARTIFACT_DIR/coverage.txt"
  echo "CLIENT_SERVER_LOOPBACK_SMOKE=PASS"
  echo "GRACEFUL_CLIENT_SERVER_STOP=PASS"
} >"$ARTIFACT_DIR/summary.txt"

echo "PASS: coverage, example builds, loopback Request/Response, one-way event and graceful shutdown"
