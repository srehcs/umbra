package receipts

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	SignatureAlgECDSAP256SHA256 = "ECDSA_P256_SHA256"
	SignatureAlgRSAPSSSHA256    = "RSASSA_PSS_SHA_256"
)

var ErrReceiptSigningUnavailable = errors.New("receipt signing unavailable")

type SignerPolicy struct {
	Enabled  bool
	Required bool
}

type SignatureMetadata struct {
	Algorithm string
	KeyID     string
	Signature string
	SignedAt  *time.Time
}

type Signer interface {
	SignHashHex(hashHex string) (SignatureMetadata, error)
}

type ECDSAP256Signer struct {
	privateKey *ecdsa.PrivateKey
	keyID      string
	now        func() time.Time
}

func NewECDSAP256Signer(privateKey *ecdsa.PrivateKey, keyID string) (*ECDSAP256Signer, error) {
	if privateKey == nil {
		return nil, errors.New("private key required")
	}
	if privateKey.Curve == nil || privateKey.Curve.Params() == nil || privateKey.Curve.Params().Name != elliptic.P256().Params().Name {
		return nil, errors.New("private key must use P-256 curve")
	}
	if strings.TrimSpace(keyID) == "" {
		return nil, errors.New("key id required")
	}
	return &ECDSAP256Signer{
		privateKey: privateKey,
		keyID:      strings.TrimSpace(keyID),
		now:        time.Now,
	}, nil
}

func NewECDSAP256SignerFromPEM(privateKeyPEM []byte, keyID string) (*ECDSAP256Signer, error) {
	privateKey, err := ParseECDSAPrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		return nil, err
	}
	return NewECDSAP256Signer(privateKey, keyID)
}

func (s *ECDSAP256Signer) SignHashHex(hashHex string) (SignatureMetadata, error) {
	hashBytes, err := decodeHashHex(hashHex)
	if err != nil {
		return SignatureMetadata{}, err
	}
	sig, err := ecdsa.SignASN1(rand.Reader, s.privateKey, hashBytes)
	if err != nil {
		return SignatureMetadata{}, err
	}
	signedAt := s.now().UTC()
	return SignatureMetadata{
		Algorithm: SignatureAlgECDSAP256SHA256,
		KeyID:     s.keyID,
		Signature: base64.StdEncoding.EncodeToString(sig),
		SignedAt:  &signedAt,
	}, nil
}

func ParseECDSAPrivateKeyFromPEM(privateKeyPEM []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(normalizePEM(privateKeyPEM))
	if block == nil {
		return nil, errors.New("invalid private key pem")
	}

	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	key, ok := keyAny.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not ecdsa")
	}
	return key, nil
}

func ParseECDSAPublicKeyFromPEM(publicKeyPEM []byte) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode(normalizePEM(publicKeyPEM))
	if block == nil {
		return nil, errors.New("invalid public key pem")
	}

	if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
		key, ok := cert.PublicKey.(*ecdsa.PublicKey)
		if !ok {
			return nil, errors.New("certificate public key is not ecdsa")
		}
		return key, nil
	}

	keyAny, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	key, ok := keyAny.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("public key is not ecdsa")
	}
	return key, nil
}

func VerifyECDSAP256SignatureHashHex(publicKey *ecdsa.PublicKey, hashHex, signatureB64 string) error {
	if publicKey == nil {
		return errors.New("public key required")
	}
	if publicKey.Curve == nil || publicKey.Curve.Params() == nil || publicKey.Curve.Params().Name != elliptic.P256().Params().Name {
		return errors.New("public key must use P-256 curve")
	}
	hashBytes, err := decodeHashHex(hashHex)
	if err != nil {
		return err
	}
	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(signatureB64))
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	if !ecdsa.VerifyASN1(publicKey, hashBytes, signature) {
		return errors.New("signature verification failed")
	}
	return nil
}

func ValidateSignatureMetadata(meta *SignatureMetadata) error {
	if meta == nil {
		return nil
	}
	meta.Algorithm = strings.TrimSpace(meta.Algorithm)
	meta.KeyID = strings.TrimSpace(meta.KeyID)
	meta.Signature = strings.TrimSpace(meta.Signature)

	if meta.Algorithm == "" && meta.KeyID == "" && meta.Signature == "" && meta.SignedAt == nil {
		return nil
	}
	if meta.Algorithm == "" || meta.KeyID == "" || meta.Signature == "" {
		return errors.New("signature metadata requires algorithm, key id, and signature")
	}
	if !isSupportedSignatureAlgorithm(meta.Algorithm) {
		return fmt.Errorf("unsupported signature algorithm: %s", meta.Algorithm)
	}
	return nil
}

func NewSignerFromEnv() (Signer, error) {
	signer, _, err := NewSignerFromEnvWithPolicy()
	return signer, err
}

func NewSignerFromEnvWithPolicy() (Signer, SignerPolicy, error) {
	policy, err := ResolveSignerPolicyFromEnv()
	if err != nil {
		return nil, policy, wrapSigningUnavailable(err)
	}
	if !policy.Enabled {
		return nil, policy, nil
	}

	keyID := strings.TrimSpace(os.Getenv("UMBRA_RECEIPT_SIGNING_KID"))
	privateKeyPEM := []byte(os.Getenv("UMBRA_RECEIPT_SIGNING_PRIVATE_KEY_PEM"))
	if keyID == "" {
		err := errors.New("UMBRA_RECEIPT_SIGNING_KID required when signing is enabled")
		if policy.Required {
			return nil, policy, wrapSigningUnavailable(err)
		}
		return nil, policy, err
	}
	if len(strings.TrimSpace(string(privateKeyPEM))) == 0 {
		err := errors.New("UMBRA_RECEIPT_SIGNING_PRIVATE_KEY_PEM required when signing is enabled")
		if policy.Required {
			return nil, policy, wrapSigningUnavailable(err)
		}
		return nil, policy, err
	}

	signer, err := NewECDSAP256SignerFromPEM(privateKeyPEM, keyID)
	if err != nil {
		if policy.Required {
			return nil, policy, wrapSigningUnavailable(err)
		}
		return nil, policy, err
	}
	return signer, policy, nil
}

func ResolveSignerPolicyFromEnv() (SignerPolicy, error) {
	policy := SignerPolicy{
		Enabled:  envEnabled("UMBRA_RECEIPT_SIGNING_ENABLED"),
		Required: envEnabled("UMBRA_RECEIPT_SIGNING_REQUIRED"),
	}
	if policy.Required && !policy.Enabled {
		return policy, errors.New("UMBRA_RECEIPT_SIGNING_REQUIRED requires UMBRA_RECEIPT_SIGNING_ENABLED=true")
	}
	return policy, nil
}

func IsReceiptSigningUnavailable(err error) bool {
	return errors.Is(err, ErrReceiptSigningUnavailable)
}

func wrapSigningUnavailable(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %v", ErrReceiptSigningUnavailable, err)
}

func isSupportedSignatureAlgorithm(alg string) bool {
	switch strings.TrimSpace(alg) {
	case SignatureAlgECDSAP256SHA256, SignatureAlgRSAPSSSHA256:
		return true
	default:
		return false
	}
}

func decodeHashHex(hashHex string) ([]byte, error) {
	hashHex = strings.TrimSpace(hashHex)
	if hashHex == "" {
		return nil, errors.New("hash required")
	}
	hashBytes, err := hex.DecodeString(hashHex)
	if err != nil {
		return nil, fmt.Errorf("invalid hash hex: %w", err)
	}
	if len(hashBytes) != 32 {
		return nil, errors.New("hash must be sha256 (32 bytes)")
	}
	return hashBytes, nil
}

func normalizePEM(in []byte) []byte {
	// Support env-var encoded PEM with literal "\n" sequences.
	return []byte(strings.ReplaceAll(string(in), `\n`, "\n"))
}

func envEnabled(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
