package main

import "testing"

func TestReconnectRejectsUnboundedWindow(t *testing.T) {
	for _, args := range [][]string{{"-window=0"}, {"-window=-1s"}, {"-window=11m"}, {"unexpected"}} {
		if err := run(args); err == nil {
			t.Errorf("expected invalid %v to fail", args)
		}
	}
}
