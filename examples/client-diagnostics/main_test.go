package main

import "testing"

func TestClientDiagnosticsConfigValidation(t *testing.T) {
	for _, args := range [][]string{
		{"-mode=invalid"},
		{"-timeout=0"},
		{"-timeout=2m"},
		{"unexpected"},
	} {
		if err := run(args); err == nil {
			t.Errorf("expected invalid options %v to fail", args)
		}
	}
}
