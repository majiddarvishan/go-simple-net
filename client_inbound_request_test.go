package neta

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

type inboundResult struct {
	response Response
	err      error
}

func startInboundTestPair(t *testing.T, serverOptions []ServerOption, clientOptions []ClientOption) (*Server, *Client, *Session, *Session) {
	t.Helper()
	bound := make(chan *Session, 1)
	serverOptions = append(serverOptions,
		WithServerListenAddress("127.0.0.1:0"),
		WithServerOnBoundHandler(func(s *Session) { bound <- s }),
	)
	server, err := NewServer(serverOptions...)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Stop(ctx)
	})

	clientOptions = append(clientOptions,
		WithClientAddress(server.Addr().String()),
		WithClientSystemID("inbound-test-peer"),
	)
	client, err := NewClient(clientOptions...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = client.Stop(ctx)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	clientSession, ok := client.CurrentSession()
	if !ok {
		t.Fatal("successful Connect did not expose Bound Session")
	}
	select {
	case serverSession := <-bound:
		return server, client, serverSession, clientSession
	case <-ctx.Done():
		t.Fatal("Server OnBound callback did not run")
	}
	return nil, nil, nil, nil
}

func inboundCompletion(t *testing.T, ch <-chan inboundResult) inboundResult {
	t.Helper()
	select {
	case result := <-ch:
		return result
	case <-time.After(4 * time.Second):
		t.Fatal("completion did not arrive within bounded test window")
	}
	return inboundResult{}
}

func TestClientInboundHandlerBidirectionalRequestAndResponse(t *testing.T) {
	for _, version := range []ProtocolVersion{ProtocolVersionV1, ProtocolVersionV2} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			clientInbound := make(chan []byte, 1)
			_, _, serverSession, clientSession := startInboundTestPair(t,
				[]ServerOption{
					WithServerOnRequestHandler(func(s *Session, r Request) {
						if err := s.SendResponse(r.Sequence, append([]byte("server:"), r.Payload...), 0); err != nil {
							t.Errorf("server SendResponse: %v", err)
						}
					}),
				},
				[]ClientOption{
					WithClientSessionOptions(WithProtocolVersion(version)),
					WithClientOnRequestHandler(func(s *Session, r Request) {
						clientInbound <- r.Payload
						if err := s.SendResponse(r.Sequence, append([]byte("client:"), r.Payload...), 0); err != nil {
							t.Errorf("client SendResponse: %v", err)
						}
					}),
				},
			)
			serverCompletion := make(chan inboundResult, 1)
			serverSeq, err := serverSession.SendRequest([]byte("from-server"), func(r Response, e error) {
				serverCompletion <- inboundResult{r, e}
			})
			if err != nil {
				t.Fatalf("server SendRequest: %v", err)
			}
			clientCompletion := make(chan inboundResult, 1)
			clientSeq, err := clientSession.SendRequest([]byte("from-client"), func(r Response, e error) {
				clientCompletion <- inboundResult{r, e}
			})
			if err != nil {
				t.Fatalf("client SendRequest: %v", err)
			}
			serverResult := inboundCompletion(t, serverCompletion)
			clientResult := inboundCompletion(t, clientCompletion)
			if serverResult.err != nil || serverResult.response.Sequence != serverSeq ||
				serverResult.response.Status != 0 || !bytes.Equal(serverResult.response.Payload, []byte("client:from-server")) {
				t.Fatalf("server-originated request completion: %+v", serverResult)
			}
			if clientResult.err != nil || clientResult.response.Sequence != clientSeq ||
				clientResult.response.Status != 0 || !bytes.Equal(clientResult.response.Payload, []byte("server:from-client")) {
				t.Fatalf("client-originated request completion: %+v", clientResult)
			}
			select {
			case payload := <-clientInbound:
				if !bytes.Equal(payload, []byte("from-server")) {
					t.Fatalf("Client OnRequest payload = %q", payload)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("Client OnRequest was not called")
			}
			if clientSession.State() != SessionStateBound {
				t.Fatalf("Client state = %v after concurrent bidirectional round trips", clientSession.State())
			}
		})
	}
}

func TestClientInboundHandlerNilKeepsOldIgnoreBehavior(t *testing.T) {
	_, _, serverSession, clientSession := startInboundTestPair(t,
		[]ServerOption{
			WithServerSessionOptions(WithRequestTimeout(200 * time.Millisecond)),
			WithServerOnRequestHandler(func(s *Session, r Request) {
				_ = s.SendResponse(r.Sequence, []byte("accepted"), 0)
			}),
		},
		nil, // No Client OnRequest handler: drain/ignore inbound Requests.
	)
	unhandled := make(chan inboundResult, 1)
	if _, err := serverSession.SendRequest([]byte("unhandled"), func(r Response, e error) {
		unhandled <- inboundResult{r, e}
	}); err != nil {
		t.Fatal(err)
	}
	if r := inboundCompletion(t, unhandled); !errors.Is(r.err, ErrRequestTimeout) {
		t.Fatalf("no handler expected RequestTimeout, got %+v", r)
	}
	if clientSession.State() != SessionStateBound {
		t.Fatal("ignoring an inbound Request must not close Client")
	}
	handled := make(chan inboundResult, 1)
	if _, err := clientSession.SendRequest([]byte("still-working"), func(r Response, e error) {
		handled <- inboundResult{r, e}
	}); err != nil {
		t.Fatal(err)
	}
	if r := inboundCompletion(t, handled); r.err != nil || string(r.response.Payload) != "accepted" {
		t.Fatalf("client outbound after ignored inbound: %+v", r)
	}
}

func TestClientInboundHandlerRejectsUnidirectionalBind(t *testing.T) {
	_, err := NewClient(
		WithClientAddress("127.0.0.1:1"),
		WithClientSystemID("peer"),
		WithClientBindType(BindTypeUniDirectional),
		WithClientOnRequestHandler(func(*Session, Request) {}),
	)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("Client with one-way Bind and inbound handler: got %v, want ErrInvalidConfig", err)
	}
	// Explicit nil is equivalent to the old default, even on a one-way link.
	_, err = NewClient(
		WithClientAddress("127.0.0.1:1"),
		WithClientSystemID("peer"),
		WithClientBindType(BindTypeUniDirectional),
		WithClientOnRequestHandler(nil),
	)
	if err != nil {
		t.Fatalf("nil handler must preserve backward compatibility: %v", err)
	}
}

func TestClientInboundHandlerPanicClosesOnlyAffectedSession(t *testing.T) {
	_, _, serverSession, clientSession := startInboundTestPair(t,
		[]ServerOption{WithServerSessionOptions(WithRequestTimeout(time.Second))},
		[]ClientOption{
			WithClientOnRequestHandler(func(*Session, Request) { panic("test inbound callback") }),
		},
	)
	terminal := make(chan inboundResult, 1)
	if _, err := serverSession.SendRequest([]byte("panic"), func(r Response, e error) {
		terminal <- inboundResult{r, e}
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-clientSession.Done():
	case <-time.After(4 * time.Second):
		t.Fatal("Client did not close after recovered OnRequest panic")
	}
	if !errors.Is(clientSession.Err(), ErrCallbackPanic) {
		t.Fatalf("Client terminal cause: got %v", clientSession.Err())
	}
	// The original accepted server Request must settle, never be replayed.
	r := inboundCompletion(t, terminal)
	if r.err == nil {
		t.Fatal("server Request unexpectedly completed successfully after peer panic")
	}
	// Panic diagnostic publication can race Done, so poll boundedly.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if record := clientSession.Diagnostics().CallbackPanics.First; record != nil {
			if record.Callback != "OnRequest" {
				t.Fatalf("first panic callback = %s", record.Callback)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("Client OnRequest panic diagnostics never published")
}
