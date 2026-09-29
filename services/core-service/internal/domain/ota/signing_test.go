package ota

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
)

func TestFirmwareSignerSignsChecksum(t *testing.T) {
	keypair, err := GenerateEd25519Keypair("argus-test-v1")
	if err != nil {
		t.Fatal(err)
	}

	signer, err := NewFirmwareSigner(SigningConfig{
		RequireSignatures: true,
		KeyID:             keypair.KeyID,
		PrivateKeyB64:     keypair.PrivateKeyB64,
	})
	if err != nil {
		t.Fatal(err)
	}

	checksum := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	alg, sigB64, keyID, err := signer.SignChecksum(checksum)
	if err != nil {
		t.Fatal(err)
	}
	if alg != SignatureAlgEd25519 {
		t.Fatalf("alg = %q", alg)
	}
	if keyID != "argus-test-v1" {
		t.Fatalf("keyID = %q", keyID)
	}

	publicKeyRaw, err := base64.StdEncoding.DecodeString(keypair.PublicKeyB64)
	if err != nil {
		t.Fatal(err)
	}
	signatureRaw, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		t.Fatal(err)
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKeyRaw), []byte(checksum), signatureRaw) {
		t.Fatal("signature did not verify")
	}
}

func TestFirmwareSignerRequiresKeyWhenEnabled(t *testing.T) {
	_, err := NewFirmwareSigner(SigningConfig{RequireSignatures: true})
	if err == nil {
		t.Fatal("expected missing key error")
	}
}

