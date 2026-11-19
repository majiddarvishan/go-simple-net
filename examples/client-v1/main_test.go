package main

import "testing"

func TestV1IdentityBound(t *testing.T) {
	if err := run([]string{"-system-id=12345678901234567890"}); err == nil {
		t.Fatal("20-byte V1 ID must be rejected before any connect")
	}
	if err := run([]string{"-system-id="}); err == nil {
		t.Fatal("empty V1 ID must be rejected")
	}
	if err := run([]string{"extra"}); err == nil {
		t.Fatal("positional args must fail")
	}
}
