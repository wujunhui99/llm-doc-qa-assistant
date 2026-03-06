package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("super-safe-password")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if hash == "" {
		t.Fatalf("expected non-empty hash")
	}
	if !VerifyPassword("super-safe-password", hash) {
		t.Fatalf("expected password verification to pass")
	}
	if VerifyPassword("wrong-password", hash) {
		t.Fatalf("expected password verification to fail for wrong password")
	}
}

func TestHashPasswordRejectsWeakPassword(t *testing.T) {
	if _, err := HashPassword("short"); err == nil {
		t.Fatalf("expected weak password error")
	}
}
