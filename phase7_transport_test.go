package neta

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/majiddarvishan/neta-go/internal/wire"
)

func readPhase7Frame(conn net.Conn) (wire.Frame, error) {
	var headerBytes [wire.HeaderSize]byte
	if _, err := io.ReadFull(conn, headerBytes[:]); err != nil {
		return wire.Frame{}, err
	}
	header, err := wire.ParseHeader(headerBytes[:])
	if err != nil {
		return wire.Frame{}, err
	}
	body := make([]byte, int(header.Length)-wire.HeaderSize)
	if _, err := io.ReadFull(conn, body); err != nil {
		return wire.Frame{}, err
	}
	return wire.Frame{Header: header, Body: body}, nil
}

func writePhase7Full(conn net.Conn, data []byte) error {
	for len(data) > 0 {
		n, err := conn.Write(data)
		if n > 0 {
			data = data[n:]
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}

func writePhase7Fragmented(conn net.Conn, data []byte, chunkSize int, delay time.Duration) error {
	if chunkSize < 1 {
		chunkSize = 1
	}
	for len(data) > 0 {
		n := chunkSize
		if n > len(data) {
			n = len(data)
		}
		if err := writePhase7Full(conn, data[:n]); err != nil {
			return err
		}
		data = data[n:]
		if delay > 0 {
			time.Sleep(delay)
		}
	}
	return nil
}

func TestPhase7TCPFragmentedBindAndOutOfOrderResponses(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	peerDone := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			peerDone <- err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(6 * time.Second))

		bindFrame, err := readPhase7Frame(conn)
		if err != nil {
			peerDone <- err
			return
		}
		bind, err := wire.DecodeBindRequest(bindFrame)
		if err != nil {
			peerDone <- err
			return
		}
		bindResponse, err := wire.EncodeBindResponse(bindFrame.Header.Sequence, wire.StatusOK, wire.Bind{
			Type: bind.Type, SystemID: "fragment-peer",
		})
		if err != nil {
			peerDone <- err
			return
		}
		if err := writePhase7Fragmented(conn, bindResponse, 1, time.Millisecond); err != nil {
			peerDone <- err
			return
		}

		first, err := readPhase7Frame(conn)
		if err != nil {
			peerDone <- err
			return
		}
		second, err := readPhase7Frame(conn)
		if err != nil {
			peerDone <- err
			return
		}
		if first.Header.Command != wire.CommandStreamRequest || second.Header.Command != wire.CommandStreamRequest {
			peerDone <- errors.New("expected two Stream Requests")
			return
		}

		secondResponse, err := wire.EncodeStreamResponse(second.Header.Sequence, wire.StatusOK, append([]byte("reply:"), second.Body...))
		if err != nil {
			peerDone <- err
			return
		}
		firstResponse, err := wire.EncodeStreamResponse(first.Header.Sequence, wire.StatusOK, append([]byte("reply:"), first.Body...))
		if err != nil {
			peerDone <- err
			return
		}
		if err := writePhase7Fragmented(conn, secondResponse, 3, 0); err != nil {
			peerDone <- err
			return
		}
		if err := writePhase7Fragmented(conn, firstResponse, 2, 0); err != nil {
			peerDone <- err
			return
		}

		unbind, err := readPhase7Frame(conn)
		if err != nil {
			peerDone <- err
			return
		}
		if unbind.Header.Command != wire.CommandUnbindRequest {
			peerDone <- errors.New("expected Unbind Request")
			return
		}
		response, err := wire.EncodeUnbindResponse(unbind.Header.Sequence)
		if err == nil {
			err = writePhase7Full(conn, response)
		}
		peerDone <- err
	}()

	client, err := NewClient(
		WithClientAddress(listener.Addr().String()),
		WithClientSystemID("fragment-client"),
		WithBindResponseTimeout(3*time.Second),
		WithClientSessionOptions(WithFrameAssemblyTimeout(2*time.Second)),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect after fragmented Bind Response: %v", err)
	}
	session, ok := client.CurrentSession()
	if !ok {
		t.Fatal("fragmented Bind did not publish a Bound Session")
	}

	type result struct {
		response Response
		err      error
	}
	completed := make(chan result, 2)
	firstSequence, err := session.SendRequest([]byte("first"), func(response Response, err error) {
		completed <- result{response: response, err: err}
	})
	if err != nil {
		t.Fatalf("SendRequest first: %v", err)
	}
	secondSequence, err := session.SendRequest([]byte("second"), func(response Response, err error) {
		completed <- result{response: response, err: err}
	})
	if err != nil {
		t.Fatalf("SendRequest second: %v", err)
	}
	if firstSequence == secondSequence {
		t.Fatal("two Requests received the same Sequence")
	}

	seen := make(map[uint32]string, 2)
	for range 2 {
		select {
		case result := <-completed:
			if result.err != nil {
				t.Fatalf("Completion: %v", result.err)
			}
			seen[result.response.Sequence] = string(result.response.Payload)
		case <-ctx.Done():
			t.Fatal("timed out waiting for out-of-order Responses")
		}
	}
	if seen[firstSequence] != "reply:first" || seen[secondSequence] != "reply:second" {
		t.Fatalf("correlation after reordered Responses = %#v", seen)
	}

	if err := client.Stop(ctx); err != nil {
		t.Fatalf("graceful Stop: %v", err)
	}
	select {
	case err := <-peerDone:
		if err != nil {
			t.Fatalf("TCP peer: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("TCP peer did not complete Unbind")
	}
}

func TestPhase7SlowTrickleCannotExtendBindResponseDeadline(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	peerDone := make(chan struct{})
	go func() {
		defer close(peerDone)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
		request, err := readPhase7Frame(conn)
		if err != nil {
			return
		}
		response, err := wire.EncodeBindResponse(request.Header.Sequence, wire.StatusOK, wire.Bind{
			Type: wire.BindBiDirectional, SystemID: "slow-peer",
		})
		if err != nil {
			return
		}
		// Frame Assembly timing starts after the complete Header is available.
		_ = writePhase7Full(conn, response[:wire.HeaderSize+2])
		time.Sleep(150 * time.Millisecond)
		_ = writePhase7Full(conn, response[wire.HeaderSize+2:])
	}()

	client, err := NewClient(
		WithClientAddress(listener.Addr().String()),
		WithClientSystemID("slow-client"),
		WithBindResponseTimeout(60*time.Millisecond),
		WithClientSessionOptions(WithFrameAssemblyTimeout(time.Second)),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	started := time.Now()
	err = client.Connect(ctx)
	elapsed := time.Since(started)
	if err == nil {
		t.Fatal("Connect unexpectedly accepted a Bind Response beyond its absolute deadline")
	}
	if elapsed > 700*time.Millisecond {
		t.Fatalf("absolute Bind Response deadline took %s", elapsed)
	}
	if _, ok := client.CurrentSession(); ok {
		t.Fatal("timed-out Bind published a current Session")
	}
	_ = client.Stop(context.Background())
	<-peerDone
}

func TestPhase7AbruptTCPResetClosesBoundGeneration(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	releaseReset := make(chan struct{})
	peerDone := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			peerDone <- err
			return
		}
		request, err := readPhase7Frame(conn)
		if err != nil {
			_ = conn.Close()
			peerDone <- err
			return
		}
		bind, err := wire.DecodeBindRequest(request)
		if err != nil {
			_ = conn.Close()
			peerDone <- err
			return
		}
		response, err := wire.EncodeBindResponse(request.Header.Sequence, wire.StatusOK, wire.Bind{
			Type: bind.Type, SystemID: "reset-peer",
		})
		if err == nil {
			err = writePhase7Full(conn, response)
		}
		if err != nil {
			_ = conn.Close()
			peerDone <- err
			return
		}
		<-releaseReset
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			_ = tcpConn.SetLinger(0)
		}
		_ = conn.Close()
		peerDone <- nil
	}()

	closed := make(chan error, 1)
	client, err := NewClient(
		WithClientAddress(listener.Addr().String()),
		WithClientSystemID("reset-client"),
		WithClientOnCloseHandler(func(_ *Session, cause error) {
			closed <- cause
		}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	close(releaseReset)

	select {
	case cause := <-closed:
		if cause == nil {
			t.Fatal("RST close notification had no terminal cause")
		}
	case <-ctx.Done():
		t.Fatal("Client did not observe abrupt TCP reset")
	}
	if _, ok := client.CurrentSession(); ok {
		t.Fatal("abruptly reset generation remained CurrentSession")
	}
	_ = client.Stop(ctx)
	select {
	case err := <-peerDone:
		if err != nil {
			t.Fatalf("TCP peer: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("TCP peer did not finish reset")
	}
}

func TestPhase7GracefulEOFClosesBoundGeneration(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	releaseEOF := make(chan struct{})
	peerDone := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			peerDone <- err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

		request, err := readPhase7Frame(conn)
		if err != nil {
			peerDone <- err
			return
		}
		bind, err := wire.DecodeBindRequest(request)
		if err != nil {
			peerDone <- err
			return
		}
		response, err := wire.EncodeBindResponse(request.Header.Sequence, wire.StatusOK, wire.Bind{
			Type: bind.Type, SystemID: "eof-peer",
		})
		if err == nil {
			err = writePhase7Full(conn, response)
		}
		if err != nil {
			peerDone <- err
			return
		}

		<-releaseEOF
		peerDone <- nil
	}()

	closed := make(chan error, 1)
	client, err := NewClient(
		WithClientAddress(listener.Addr().String()),
		WithClientSystemID("eof-client"),
		WithClientOnCloseHandler(func(_ *Session, cause error) {
			closed <- cause
		}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	close(releaseEOF)

	select {
	case cause := <-closed:
		if cause == nil {
			t.Fatal("graceful TCP EOF close notification had no terminal cause")
		}
	case <-ctx.Done():
		t.Fatal("Client did not observe graceful TCP EOF")
	}
	if _, ok := client.CurrentSession(); ok {
		t.Fatal("EOF-closed generation remained CurrentSession")
	}
	_ = client.Stop(ctx)
	select {
	case err := <-peerDone:
		if err != nil {
			t.Fatalf("TCP peer: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("TCP peer did not finish EOF scenario")
	}
}
