package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
)

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken string `json:"access_token"`
}

type Workspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type CreateDeviceRequest struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func main() {
	baseURL := flag.String("base-url", "https://localhost:8080", "Base URL of Argus core-service")
	caCertPath := flag.String("ca-cert", "", "Path to root CA PEM")
	outputPath := flag.String("output", "device.env", "Path for device.env output")
	cleanup := flag.Bool("cleanup", false, "Run cleanup mode (delete test device)")
	deviceIDFlag := flag.String("device-id", "", "Device ID to delete in cleanup mode")
	otaKeysPath := flag.String("ota-keys-output", "ota_keys.env", "Path for ota_keys.env output")
	generateOtaKeys := flag.Bool("generate-ota-keys", false, "Generate Ed25519 OTA keys and write to ota-keys-output")
	flag.Parse()

	// Override from env vars if present
	if envBaseURL := os.Getenv("ARGUS_BASE_URL"); envBaseURL != "" {
		*baseURL = envBaseURL
	}
	if envCACert := os.Getenv("ARGUS_CA_CERT_PATH"); envCACert != "" {
		*caCertPath = envCACert
	}
	if envOutput := os.Getenv("ARGUS_DEVICE_ENV_PATH"); envOutput != "" {
		*outputPath = envOutput
	}
	if envDeviceID := os.Getenv("ARGUS_TEST_DEVICE_ID"); envDeviceID != "" {
		*deviceIDFlag = envDeviceID
	}

	// Create TLS client
	client, err := createHTTPClient(*caCertPath)
	if err != nil {
		fmt.Printf("Error creating HTTP client: %v\n", err)
		os.Exit(1)
	}

	if *generateOtaKeys {
		fmt.Println("Generating Ed25519 OTA signing keys...")
		err := generateAndWriteOTAKeys(*otaKeysPath)
		if err != nil {
			fmt.Printf("Error generating OTA keys: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("OTA signing keys written to %s\n", *otaKeysPath)
		return
	}

	if *cleanup {
		fmt.Println("Running cleanup mode...")
		deviceID := *deviceIDFlag
		if deviceID == "" {
			data, err := os.ReadFile("device_id.txt")
			if err == nil {
				deviceID = string(bytes.TrimSpace(data))
			}
		}
		if deviceID == "" {
			fmt.Println("Error: No device ID provided for cleanup")
			os.Exit(1)
		}

		err := runCleanup(client, *baseURL, deviceID)
		if err != nil {
			fmt.Printf("Cleanup failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Cleanup completed successfully.")
		return
	}

	fmt.Println("Starting device provisioning...")
	err = runProvisioning(client, *baseURL, *outputPath)
	if err != nil {
		fmt.Printf("Provisioning failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Device provisioning completed successfully.")
}

func createHTTPClient(caCertPath string) (*http.Client, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
	}

	if caCertPath != "" {
		caCert, err := os.ReadFile(caCertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA cert from %s: %w", caCertPath, err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate from PEM")
		}
		tlsConfig.RootCAs = caCertPool
	} else {
		// Fallback to system pool or insecure skip if no cert provided in local test
		tlsConfig.InsecureSkipVerify = true
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}, nil
}

func generateAndWriteOTAKeys(path string) error {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}

	randomBytes := make([]byte, 4)
	_, _ = rand.Read(randomBytes)
	keyID := fmt.Sprintf("ci-test-key-%x", randomBytes)

	privB64 := base64.StdEncoding.EncodeToString(priv)
	pubB64 := base64.StdEncoding.EncodeToString(pub)

	content := fmt.Sprintf(
		"OTA_KEY_ID=\"%s\"\nOTA_PRIVATE_KEY_B64=\"%s\"\nOTA_PUBLIC_KEY_B64=\"%s\"\nOTA_REQUIRE_SIGNATURES=\"true\"\n",
		keyID, privB64, pubB64,
	)

	return os.WriteFile(path, []byte(content), 0644)
}

func authenticate(client *http.Client, baseURL string) (string, error) {
	// 1. Register User (ignore conflict)
	regReq := RegisterRequest{
		Email:    "sdktest@ci.local",
		Password: "CiTestPass123!",
		Name:     "SDK CI Test",
	}
	regBody, _ := json.Marshal(regReq)
	resp, err := client.Post(baseURL+"/api/auth/register", "application/json", bytes.NewBuffer(regBody))
	if err == nil {
		resp.Body.Close()
	}

	// 2. Login
	logReq := LoginRequest{
		Email:    "sdktest@ci.local",
		Password: "CiTestPass123!",
	}
	logBody, _ := json.Marshal(logReq)
	resp, err = client.Post(baseURL+"/api/auth/login", "application/json", bytes.NewBuffer(logBody))
	if err != nil {
		return "", fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("login failed with status %d: %s", resp.StatusCode, string(body))
	}

	var loginResp LoginResponse
	err = json.NewDecoder(resp.Body).Decode(&loginResp)
	if err != nil {
		return "", fmt.Errorf("failed to parse login response: %w", err)
	}

	return loginResp.AccessToken, nil
}

func getOrCreateWorkspace(client *http.Client, baseURL, token string) (string, error) {
	// Always create a new unique workspace to ensure the logged-in user has membership
	randomBytes := make([]byte, 4)
	_, _ = rand.Read(randomBytes)
	wsName := fmt.Sprintf("sdk-ci-workspace-%x", randomBytes)

	workReq := map[string]string{"name": wsName}
	workBody, _ := json.Marshal(workReq)
	req, _ := http.NewRequest("POST", baseURL+"/api/workspaces/", bytes.NewBuffer(workBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("create workspace request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create workspace failed status %d: %s", resp.StatusCode, string(body))
	}

	var ws Workspace
	if err := json.NewDecoder(resp.Body).Decode(&ws); err != nil {
		return "", fmt.Errorf("failed to decode workspace: %w", err)
	}

	return ws.ID, nil
}

func runProvisioning(client *http.Client, baseURL, outputPath string) error {
	token, err := authenticate(client, baseURL)
	if err != nil {
		return err
	}

	workspaceID, err := getOrCreateWorkspace(client, baseURL, token)
	if err != nil {
		return err
	}

	_ = os.WriteFile("workspace_id.txt", []byte(workspaceID), 0644)

	// Create Device
	devReq := CreateDeviceRequest{
		Name: "sdk-ci-test-device",
		Type: "esp32",
	}
	devBody, _ := json.Marshal(devReq)
	req, _ := http.NewRequest("POST", baseURL+"/api/devices/", bytes.NewBuffer(devBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Workspace-ID", workspaceID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("create device request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create device failed status %d: %s", resp.StatusCode, string(body))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read device response: %w", err)
	}

	inoContent := string(bodyBytes)

	// Parse .ino content using regexes
	deviceID := extractRegex(inoContent, `const char ARGUS_DEVICE_ID\[\] = "(.*?)";`)
	apiKey := extractRegex(inoContent, `const char ARGUS_API_KEY\[\] = "(.*?)";`)
	httpPort := extractRegex(inoContent, `const uint16_t ARGUS_HTTP_PORT = (\d+);`)
	mqttPort := extractRegex(inoContent, `const uint16_t ARGUS_MQTT_PORT = (\d+);`)
	_ = mqttPort
	otaKeyID := extractRegex(inoContent, `const char ARGUS_OTA_KEY_ID\[\] = "(.*?)";`)
	otaPubKeyB64 := extractRegex(inoContent, `const char ARGUS_OTA_PUBLIC_KEY_B64\[\] = "(.*?)";`)

	rootCA := extractPEM(inoContent, "ARGUS_ROOT_CA")
	deviceCert := extractPEM(inoContent, "ARGUS_DEVICE_CERT")
	deviceKey := extractPEM(inoContent, "ARGUS_DEVICE_PRIVATE_KEY")

	if deviceID == "" || apiKey == "" {
		return fmt.Errorf("failed to extract critical config fields from .ino response")
	}

	_ = os.WriteFile("device_id.txt", []byte(deviceID), 0644)

	// Format PEM strings with literal \n escapes
	escapePEM := func(pem string) string {
		pem = strings.ReplaceAll(pem, "\r", "")
		pem = strings.TrimSpace(pem)
		return strings.ReplaceAll(pem, "\n", "\\n")
	}

	envContent := fmt.Sprintf(
		"ARGUS_DEVICE_ID=\"%s\"\n"+
			"ARGUS_API_KEY=\"%s\"\n"+
			"ARGUS_SERVER_HOST=\"localhost\"\n"+
			"ARGUS_HTTP_PORT=\"%s\"\n"+
			"ARGUS_MQTT_HOST=\"localhost\"\n"+
			"ARGUS_MQTT_PORT=\"1883\"\n"+ // force plaintext MQTT
			"ARGUS_FW_VERSION=\"1.0.0\"\n"+
			"ARGUS_OTA_KEY_ID=\"%s\"\n"+
			"ARGUS_OTA_PUBLIC_KEY_B64=\"%s\"\n"+
			"ARGUS_ROOT_CA_PEM=\"%s\"\n"+
			"ARGUS_DEVICE_CERT_PEM=\"%s\"\n"+
			"ARGUS_DEVICE_PRIVATE_KEY_PEM=\"%s\"\n",
		deviceID, apiKey, httpPort, otaKeyID, otaPubKeyB64,
		escapePEM(rootCA), escapePEM(deviceCert), escapePEM(deviceKey),
	)

	return os.WriteFile(outputPath, []byte(envContent), 0644)
}

func runCleanup(client *http.Client, baseURL, deviceID string) error {
	token, err := authenticate(client, baseURL)
	if err != nil {
		return err
	}

	workspaceID := ""
	data, err := os.ReadFile("workspace_id.txt")
	if err == nil {
		workspaceID = string(bytes.TrimSpace(data))
	}

	if workspaceID == "" {
		workspaceID, err = getOrCreateWorkspace(client, baseURL, token)
		if err != nil {
			return err
		}
	}

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/devices/%s/", baseURL, deviceID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Workspace-ID", workspaceID)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("delete device request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete device failed status %d: %s", resp.StatusCode, string(body))
	}

	// Clean up local files
	_ = os.Remove("device_id.txt")
	_ = os.Remove("workspace_id.txt")

	return nil
}

func extractRegex(content, pattern string) string {
	re := regexp.MustCompile(pattern)
	m := re.FindStringSubmatch(content)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

func extractPEM(content, varName string) string {
	// Match R"EOF( ... )EOF" block
	pattern := fmt.Sprintf(`const char %s\[\]\s*(?:PROGMEM\s*)?=\s*R"EOF\([\r\n]+([\s\S]*?)\)EOF";`, varName)
	re := regexp.MustCompile(pattern)
	m := re.FindStringSubmatch(content)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}
