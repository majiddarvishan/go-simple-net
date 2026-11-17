// Package neta provides a bounded, callback-oriented TCP messaging Client and
// Server over a Pinex-compatible TCP wire protocol.
//
// Construct a Client with NewClient and Connect, or construct a Server with
// NewServer and Start. A successful SendRequest or SendNoWait call confirms
// local queue admission only; it does not confirm peer delivery or processing.
// SendRequest reports its terminal result through a CompletionHandler.
//
// Request and Response payloads are owned snapshots. Outgoing payload bytes
// are copied before successful send admission returns. Incoming payloads may
// be retained by application code after the callback returns.
//
// User callbacks are dispatched asynchronously through bounded, owner-scoped
// Application, Completion and OnClose worker pools. They are never invoked
// inline by the transport Reader or Writer. Callback concurrency and resource
// budgets are configurable with the typed With... options.
//
// Client, Server and Session methods are safe for concurrent use. Stop is
// terminal and idempotent. Session.Done reports internal teardown completion;
// it does not wait for arbitrary user callbacks.
//
// The public API lives in this root package. Wire codecs, transport loops,
// callback schedulers and resource ledgers are implementation details.
package neta
