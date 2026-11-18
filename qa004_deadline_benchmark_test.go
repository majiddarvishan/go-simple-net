package neta

import (
	"fmt"
	"net"
	"sort"
	"testing"
	"time"

	"github.com/majiddarvishan/neta-go/internal/wire"
)

// BenchmarkPendingDeadlineDetection uses the real controlLoop terminal-settlement
// point. Unlike Completion callback timing, this isolates scheduler detection
// delay from a possibly blocked user Completion. 2048/8192 populations are
// SYNTHETIC scan fixtures and must not be treated as accepted Request counts
// on a single real Session (hard public maximum 512).
func BenchmarkPendingDeadlineDetection(b *testing.B) {
	for _, population := range []int{32, 128, 512, 2048, 8192} {
		b.Run(fmt.Sprintf("synthetic-pending=%d", population), func(b *testing.B) {
			local, peer := net.Pipe()
			config := defaultSessionConfig()
			config.enquireLinkIdleInterval = 0
			owner := defaultOwnerConfig()
			session := newBoundSessionWithResources(local, config,
				sessionIdentity{role: sessionRoleClient, bindType: wire.BindBiDirectional},
				newOwnerCallbackDispatcher(owner), sessionHandlers{}, newClientResourceLedger(owner))
			if session == nil {
				b.Fatal("benchmark Session rejected")
			}
			defer func() {
				session.mu.Lock()
				session.pending = make(map[uint32]*pendingRequest)
				session.mu.Unlock()
				session.Close()
				_ = peer.Close()
				select {
				case <-session.Done():
				case <-time.After(3 * time.Second):
					b.Error("Session cleanup timeout")
				}
			}()
			settlements := make(chan time.Duration, 1)
			session.mu.Lock()
			guardDeadline := time.Now().Add(time.Hour)
			for i := 1; i < population; i++ {
				session.pending[uint32(i)] = &pendingRequest{
					deadline: guardDeadline, writerClaimed: true,
				}
			}
			session.deadlineSettlementObserver = func(deadline, settledAt time.Time) {
				delay := settledAt.Sub(deadline)
				if delay < 0 {
					delay = 0
				}
				settlements <- delay
			}
			session.mu.Unlock()
			delays := make([]int64, 0, b.N)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sequence := uint32(population + i + 1)
				session.mu.Lock()
				session.pending[sequence] = &pendingRequest{
					deadline:      time.Now().Add(2 * time.Millisecond),
					writerClaimed: true,
					completion:    func(Response, error) {},
				}
				session.mu.Unlock()
				session.signalControl()
				select {
				case delay := <-settlements:
					delays = append(delays, delay.Nanoseconds())
				case <-time.After(5 * time.Second):
					b.Fatal("timeout settlement was not observed")
				}
			}
			b.StopTimer()
			sort.Slice(delays, func(i, j int) bool { return delays[i] < delays[j] })
			b.ReportMetric(benchmarkPercentile(delays, 0.50), "detect-p50-ns")
			b.ReportMetric(benchmarkPercentile(delays, 0.95), "detect-p95-ns")
			b.ReportMetric(benchmarkPercentile(delays, 0.99), "detect-p99-ns")
			b.ReportMetric(float64(len(delays)), "detect-samples")
		})
	}
}

func TestQA004DeadlineSettlementPrecedesBlockedCompletion(t *testing.T) {
	local, peer := net.Pipe()
	owner := defaultOwnerConfig()
	config := defaultSessionConfig()
	config.enquireLinkIdleInterval = 0
	session := newBoundSessionWithResources(local, config,
		sessionIdentity{role: sessionRoleClient, bindType: wire.BindBiDirectional},
		newOwnerCallbackDispatcher(owner), sessionHandlers{}, newClientResourceLedger(owner))
	if session == nil {
		t.Fatal("Session rejected")
	}
	defer func() {
		session.Close()
		_ = peer.Close()
		<-session.Done()
	}()
	settled := make(chan time.Time, 1)
	release := make(chan struct{})
	completionStarted := make(chan struct{})
	deadline := time.Now().Add(20 * time.Millisecond)
	session.mu.Lock()
	session.deadlineSettlementObserver = func(_ time.Time, at time.Time) {
		settled <- at
	}
	session.pending[100] = &pendingRequest{
		deadline: deadline, writerClaimed: true,
		completion: func(Response, error) {
			close(completionStarted)
			<-release
		},
	}
	session.mu.Unlock()
	session.signalControl()
	var settlement time.Time
	select {
	case settlement = <-settled:
	case <-time.After(3 * time.Second):
		t.Fatal("missing terminal settlement")
	}
	select {
	case <-completionStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("Completion not dispatched")
	}
	if settlement.Before(deadline) {
		t.Fatalf("settlement was before absolute deadline: %s", settlement)
	}
	// Completion is deliberately blocked: the recorded terminal settlement
	// must already be available independently of callback completion.
	close(release)
}
