package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestGenerateRefreshToken_LengthAndEncoding(t *testing.T) {
	tok, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	// 32 bytes encoded as base64url without padding yields 43 ASCII chars.
	if len(tok) != 43 {
		t.Errorf("expected 43-char token, got %d (%q)", len(tok), tok)
	}

	for _, c := range tok {
		if c >= 'a' && c <= 'z' {
			continue
		}

		if c >= 'A' && c <= 'Z' {
			continue
		}

		if c >= '0' && c <= '9' {
			continue
		}

		if c == '-' || c == '_' {
			continue
		}

		t.Errorf("token contains non-base64url char %q in %q", c, tok)
	}
}

func TestGenerateRefreshToken_Unique(t *testing.T) {
	const n = 16

	seen := make(map[string]struct{}, n)
	for range n {
		tok, err := GenerateRefreshToken()
		if err != nil {
			t.Fatalf("GenerateRefreshToken: %v", err)
		}

		if _, dup := seen[tok]; dup {
			t.Fatalf("duplicate token after %d draws: %s", len(seen), tok)
		}

		seen[tok] = struct{}{}
	}
}

func TestHashRefreshToken_Deterministic(t *testing.T) {
	tok := "abc123"
	h1 := HashRefreshToken(tok)

	h2 := HashRefreshToken(tok)
	if h1 != h2 {
		t.Fatalf("hash not deterministic: %s != %s", h1, h2)
	}

	if len(h1) != 64 {
		t.Errorf("expected 64-char hex sha256, got %d (%q)", len(h1), h1)
	}
}

func TestHashRefreshToken_DifferentInput(t *testing.T) {
	if HashRefreshToken("a") == HashRefreshToken("b") {
		t.Fatal("distinct inputs produced identical hashes (collision)")
	}
}

func TestSignerVerifier_Roundtrip(t *testing.T) {
	s := NewSigner("test-secret", "gostartv2-test", 15*time.Minute)
	v := NewVerifier("test-secret", "gostartv2-test")

	uid := uuid.New()
	cid := uuid.New()

	signed, err := s.Sign(uid, cid)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	claims, err := v.Verify(signed)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}

	if claims.UserID != uid {
		t.Errorf("UserID: got %s want %s", claims.UserID, uid)
	}

	if claims.ChainID != cid {
		t.Errorf("ChainID: got %s want %s", claims.ChainID, cid)
	}

	if claims.Issuer != "gostartv2-test" {
		t.Errorf("Issuer: got %q want %q", claims.Issuer, "gostartv2-test")
	}

	if claims.Subject != uid.String() {
		t.Errorf("Subject: got %q want %q", claims.Subject, uid.String())
	}
}

func TestVerifier_Tampered(t *testing.T) {
	s := NewSigner("test-secret", "gostartv2-test", 15*time.Minute)
	v := NewVerifier("test-secret", "gostartv2-test")

	signed, _ := s.Sign(uuid.New(), uuid.New())
	// Flip one character in the signature segment.
	tampered := signed[:len(signed)-1]
	if last := signed[len(signed)-1]; last == 'a' {
		tampered += "b"
	} else {
		tampered += "a"
	}

	if _, err := v.Verify(tampered); err == nil {
		t.Fatal("Verify accepted a tampered token")
	} else if !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("expected ErrTokenInvalid, got %v", err)
	}
}

func TestVerifier_WrongSecret(t *testing.T) {
	s := NewSigner("test-secret", "gostartv2-test", 15*time.Minute)
	v := NewVerifier("different-secret", "gostartv2-test")

	signed, _ := s.Sign(uuid.New(), uuid.New())
	if _, err := v.Verify(signed); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("expected ErrTokenInvalid for wrong-secret token, got %v", err)
	}
}

func TestVerifier_Expired(t *testing.T) {
	s := NewSigner("test-secret", "gostartv2-test", -1*time.Second)
	v := NewVerifier("test-secret", "gostartv2-test")

	signed, _ := s.Sign(uuid.New(), uuid.New())

	_, err := v.Verify(signed)
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("expected ErrTokenExpired, got %v", err)
	}
}

func TestVerifier_WrongIssuer(t *testing.T) {
	s := NewSigner("test-secret", "real-issuer", 15*time.Minute)
	v := NewVerifier("test-secret", "different-issuer")

	signed, _ := s.Sign(uuid.New(), uuid.New())
	if _, err := v.Verify(signed); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("expected ErrTokenInvalid for wrong-issuer token, got %v", err)
	}
}

func TestVerifier_Malformed(t *testing.T) {
	v := NewVerifier("test-secret", "gostartv2-test")
	if _, err := v.Verify("not-a-jwt"); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("expected ErrTokenInvalid for garbage input, got %v", err)
	}

	if _, err := v.Verify(""); !errors.Is(err, ErrTokenInvalid) {
		t.Errorf("expected ErrTokenInvalid for empty input, got %v", err)
	}
}
