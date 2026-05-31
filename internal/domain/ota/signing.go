package ota

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

const SignatureAlgEd25519 = "ed25519"

type SigningConfig struct {
	RequireSignatures bool
	KeyID             string
	PrivateKeyB64     string
}

type FirmwareSigner struct {
	requireSignatures bool
	keyID             string
	privateKey        ed25519.PrivateKey
}

func NewFirmwareSigner(cfg SigningConfig) (*FirmwareSigner, error) {
	keyID := strings.TrimSpace(cfg.KeyID)
	privateKeyB64 := strings.TrimSpace(cfg.PrivateKeyB64)
	if privateKeyB64 == "" {
		if cfg.RequireSignatures {
			return nil, errors.New("OTA_SIGNING_PRIVATE_KEY_B64 is required when OTA_REQUIRE_SIGNATURES=true")
		}
		return &FirmwareSigner{requireSignatures: false}, nil
	}
	if keyID == "" {
		return nil, errors.New("OTA_SIGNING_KEY_ID is required when OTA signing key is configured")
	}

	raw, err := base64.StdEncoding.DecodeString(privateKeyB64)
	if err != nil {
		return nil, fmt.Errorf("decode OTA signing private key: %w", err)
	}

	var privateKey ed25519.PrivateKey
	switch len(raw) {
	case ed25519.SeedSize:
		privateKey = ed25519.NewKeyFromSeed(raw)
	case ed25519.PrivateKeySize:
		privateKey = ed25519.PrivateKey(raw)
	default:
		return nil, fmt.Errorf("OTA signing private key must be %d-byte seed or %d-byte private key", ed25519.SeedSize, ed25519.PrivateKeySize)
	}

	return &FirmwareSigner{
		requireSignatures: cfg.RequireSignatures,
		keyID:             keyID,
		privateKey:        privateKey,
	}, nil
}

func (s *FirmwareSigner) RequireSignatures() bool {
	return s != nil && s.requireSignatures
}

func (s *FirmwareSigner) SignChecksum(checksumHex string) (signatureAlg string, signatureB64 string, signingKeyID string, err error) {
	if s == nil || len(s.privateKey) == 0 {
		if s != nil && s.requireSignatures {
			return "", "", "", errors.New("OTA signatures are required but no signing key is configured")
		}
		return "", "", "", nil
	}

	checksumHex = strings.ToLower(strings.TrimSpace(checksumHex))
	if len(checksumHex) != 64 {
		return "", "", "", errors.New("checksum_sha256 must be a 64-character hex string before signing")
	}

	signature := ed25519.Sign(s.privateKey, []byte(checksumHex))
	return SignatureAlgEd25519, base64.StdEncoding.EncodeToString(signature), s.keyID, nil
}

type GeneratedKeypair struct {
	KeyID         string
	PrivateKeyB64 string
	PublicKeyB64  string
}

func GenerateEd25519Keypair(keyID string) (*GeneratedKeypair, error) {
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return nil, errors.New("key id is required")
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &GeneratedKeypair{
		KeyID:         keyID,
		PrivateKeyB64: base64.StdEncoding.EncodeToString(privateKey),
		PublicKeyB64:  base64.StdEncoding.EncodeToString(publicKey),
	}, nil
}
