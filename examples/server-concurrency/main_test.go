package main

import "testing"

func TestServerConcurrentInvalidConfig(t *testing.T) {
	for _, args := range [][]string{
		{"-max-bound=0"},
		{"-app-workers=0"},
		{"-app-concurrency=0"},
		{"-app-concurrency=33"},
		{"-queue-frames=0"},
		{"-queue-bytes=0"},
		{"-onclose-credits=0"},
		{"garbage"},
	} {
		if err := run(args); err == nil {
			t.Errorf("run(%v) should fail before startup", args)
		}
	}
}
