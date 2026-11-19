package main

import "testing"

func TestServerV1RejectsPositionalArgs(t *testing.T) {
	if err := run([]string{"unexpected"}); err == nil {
		t.Fatal("expected argument rejection")
	}
}
