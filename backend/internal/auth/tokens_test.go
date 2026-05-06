package auth

import "testing"

func TestTokenHashVerification(t *testing.T) {
	token := "paa_test_token"
	hash := HashToken(token)
	if !VerifyTokenHash(token, hash) {
		t.Fatal("expected token to verify")
	}
	if VerifyTokenHash("paa_other", hash) {
		t.Fatal("expected wrong token to fail")
	}
}
