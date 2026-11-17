package neta

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestSendNoWaitDeliveryRouting(t *testing.T) {
	env := newBenchmarkEnv(t, 2, 1, 1, false, false)

	const producers = 8
	var wg sync.WaitGroup
	errs := make(chan error, producers)

	for producer := 0; producer < producers; producer++ {
		session := env.sessions[producer%len(env.sessions)]
		tracker := env.deliveryTracker(session.conn.LocalAddr().String())
		if tracker == nil {
			t.Fatalf("missing delivery tracker for Session %d", producer%len(env.sessions))
		}

		wg.Add(1)
		go func(producer int, session *Session, tracker *benchmarkDeliveryTracker) {
			defer wg.Done()
			sequence, err := session.SendNoWait([]byte{byte(producer)})
			if err != nil {
				errs <- fmt.Errorf("producer %d SendNoWait: %w", producer, err)
				return
			}

			ready := tracker.wait(sequence)
			select {
			case <-ready:
			case <-time.After(5 * time.Second):
				errs <- fmt.Errorf("producer %d sequence %d delivery timed out", producer, sequence)
			}
		}(producer, session, tracker)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	env.checkError(t)
}
