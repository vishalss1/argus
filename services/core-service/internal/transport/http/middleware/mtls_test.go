package middleware

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vishalss1/argus/core/internal/domain/device"
)

type mockDeviceRepository struct {
	device.Repository
	getByIDFunc           func(ctx context.Context, id string) (*device.Device, error)
	getByAPIKeyPrefixFunc func(ctx context.Context, prefix string) (*device.Device, error)
}

func (m *mockDeviceRepository) GetByID(ctx context.Context, id string) (*device.Device, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return nil, errors.New("not implemented")
}

func (m *mockDeviceRepository) GetByAPIKeyPrefix(ctx context.Context, prefix string) (*device.Device, error) {
	if m.getByAPIKeyPrefixFunc != nil {
		return m.getByAPIKeyPrefixFunc(ctx, prefix)
	}
	return nil, errors.New("not implemented")
}

func TestDualAuthenticationMiddlewares(t *testing.T) {
	deviceID := "75596200-a44a-43bc-8ff0-56d9e843cbfc"
	workspaceID := "5ab8eeaa-a44a-43bc-8ff0-56d9e843cbfc"
	apiKey := "argus_mockkey123"
	keyPrefix := apiKey[:8]
	keyHash := sha256.Sum256([]byte(apiKey))

	// Base active device template
	activeDevice := device.Device{
		ID:           deviceID,
		Status:       "active",
		WorkspaceID:  &workspaceID,
		APIKeyHash:   keyHash[:],
		APIKeyPrefix: &keyPrefix,
	}

	repo := &mockDeviceRepository{
		getByIDFunc: func(ctx context.Context, id string) (*device.Device, error) {
			if id == deviceID {
				return &activeDevice, nil
			}
			return nil, errors.New("not found")
		},
		getByAPIKeyPrefixFunc: func(ctx context.Context, prefix string) (*device.Device, error) {
			if prefix == keyPrefix {
				return &activeDevice, nil
			}
			return nil, errors.New("not found")
		},
	}

	mtlsMiddleware := MTLSAuth(repo)
	deviceAuthMiddleware := DeviceAuth(repo)

	// Handler that returns 200 OK
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handlerToTest := mtlsMiddleware(deviceAuthMiddleware(nextHandler))

	tests := []struct {
		name           string
		setupTLS       func(req *http.Request)
		setupHeaders   func(req *http.Request)
		deviceStatus   string
		deviceWS       string
		expectedStatus int
	}{
		{
			name: "Success - Valid mTLS and Valid API Key",
			setupTLS: func(req *http.Request) {
				req.TLS = &tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{
						{
							Subject: pkix.Name{
								CommonName:         "device:" + deviceID,
								OrganizationalUnit: []string{"workspace:" + workspaceID},
							},
						},
					},
				}
			},
			setupHeaders: func(req *http.Request) {
				req.Header.Set("X-Device-API-Key", apiKey)
			},
			deviceStatus:   "active",
			deviceWS:       workspaceID,
			expectedStatus: http.StatusOK,
		},
		{
			name:     "Failure - No mTLS Certificate",
			setupTLS: func(req *http.Request) {},
			setupHeaders: func(req *http.Request) {
				req.Header.Set("X-Device-API-Key", apiKey)
			},
			deviceStatus:   "active",
			deviceWS:       workspaceID,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Failure - No API Key",
			setupTLS: func(req *http.Request) {
				req.TLS = &tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{
						{
							Subject: pkix.Name{
								CommonName:         "device:" + deviceID,
								OrganizationalUnit: []string{"workspace:" + workspaceID},
							},
						},
					},
				}
			},
			setupHeaders:   func(req *http.Request) {},
			deviceStatus:   "active",
			deviceWS:       workspaceID,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Failure - Mismatched API Key Device ID",
			setupTLS: func(req *http.Request) {
				req.TLS = &tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{
						{
							Subject: pkix.Name{
								CommonName:         "device:another-device-id",
								OrganizationalUnit: []string{"workspace:" + workspaceID},
							},
						},
					},
				}
			},
			setupHeaders: func(req *http.Request) {
				req.Header.Set("X-Device-API-Key", apiKey)
			},
			deviceStatus:   "active",
			deviceWS:       workspaceID,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Failure - Device Disabled",
			setupTLS: func(req *http.Request) {
				req.TLS = &tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{
						{
							Subject: pkix.Name{
								CommonName:         "device:" + deviceID,
								OrganizationalUnit: []string{"workspace:" + workspaceID},
							},
						},
					},
				}
			},
			setupHeaders: func(req *http.Request) {
				req.Header.Set("X-Device-API-Key", apiKey)
			},
			deviceStatus:   "DISABLED",
			deviceWS:       workspaceID,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Failure - Device Decommissioned",
			setupTLS: func(req *http.Request) {
				req.TLS = &tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{
						{
							Subject: pkix.Name{
								CommonName:         "device:" + deviceID,
								OrganizationalUnit: []string{"workspace:" + workspaceID},
							},
						},
					},
				}
			},
			setupHeaders: func(req *http.Request) {
				req.Header.Set("X-Device-API-Key", apiKey)
			},
			deviceStatus:   "DECOMMISSIONED",
			deviceWS:       workspaceID,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Failure - Workspace Mismatch",
			setupTLS: func(req *http.Request) {
				req.TLS = &tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{
						{
							Subject: pkix.Name{
								CommonName:         "device:" + deviceID,
								OrganizationalUnit: []string{"workspace:mismatched-workspace-id"},
							},
						},
					},
				}
			},
			setupHeaders: func(req *http.Request) {
				req.Header.Set("X-Device-API-Key", apiKey)
			},
			deviceStatus:   "active",
			deviceWS:       workspaceID,
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Update mock device status and workspace
			activeDevice.Status = tt.deviceStatus
			activeDevice.WorkspaceID = &tt.deviceWS

			req := httptest.NewRequest("POST", "/devices/heartbeat", nil)
			tt.setupTLS(req)
			tt.setupHeaders(req)

			rr := httptest.NewRecorder()
			handlerToTest.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d. Response: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}
		})
	}
}
