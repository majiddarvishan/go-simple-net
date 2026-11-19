#!/usr/bin/env bash
# Fast, bounded, stdlib-only examples check. Not QA-004/005 benchmark qualification.
set -euo pipefail

GO="${GO:-go}"
ARTIFACT_DIR="${ARTIFACT_DIR:-artifacts/examples-dev}"
mkdir -p "$ARTIFACT_DIR"
tmp="$(mktemp -d)"
active_server=""
active_server_log=""
active_client_log=""

cleanup() {
  local status=$?
  trap - EXIT
  if [[ -n "$active_server" ]] && kill -0 "$active_server" 2>/dev/null; then
    kill -INT "$active_server" 2>/dev/null || true
    sleep 0.2
    if kill -0 "$active_server" 2>/dev/null; then
      kill -KILL "$active_server" 2>/dev/null || true
    fi
    wait "$active_server" 2>/dev/null || true
  fi
  if [[ "$status" -ne 0 ]]; then
    echo "Lightweight example smoke failed; logs:" >&2
    [[ -f "$active_server_log" ]] && cat "$active_server_log" >&2
    [[ -f "$active_client_log" ]] && cat "$active_client_log" >&2
  fi
  rm -rf "$tmp"
  exit "$status"
}
trap cleanup EXIT

echo "== Build all twelve public example commands =="
examples=(
  client server
  client-config server-config
  client-concurrent server-concurrency
  client-reconnect server-bind-validator
  client-v1 server-v1
  client-diagnostics server-diagnostics
)
for example in "${examples[@]}"; do
  "$GO" build -trimpath -o "$tmp/$example" "./examples/$example"
done

echo "== Focused example unit tests =="
"$GO" test -count=1 ./examples/...

run_pair() {
  local scenario="$1" server="$2" client="$3"
  local -a server_args=(-listen 127.0.0.1:0)
  local -a client_args=()
  local server_log="$ARTIFACT_DIR/${scenario}-server.log"
  local client_log="$ARTIFACT_DIR/${scenario}-client.log"
  : >"$server_log"
  : >"$client_log"
  active_server_log="$server_log"
  active_client_log="$client_log"

  case "$scenario" in
    config-json)
      server_args=(-config examples/config/server.json -listen 127.0.0.1:0)
      ;;
    timeout)
      server_args+=(-drop-timeout-demo=true)
      ;;
  esac

  "$tmp/$server" "${server_args[@]}" >"$server_log" 2>&1 &
  active_server="$!"

  local address=""
  for ((i=0; i<200; i++)); do
    address="$(sed -nE 's/.*listening on ([^;]+);.*/\1/p' "$server_log" | head -n 1)"
    if [[ -n "$address" ]]; then
      break
    fi
    if ! kill -0 "$active_server" 2>/dev/null; then
      echo "$scenario: server exited before ready" >&2
      return 1
    fi
    sleep 0.05
  done
  if [[ -z "$address" ]]; then
    echo "$scenario: server readiness timeout" >&2
    return 1
  fi

  client_args=(-addr "$address")
  case "$scenario" in
    config-json)
      client_args=(-config examples/config/client.json -addr "$address")
      ;;
    concurrent)
      client_args+=(-count=8 -inflight=2)
      ;;
    reconnect)
      client_args+=(-window=2s)
      ;;
    timeout)
      client_args+=(-mode=timeout -timeout=500ms)
      ;;
    diagnostics)
      client_args+=(-mode=ok)
      ;;
    panic)
      client_args+=(-mode=panic)
      ;;
  esac
  echo "== $scenario: $server $client on $address =="
  timeout --signal=INT --kill-after=3s 35s "$tmp/$client" "${client_args[@]}" >"$client_log" 2>&1 || {
    echo "$scenario: client failed or exceeded 35s" >&2
    return 1
  }

  case "$scenario" in
    config|config-json|bind) grep -q "Completion sequence=" "$client_log" ;;
    concurrent) grep -q 'finished: requested=8 locally_admitted=8 completions_ok=8 failures=0' "$client_log" ;;
    reconnect) grep -q 'window ended; observed OnBound callbacks=' "$client_log" ;;
    v1) grep -q 'V1 Completion sequence=' "$client_log" ;;
    diagnostics) grep -q 'diagnostics: recent_panics=' "$client_log" ;;
    timeout) grep -q 'expected RequestTimeout occurred' "$client_log" ;;
    panic) grep -q 'recovered Completion panic:' "$client_log" ;;
    *) echo "unsupported smoke scenario $scenario" >&2; return 1 ;;
  esac

  kill -INT "$active_server" 2>/dev/null || true
  if ! wait "$active_server"; then
    echo "$scenario: Server did not stop cleanly" >&2
    return 1
  fi
  active_server=""
  echo "PASS: $scenario"
}

# Original commands are built; existing scripts/example-smoke.sh stays unchanged.
run_pair config server-config client-config
run_pair config-json server-config client-config
run_pair concurrent server-concurrency client-concurrent
run_pair bind server-bind-validator client-config
run_pair reconnect server-bind-validator client-reconnect
run_pair v1 server-v1 client-v1
run_pair diagnostics server-diagnostics client-diagnostics
run_pair timeout server-diagnostics client-diagnostics
run_pair panic server-diagnostics client-diagnostics

echo "EXAMPLE_DEV_SMOKE=PASS" >"$ARTIFACT_DIR/summary.txt"
echo "PASS: twelve example builds, unit tests and nine bounded loopback pairs"
