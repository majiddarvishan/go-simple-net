package main

import "testing"

func TestConcurrentArgValidation(t *testing.T) {
	for _, tc := range [][]string{
		{"-count", "0"},
		{"-count", "1001"},
		{"-inflight", "0"},
		{"-inflight", "33"},
		{"-timeout", "0"},
		{"unexpected"},
	} {
		if err := run(tc); err == nil {
			t.Errorf("run(%v) expected argument error", tc)
		}
	}
}
