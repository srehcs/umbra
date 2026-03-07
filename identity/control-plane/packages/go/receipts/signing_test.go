package receipts

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"testing"
	"time"
)

func TestECDSASignAndVerifyHashHex(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer, err := NewECDSAP256Signer(privateKey, "key://test")
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	signer.now = func() time.Time { return time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC) }

	hashHex := HashBytes([]byte("signed-receipt"))
	meta, err := signer.SignHashHex(hashHex)
	if err != nil {
		t.Fatalf("sign hash: %v", err)
	}
	if meta.Algorithm != SignatureAlgECDSAP256SHA256 {
		t.Fatalf("unexpected algorithm: %s", meta.Algorithm)
	}
	if meta.KeyID != "key://test" {
		t.Fatalf("unexpected key id: %s", meta.KeyID)
	}
	if meta.Signature == "" {
		t.Fatal("expected signature")
	}

	if err := VerifyECDSAP256SignatureHashHex(&privateKey.PublicKey, hashHex, meta.Signature); err != nil {
		t.Fatalf("verify signature: %v", err)
	}
}

func TestVerifyECDSASignatureHashHexFailsOnTamper(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer, err := NewECDSAP256Signer(privateKey, "key://test")
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	hashHex := HashBytes([]byte("signed-receipt"))
	meta, err := signer.SignHashHex(hashHex)
	if err != nil {
		t.Fatalf("sign hash: %v", err)
	}

	tampered := HashBytes([]byte("tampered-receipt"))
	if err := VerifyECDSAP256SignatureHashHex(&privateKey.PublicKey, tampered, meta.Signature); err == nil {
		t.Fatal("expected verify failure for tampered hash")
	}
}

func TestNewSignerFromEnv(t *testing.T) {
	t.Setenv("UMBRA_RECEIPT_SIGNING_ENABLED", "true")
	t.Setenv("UMBRA_RECEIPT_SIGNING_KID", "key://env-test")

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	t.Setenv("UMBRA_RECEIPT_SIGNING_PRIVATE_KEY_PEM", string(pemBytes))

	signer, err := NewSignerFromEnv()
	if err != nil {
		t.Fatalf("new signer from env: %v", err)
	}
	if signer == nil {
		t.Fatal("expected signer")
	}

	hashHex := HashBytes([]byte("env-signed-receipt"))
	meta, err := signer.SignHashHex(hashHex)
	if err != nil {
		t.Fatalf("sign hash: %v", err)
	}
	if meta.Signature == "" {
		t.Fatal("expected signature")
	}
}

func TestValidateSignatureMetadata(t *testing.T) {
	if err := ValidateSignatureMetadata(nil); err != nil {
		t.Fatalf("unexpected nil metadata error: %v", err)
	}
	if err := ValidateSignatureMetadata(&SignatureMetadata{Algorithm: "ECDSA_P256_SHA256"}); err == nil {
		t.Fatal("expected metadata validation error for partial fields")
	}
	if err := ValidateSignatureMetadata(&SignatureMetadata{
		Algorithm: SignatureAlgECDSAP256SHA256,
		KeyID:     "key://1",
		Signature: "abc",
	}); err != nil {
		t.Fatalf("expected valid signature metadata: %v", err)
	}
}

func TestNormalizePEM(t *testing.T) {
	got := string(normalizePEM([]byte("line1\\nline2")))
	if got != "line1\nline2" {
		t.Fatalf("unexpected normalize result: %q", got)
	}
}

func TestNewECDSAP256SignerRejectsNonP256(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	if _, err := NewECDSAP256Signer(privateKey, "key://test"); err == nil {
		t.Fatal("expected P-256 curve validation error")
	}
}

func TestVerifyECDSAP256SignatureHashHexRejectsNonP256PublicKey(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	if err := VerifyECDSAP256SignatureHashHex(&privateKey.PublicKey, HashBytes([]byte("signed-receipt")), "ZmFrZQ=="); err == nil {
		t.Fatal("expected P-256 public key validation error")
	}
}

func TestResolveSignerPolicyFromEnvRequiredNeedsEnabled(t *testing.T) {
	t.Setenv("UMBRA_RECEIPT_SIGNING_ENABLED", "false")
	t.Setenv("UMBRA_RECEIPT_SIGNING_REQUIRED", "true")
	if _, err := ResolveSignerPolicyFromEnv(); err == nil {
		t.Fatal("expected required-without-enabled error")
	}
}

func TestNewSignerFromEnvWithPolicyRequiredMode(t *testing.T) {
	t.Setenv("UMBRA_RECEIPT_SIGNING_ENABLED", "true")
	t.Setenv("UMBRA_RECEIPT_SIGNING_REQUIRED", "true")
	t.Setenv("UMBRA_RECEIPT_SIGNING_KID", "key://required-test")
	t.Setenv("UMBRA_RECEIPT_SIGNING_PRIVATE_KEY_PEM", "")

	_, _, err := NewSignerFromEnvWithPolicy()
	if !errors.Is(err, ErrReceiptSigningUnavailable) {
		t.Fatalf("expected signing unavailable error, got %v", err)
	}
}

func TestNewSignerFromEnvWithPolicyNonRequiredBadConfig(t *testing.T) {
	t.Setenv("UMBRA_RECEIPT_SIGNING_ENABLED", "true")
	t.Setenv("UMBRA_RECEIPT_SIGNING_REQUIRED", "false")
	t.Setenv("UMBRA_RECEIPT_SIGNING_KID", "")
	t.Setenv("UMBRA_RECEIPT_SIGNING_PRIVATE_KEY_PEM", "")

	_, _, err := NewSignerFromEnvWithPolicy()
	if err == nil {
		t.Fatal("expected signer configuration error")
	}
	if errors.Is(err, ErrReceiptSigningUnavailable) {
		t.Fatalf("expected non-required mode to avoid unavailable sentinel, got %v", err)
	}
}
