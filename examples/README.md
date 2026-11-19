# neta-go — Twelve runnable Client/Server examples

The original two public examples are preserved. Ten additional independent stdlib-only commands demonstrate configuration, capacity, reconnect, V1, Bind policy and Diagnostics. Run every command from repository root with Go 1.23+.

## Paired commands (terminal 1 = Server, terminal 2 = Client)

| Scenario | Start the Server | Then run the Client | Expected |
| --- | --- | --- | --- |
| Basic, unchanged | go run ./examples/server | go run ./examples/client | Bind, echo Completion, one-way event, Ctrl-C Stop |
| Config CLI/JSON | go run ./examples/server-config -config examples/config/server.json | go run ./examples/client-config -config examples/config/client.json | Strict config with public Functional Options and Completion |
| Bounded concurrency | go run ./examples/server-concurrency -max-bound 8 -app-workers 4 -app-concurrency 2 | go run ./examples/client-concurrent -count 16 -inflight 4 | Worker and Request credits bounded; correlated Completion |
| Reconnect | go run ./examples/server | go run ./examples/client-reconnect -window 60s | Bound generations and CurrentSession on abrupt transport loss, without replay |
| Demo Bind allowlist | go run ./examples/server-bind-validator -allow neta-client-config,neta-concurrent | go run ./examples/client-config -system-id neta-client-config | Accepted SystemID; unknown/duplicate IDs rejected |
| V1 only | go run ./examples/server-v1 | go run ./examples/client-v1 | NUL-terminated V1 Bind; reject default V2 clients |
| Diagnostics normal | go run ./examples/server-diagnostics | go run ./examples/client-diagnostics | Completion, typed Session.Diagnostics snapshot |
| Diagnostics timeout | go run ./examples/server-diagnostics -drop-timeout-demo=true | go run ./examples/client-diagnostics -mode=timeout -timeout=500ms | Expected RequestTimeout Completion |
| Diagnostics Completion panic (opt in) | go run ./examples/server-diagnostics | go run ./examples/client-diagnostics -mode=panic | Recovered callback panic, OnClose cause and typed record |
| Diagnostics Server OnRequest panic (opt in) | go run ./examples/server-diagnostics -panic-on-request=true | go run ./examples/client-diagnostics -mode=server-panic | Peer panic closes Session; Server OnClose prints first recovered OnRequest record |

Long-running Servers stop with Ctrl-C. The original basic Client also stays alive after its first Request until Ctrl-C. New example Clients have finite windows or Request deadlines. For ephemeral ports, start Server with -listen 127.0.0.1:0, copy the printed actual address to Client -addr.

## EX-C01/EX-S01 — optional JSON and CLI precedence

Sample configuration files: examples/config/client.json and examples/config/server.json. Both programs work without -config and do not override library defaults for unspecified fields.

~~~bash
go run ./examples/server-config -config examples/config/server.json -listen 127.0.0.1:2775 -max-bound 4
go run ./examples/client-config -config examples/config/client.json -addr 127.0.0.1:2775 -system-id cli-identity -protocol v2 -request-timeout 5s
~~~

Precedence is **explicit CLI flag > JSON field > library default**. File parsing rejects unknown fields and trailing JSON values; durations use Go time.ParseDuration, for example 5s or 250ms. Invalid version/identity/limits fail before Connect/Start. Flags are available with -h. No hot reload, additional dependency or exported configuration framework is used.

Client flags include -addr, -system-id, -bind-type (bi/uni), -protocol (v1/v2), connect/bind/Request/Write timeouts, -enquire-idle (0s disables), -auto-reconnect, -max-outstanding, -app-workers and -max-retained-app-bytes. Server flags include -listen, -protocol, Pre-Bind/Frame/Write timeouts, -max-bound, -max-pending-prebind, worker ceilings, -onclose-credits, -queue-bytes/-queue-frames, -app-concurrency and -max-incoming. All nonmandatory values inherit the library's documented defaults.

## EX-C02/EX-S03 — concurrency and capacity

client-concurrent launches no more than its -inflight worker count (1..32) and submits at most -count bounded Request jobs (1..1000). It waits for matching asynchronous Completion, validates Sequence/Status/echo and prints separate local-admission and terminal-result messages. Admission errors and RequestTimeout are failures, not silently successful deliveries. Server limits active callback workers, per-Session callback concurrency and owner-wide queued frame/byte budgets.

To demonstrate multiple simultaneously Bound Sessions, start server-concurrency, then launch two client-concurrent instances at the same time with **different** -system-id values.

## EX-C03 — reconnect without replay

Auto-reconnect is **opt-in**. It reacts to approved retryable transport failures, uses bounded jitter, observes OnBound/OnClose and changes of CurrentSession. A graceful peer Unbind is terminal, not a retry signal. For a real reconnect demo, run the Server as a compiled binary, terminate its process **abruptly**, restart it listening at the same address, and inspect the Client logs for a second Bound generation. The Client does not submit or replay any Requests. No guaranteed reconnect time or at-least-once claim is made.

## EX-S02 — BindValidator is not authentication

A nonempty demo SystemID allowlist is supplied via -allow; the library itself also prevents simultaneous duplicate bound SystemIDs. The sample does not prove a client's identity cryptographically and must not be deployed as a production authentication service. SystemID is an identity label, not a secret. V1 19-byte and V2 20-byte limits remain enforced by the protocol.

## EX-C04/EX-S04 — protocol versions

| | V1 | V2 (default) |
| --- | --- | --- |
| Bind | BindType + SystemID + NUL terminator | BindType + SystemID, no NUL |
| SystemID limit | 19 bytes | 20 bytes |
| Header and Stream | Fixed 10-byte header, unchanged | Fixed 10-byte header, unchanged |
| Peer choice | V1-only Server rejects a V2 Bind | Default V2 Server detects incoming V1/V2 Bind shape and responds in kind |

A Client must choose V1 explicitly with WithClientSessionOptions(WithProtocolVersion(ProtocolVersionV1)). There is no automatic downgrade, negotiation frame or Header version field.

## EX-C05/EX-S05 — Completion, panic and shutdown

Safe default settings do not intentionally panic or omit responses. Use the explicit -mode=timeout Client with the Server -drop-timeout-demo=true to demonstrate no Response and a terminal RequestTimeout callback. Use the explicit -mode=panic Client to demonstrate a recovered local Completion panic and a bounded typed Session.Diagnostics snapshot. The Server also supports -panic-on-request=true for an explicitly prefixed panic: Request; the default is false. Recent diagnostic record ring/byte ceilings are not total heap caps, and foreign panic graphs are not safely measurable. Do not expose arbitrary recovered raw objects.

Session.Done, Client.Stop and Server.Stop(ctx) wait for internal transport teardown; **not arbitrary user callbacks**. Clean graceful shutdown is a bounded operation. No example is a reliable-delivery protocol.

## Fast Development verification

~~~bash
bash scripts/examples-dev-smoke.sh
~~~

The fast harness builds all 12 binaries, runs focused example unit tests (including V1-only rejection and duplicate/unknown Bind tests), checks twelve positive/negative Client/Server loopback pairs, and tests a real Client reconnect by killing and restarting the Server on the identical ephemeral port. Readiness, second Bound generation, cleanup and client deadlines are bounded and observable. Logs go to artifacts/examples-dev. It is **not** QA-004 P99, QA-005 matched Core/Full runs, 30-minute soak, stress/fuzz or Performance Gate qualification. The original scripts/example-smoke.sh remains unchanged.
