package minio

import "testing"

func TestPublicMinIOEndpointUsesPublicURL(t *testing.T) {
	endpoint, secure, publicURL, err := publicMinIOEndpoint(Config{
		Endpoint:  "localhost:9000",
		PublicURL: "http://192.168.29.222:9000",
		UseSSL:    false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if endpoint != "192.168.29.222:9000" {
		t.Fatalf("expected public endpoint, got %s", endpoint)
	}
	if secure {
		t.Fatal("expected insecure public endpoint")
	}
	if publicURL != "http://192.168.29.222:9000" {
		t.Fatalf("expected normalized public url, got %s", publicURL)
	}
}

func TestPublicMinIOEndpointFallsBackToInternalEndpoint(t *testing.T) {
	endpoint, secure, publicURL, err := publicMinIOEndpoint(Config{
		Endpoint: "minio:9000",
		UseSSL:   true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if endpoint != "minio:9000" || !secure || publicURL != "https://minio:9000" {
		t.Fatalf("unexpected fallback endpoint=%s secure=%t publicURL=%s", endpoint, secure, publicURL)
	}
}

func TestPublicMinIOEndpointRejectsInvalidURL(t *testing.T) {
	_, _, _, err := publicMinIOEndpoint(Config{
		Endpoint:  "localhost:9000",
		PublicURL: "192.168.29.222:9000",
	})
	if err == nil {
		t.Fatal("expected invalid public url error")
	}
}

