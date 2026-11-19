package main

import "testing"

func TestServerDiagnosticsInvalidConfig(t *testing.T) {
	for _, args := range [][]string{
		{"-panic-ring=0"},
		{"-panic-ring=33"},
		{"-panic-byte-budget=100"},
		{"-panic-byte-budget=2097153"},
		{"extra"},
	} {
		if err := run(args); err == nil {
			t.Errorf("expected invalid %v to fail", args)
		}
	}
}
