package app

import "testing"

func TestPasswordHashAndVerification(t *testing.T) {
	if _, err := hashPassword("short"); err == nil {
		t.Fatal("short password accepted")
	}
	h, err := hashPassword("a secure password")
	if err != nil || !verifyPassword(h, "a secure password") || verifyPassword(h, "wrong password") {
		t.Fatal("password verification")
	}
}

func TestConstantTimeHelper(t *testing.T) {
	if !subtleEqual([]byte("same"), []byte("same")) || subtleEqual([]byte("same"), []byte("nope")) {
		t.Fatal("comparison")
	}
}
