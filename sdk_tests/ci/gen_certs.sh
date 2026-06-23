#!/usr/bin/env bash
set -euo pipefail
CERT_DIR="${1:-certs}"
mkdir -p "$CERT_DIR"

# Root CA (EC P-256)
openssl ecparam -genkey -name prime256v1 -noout -out "$CERT_DIR/root-ca.key"
openssl req -new -x509 -key "$CERT_DIR/root-ca.key" -out "$CERT_DIR/root-ca.pem" \
    -days 365 -subj "/CN=Argus CI Root CA"

# Server cert CSR
openssl ecparam -genkey -name prime256v1 -noout -out "$CERT_DIR/privkey.pem"
openssl req -new -key "$CERT_DIR/privkey.pem" -out /tmp/server.csr \
    -subj "/CN=localhost"

# Sign server cert with root CA + SAN
openssl x509 -req -in /tmp/server.csr \
    -CA "$CERT_DIR/root-ca.pem" -CAkey "$CERT_DIR/root-ca.key" \
    -CAcreateserial -out "$CERT_DIR/fullchain.pem" -days 365 \
    -extfile <(printf 'subjectAltName=DNS:localhost,IP:127.0.0.1\nbasicConstraints=CA:FALSE\nkeyUsage=digitalSignature,keyEncipherment\nextendedKeyUsage=serverAuth')

# ca.pem = copy of root CA
cp "$CERT_DIR/root-ca.pem" "$CERT_DIR/ca.pem"

# Print fingerprint
FINGERPRINT=$(openssl x509 -in "$CERT_DIR/fullchain.pem" -noout -fingerprint -sha256 | sed 's/://g' | cut -d= -f2)
echo "Server cert SHA256 fingerprint: $FINGERPRINT"
echo "Certificates generated in $CERT_DIR/"
