#!/usr/bin/env bash
# Exact-release, bounded CLI smoke for both generic Peer Router commands.
# Usage: bash scripts/phase-q-peer-router-smoke.sh <compiled-bin-dir> <artifacts>
set -Eeuo pipefail
if (( $# != 2 )); then
  echo "Usage: phase-q-peer-router-smoke.sh <bin-dir> <artifacts>" >&2
  exit 2
fi
BIN="$(cd -- "$1" && pwd)"
OUT="$(cd -- "$2" && pwd)"
SERVER="$BIN/server-peer-router"
CLIENT="$BIN/client-peer-router"
test -x "$SERVER" && test -x "$CLIENT"
command -v timeout >/dev/null
TMP="$(mktemp -d)"
mkfifo "$TMP/server.stdin"
# Keep a persistent write end; the server's console scanner must not see EOF.
exec 9<>"$TMP/server.stdin"
server_pid=""
east_pid=""
west_pid=""
cleanup() {
  local pid
  for pid in "$east_pid" "$west_pid" "$server_pid"; do
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
      kill -INT "$pid" 2>/dev/null || true
    fi
  done
  sleep 0.2
  for pid in "$east_pid" "$west_pid" "$server_pid"; do
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
      kill -KILL "$pid" 2>/dev/null || true
    fi
    if [[ -n "$pid" ]]; then wait "$pid" 2>/dev/null || true; fi
  done
  exec 9>&-
  rm -rf -- "$TMP"
}
trap cleanup EXIT

"$SERVER" -config examples/config/server-peer-router.json -listen 127.0.0.1:0 \
  <"$TMP/server.stdin" >"$OUT/router-server.log" 2>&1 &
server_pid="$!"
address=""
for ((i=0; i<200; i++)); do
  address="$(sed -nE 's/.*internal neta listener=([^ ]+).*/\1/p' "$OUT/router-server.log" | head -n 1)"
  if [[ -n "$address" ]]; then break; fi
  kill -0 "$server_pid" 2>/dev/null || { echo "Router Server exited before readiness" >&2; exit 1; }
  sleep 0.05
done
[[ -n "$address" ]] || { echo "Router Server readiness timeout" >&2; exit 1; }

"$CLIENT" -addr "$address" -system-id peer-east -window 15s >"$OUT/router-east.log" 2>&1 &
east_pid="$!"
"$CLIENT" -addr "$address" -system-id peer-west -window 15s >"$OUT/router-west.log" 2>&1 &
west_pid="$!"

bound=0
for ((i=0; i<200; i++)); do
  if grep -q 'peer Bound: bind_id="peer-east"' "$OUT/router-server.log" &&
     grep -q 'peer Bound: bind_id="peer-west"' "$OUT/router-server.log"; then
    bound=1; break
  fi
  kill -0 "$server_pid" 2>/dev/null || { echo "Router Server died during Bind" >&2; exit 1; }
  kill -0 "$east_pid" 2>/dev/null || { echo "East Client died during Bind" >&2; exit 1; }
  kill -0 "$west_pid" 2>/dev/null || { echo "West Client died during Bind" >&2; exit 1; }
  sleep 0.05
done
[[ "$bound" == 1 ]] || { echo "Both router peers did not bind" >&2; exit 1; }

printf 'send east hello-east\nsend west hello-west\nsend missing must-fail\n' >&9
complete=0
for ((i=0; i<200; i++)); do
  if [[ "$(grep -c 'outbound completion:' "$OUT/router-server.log" || true)" -ge 2 ]] &&
     grep -q 'not admitted:' "$OUT/router-server.log" &&
     grep -q 'peer received Request:.*hello-east' "$OUT/router-east.log" &&
     grep -q 'peer received Request:.*hello-west' "$OUT/router-west.log" &&
     grep -q 'peer initial Completion:' "$OUT/router-east.log" &&
     grep -q 'peer initial Completion:' "$OUT/router-west.log"; then
    complete=1; break
  fi
  kill -0 "$server_pid" 2>/dev/null || { echo "Router Server died during dispatch" >&2; exit 1; }
  sleep 0.05
done
[[ "$complete" == 1 ]] || { echo "Router bidirectional completion or unknown-route rejection missing" >&2; exit 1; }
[[ "$(grep -c 'outbound completion:.*err=<nil>' "$OUT/router-server.log" || true)" -ge 2 ]] ||
  { echo "Router Completion reported an error" >&2; exit 1; }

kill -INT "$east_pid" "$west_pid"
wait "$east_pid"
wait "$west_pid"
east_pid=""
west_pid=""
kill -INT "$server_pid"
wait "$server_pid"
server_pid=""
echo "PASS: 2 real bound CLI peers, two-way Request/Response, exact routing, unknown-route rejection, graceful stop."
