package neta

import (
	"fmt"
	"testing"
)

type qa005Scenario struct {
	tier        string
	kind        string
	payload     int
	sessions    int
	concurrency uint32
	load        string
}

// Intentionally selected (rather than Cartesian) 8 Core + 16 Full cases.
// All measurements run outside the production hot path on loopback peers.
// "near" is an elevated offered-load setting; qualification must report
// observed saturation instead of assuming that a credit ceiling was reached.
var qa005Scenarios = []qa005Scenario{
	{"Core", "RR", 64, 1, 1, "medium"},
	{"Core", "NoWait", 64, 1, 1, "medium"},
	{"Core", "RR", 4096, 32, 4, "medium"},
	{"Core", "NoWait", 4096, 32, 4, "medium"},
	{"Core", "RR", 65536, 1, 4, "near"},
	{"Core", "NoWait", 65536, 1, 4, "near"},
	{"Core", "RR", 64, 256, 1, "near"},
	{"Core", "NoWait", 4096, 256, 4, "near"},

	{"Full", "RR", 64, 1, 4, "near"},
	{"Full", "NoWait", 64, 1, 4, "near"},
	{"Full", "RR", 65536, 1, 1, "medium"},
	{"Full", "NoWait", 65536, 1, 1, "medium"},
	{"Full", "RR", 64, 32, 1, "medium"},
	{"Full", "NoWait", 64, 32, 1, "medium"},
	{"Full", "RR", 4096, 32, 1, "near"},
	{"Full", "NoWait", 4096, 32, 1, "near"},
	{"Full", "RR", 65536, 32, 4, "near"},
	{"Full", "NoWait", 65536, 32, 4, "near"},
	{"Full", "RR", 64, 256, 4, "medium"},
	{"Full", "NoWait", 64, 256, 4, "medium"},
	{"Full", "RR", 4096, 256, 1, "near"},
	{"Full", "NoWait", 4096, 256, 1, "near"},
	{"Full", "RR", 65536, 256, 1, "medium"},
	{"Full", "NoWait", 65536, 256, 1, "medium"},
}

func qa005Producers(s qa005Scenario) int {
	if s.load == "medium" {
		if s.sessions == 1 {
			return 1
		}
		return s.sessions / 2
	}
	if s.sessions == 1 {
		return 32
	}
	producers := s.sessions * 4
	if producers > 256 {
		producers = 256
	}
	return producers
}

func BenchmarkQA005Qualification(b *testing.B) {
	for _, scenario := range qa005Scenarios {
		name := fmt.Sprintf("%s/%s-%dB-S%d-C%d-%s",
			scenario.tier, scenario.kind, scenario.payload,
			scenario.sessions, scenario.concurrency, scenario.load)
		b.Run(name, func(b *testing.B) {
			respond := scenario.kind == "RR"
			env := newBenchmarkEnv(b, scenario.sessions, scenario.concurrency,
				scenario.concurrency, respond, true)
			payload := benchmarkPayload(scenario.payload)
			producers := qa005Producers(scenario)
			if respond {
				benchmarkRunRequestResponse(b, env, payload, producers)
			} else {
				benchmarkRunSendNoWait(b, env, payload, producers)
			}
		})
	}
}

func TestQA005QualificationScenarioMatrix(t *testing.T) {
	core, full := 0, 0
	names := make(map[string]struct{}, len(qa005Scenarios))
	for _, s := range qa005Scenarios {
		if s.tier == "Core" {
			core++
		} else if s.tier == "Full" {
			full++
		} else {
			t.Fatalf("unexpected tier %q", s.tier)
		}
		if s.payload != 64 && s.payload != 4096 && s.payload != 65536 {
			t.Fatalf("unexpected payload %d", s.payload)
		}
		if s.sessions != 1 && s.sessions != 32 && s.sessions != 256 {
			t.Fatalf("unexpected Session population %d", s.sessions)
		}
		if s.concurrency != 1 && s.concurrency != 4 {
			t.Fatalf("unexpected callback concurrency %d", s.concurrency)
		}
		if s.kind != "RR" && s.kind != "NoWait" {
			t.Fatalf("unexpected mode %q", s.kind)
		}
		if s.load != "medium" && s.load != "near" {
			t.Fatalf("unexpected load %q", s.load)
		}
		key := fmt.Sprintf("%s-%s-%d-%d-%d-%s", s.tier, s.kind,
			s.payload, s.sessions, s.concurrency, s.load)
		if _, duplicate := names[key]; duplicate {
			t.Fatalf("duplicate scenario %q", key)
		}
		names[key] = struct{}{}
	}
	if core != 8 || core+full != 24 {
		t.Fatalf("matrix counts core=%d full=%d, want 8/24 total", core, core+full)
	}
}
