package adapter

import (
	"testing"
)

func TestHMACTokenSigner_SignAndVerify(t *testing.T) {
	signer := NewHMACTokenSigner("test-secret-key")
	code := "550e8400-e29b-41d4-a716-446655440000"

	token := signer.Sign(code)

	if token == code {
		t.Error("expected token to differ from raw code")
	}

	gotCode, valid := signer.Verify(token)
	if !valid {
		t.Fatal("expected token to be valid")
	}

	if gotCode != code {
		t.Errorf("expected code %q, got %q", code, gotCode)
	}
}

func TestHMACTokenSigner_Verify_InvalidSignature(t *testing.T) {
	signer := NewHMACTokenSigner("test-secret-key")
	code := "550e8400-e29b-41d4-a716-446655440000"

	token := code + ".invalidsignature"

	_, valid := signer.Verify(token)
	if valid {
		t.Error("expected token with bad signature to be invalid")
	}
}

func TestHMACTokenSigner_Verify_NoSeparator(t *testing.T) {
	signer := NewHMACTokenSigner("test-secret-key")

	_, valid := signer.Verify("no-separator-here")
	if valid {
		t.Error("expected token without separator to be invalid")
	}
}

func TestHMACTokenSigner_Verify_DifferentKey(t *testing.T) {
	signer1 := NewHMACTokenSigner("key-one")
	signer2 := NewHMACTokenSigner("key-two")

	code := "550e8400-e29b-41d4-a716-446655440000"
	token := signer1.Sign(code)

	_, valid := signer2.Verify(token)
	if valid {
		t.Error("expected token signed with different key to be invalid")
	}
}

func TestHMACTokenSigner_Verify_TamperedCode(t *testing.T) {
	signer := NewHMACTokenSigner("test-secret-key")
	code := "550e8400-e29b-41d4-a716-446655440000"

	token := signer.Sign(code)

	// Tamper with the code portion
	tampered := "tampered-code" + token[len(code):]

	_, valid := signer.Verify(tampered)
	if valid {
		t.Error("expected tampered token to be invalid")
	}
}
