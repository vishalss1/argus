package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"
)

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

func main() {
	keyID := flag.String("key-id", "argus-prod-v1", "OTA signing key identifier")
	flag.Parse()

	keypair, err := GenerateEd25519Keypair(*keyID)
	if err != nil {
		log.Fatal(err)
	}

	publicRaw, err := decodeB64(keypair.PublicKeyB64)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("OTA_SIGNING_KEY_ID=%s\n", keypair.KeyID)
	fmt.Printf("OTA_SIGNING_PRIVATE_KEY_B64=%s\n", keypair.PrivateKeyB64)
	fmt.Printf("ARGUS_ED25519_PUBLIC_KEY_B64=%s\n", keypair.PublicKeyB64)
	fmt.Printf("ARGUS_ED25519_PUBLIC_KEY_HEX=%s\n", hex.EncodeToString(publicRaw))
}

func decodeB64(value string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(value)
}
