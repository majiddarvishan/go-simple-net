#!/usr/bin/env bash
# Fast, bounded, stdlib-only examples check. Not QA-004/005 benchmark qualification.
set -euo pipefail

GO="${GO:-go}"
ARTIFACT_DIR="${ARTIFACT_DIR:-artifacts/examples-dev}"
mkdir -p "$ARTIFACT_DIR"
tmp="$(mktemp -d)"
active_server=""
active_client=""
active_server_log=""
active_client_log=""

cleanup() {
  local status=$?
  trap - EXIT
  if [[ -n "$active_client" ]] && kill -0 "$active_client" 2>/dev/null; then
    kill -INT "$active_client" 2>/dev/null || true
    wait "$active_client" 2>/dev/null || true
  fi
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
    server-panic)
      server_args+=(-panic-on-request=true)
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
    server-panic)
      client_args+=(-mode=server-panic)
      ;;
    bind-reject)
      client_args+=(-system-id=unknown-demo)
      ;;
    v2-reject)
      client_args+=(-system-id=v2-not-supported)
      ;;
  esac
  echo "== $scenario: $server $client on $address =="
  if [[ "$scenario" == "bind-reject" || "$scenario" == "v2-reject" ]]; then
    if timeout --signal=INT --kill-after=3s 35s "$tmp/$client" "${client_args[@]}" >"$client_log" 2>&1; then
      echo "$scenario: unexpected Bind success" >&2
      return 1
    fi
  else
    timeout --signal=INT --kill-after=3s 35s "$tmp/$client" "${client_args[@]}" >"$client_log" 2>&1 || {
      echo "$scenario: client failed or exceeded 35s" >&2
      return 1
    }
  fi

  case "$scenario" in
    config|config-json|bind) grep -q "Completion sequence=" "$client_log" ;;
    concurrent) grep -q 'finished: requested=8 locally_admitted=8 completions_ok=8 failures=0' "$client_log" ;;
    reconnect) grep -q 'window ended; observed OnBound callbacks=' "$client_log" ;;
    v1) grep -q 'V1 Completion sequence=' "$client_log" ;;
    diagnostics) grep -q 'diagnostics: recent_panics=' "$client_log" ;;
    timeout) grep -q 'expected RequestTimeout occurred' "$client_log" ;;
    panic) grep -q 'recovered Completion panic:' "$client_log" ;;
    bind-reject) grep -q 'Bind rejected by demo allowlist' "$server_log" ;;
    v2-reject) grep -q 'Connect:' "$client_log" ;;
    server-panic)
      grep -q 'expected peer callback panic terminated transport' "$client_log"
      local seen=0
      for ((i=0; i<100; i++)); do
        if grep -q 'first recovered callback="OnRequest"' "$server_log"; then seen=1; break; fi
        sleep 0.05
      done
      [[ "$seen" == 1 ]] || { echo "server did not record OnRequest callback panic" >&2; return 1; }
      ;;
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


# A separate, real lifecycle check, not merely the initial Bound state.
# Server readiness and the Client's second OnBound are polled, never guessed.
run_reconnect_restart() {
  local scenario="reconnect-restart"
  local server_log="$ARTIFACT_DIR/reconnect-restart-server.log"
  local restarted_log="$ARTIFACT_DIR/reconnect-restart-server-restarted.log"
  local client_log="$ARTIFACT_DIR/reconnect-restart-client.log"
  active_server_log="$server_log"
  active_client_log="$client_log"
  : >"$server_log"; : >"$restarted_log"; : >"$client_log"
  "$tmp/server-bind-validator" -listen 127.0.0.1:0 >"$server_log" 2>&1 &
  active_server="$!"
  local address=""
  for ((i=0; i<200; i++)); do
    address="$(sed -nE 's/.*listening on ([^;]+);.*/\1/p' "$server_log" | head -n 1)"
    [[ -n "$address" ]] && break
    kill -0 "$active_server" 2>/dev/null || { echo "first Server exited before ready" >&2; return 1; }
    sleep 0.05
  done
  [[ -n "$address" ]] || { echo "first Server not ready" >&2; return 1; }
  "$tmp/client-reconnect" -addr "$address" -window=22s >"$client_log" 2>&1 &
  active_client="$!"
  local observed_first=0
  for ((i=0; i<200; i++)); do
    if grep -q 'OnBound generation-observation=1' "$client_log"; then observed_first=1; break; fi
    kill -0 "$active_client" 2>/dev/null || { echo "Client exited before first Bind" >&2; return 1; }
    sleep 0.05
  done
  [[ "$observed_first" == 1 ]] || { echo "first Bind missing" >&2; return 1; }
  kill -KILL "$active_server"
  wait "$active_server" 2>/dev/null || true
  active_server=""
  "$tmp/server-bind-validator" -listen "$address" >"$restarted_log" 2>&1 &
  active_server="$!"
  local observed_second=0
  for ((i=0; i<300; i++)); do
    if grep -q 'OnBound generation-observation=2' "$client_log"; then observed_second=1; break; fi
    kill -0 "$active_client" 2>/dev/null || { echo "Client exited before reconnect" >&2; return 1; }
    kill -0 "$active_server" 2>/dev/null || { echo "replacement Server exited" >&2; return 1; }
    sleep 0.05
  done
  [[ "$observed_second" == 1 ]] || { echo "second Bound after restart was not observed" >&2; return 1; }
  kill -INT "$active_client" 2>/dev/null || true
  wait "$active_client" || { echo "reconnect Client shutdown failed" >&2; return 1; }
  active_client=""
  kill -INT "$active_server" 2>/dev/null || true
  wait "$active_server" || { echo "replacement Server shutdown failed" >&2; return 1; }
  active_server=""
  echo "PASS: real reconnect after abrupt Server loss and new Server on same port"
}

# Original commands are built; existing scripts/example-smoke.sh stays unchanged.
run_pair config server-config client-config
run_pair config-json server-config client-config
run_pair concurrent server-concurrency client-concurrent
run_pair bind server-bind-validator client-config
run_pair bind-reject server-bind-validator client-config
run_pair reconnect server-bind-validator client-reconnect
run_pair v1 server-v1 client-v1
run_pair v2-reject server-v1 client-config
run_pair diagnostics server-diagnostics client-diagnostics
run_pair timeout server-diagnostics client-diagnostics
run_pair panic server-diagnostics client-diagnostics
run_pair server-panic server-diagnostics client-diagnostics
run_reconnect_restart

echo "EXAMPLE_DEV_SMOKE=PASS" >"$ARTIFACT_DIR/summary.txt"
echo "PASS: twelve example builds, unit tests and twelve bounded loopback pairs plus real restart/reconnect"
