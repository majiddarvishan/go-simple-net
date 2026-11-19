package main

import "testing"

func TestAllowlistValidation(t *testing.T) {
	for _, raw := range []string{"", "one,", ",one", "a,a", "123456789012345678901"} {
		if _, err := parseAllowlist(raw); err == nil {
			t.Errorf("expected failure for %q", raw)
		}
	}
	allowed, err := parseAllowlist("one,two")
	if err != nil || len(allowed) != 2 {
		t.Fatalf("unexpected allowlist: %v %v", allowed, err)
	}
}

func TestBindValidatorConfigErrors(t *testing.T) {
	for _, args := range [][]string{{"-protocol=v3"}, {"-allow=a,a"}, {"-protocol=v1", "-allow=12345678901234567890"}, {"garbage"}} {
		if err := run(args); err == nil {
			t.Errorf("expected failure for %v", args)
		}
	}
}
