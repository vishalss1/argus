package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/vishalss1/argus/core/internal/domain/device"
	"github.com/vishalss1/argus/shared/common"
)

func MTLSAuth(deviceRepo device.Repository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized: mTLS client certificate required")
				return
			}

			cert := r.TLS.PeerCertificates[0]
			subject := cert.Subject

			// Extract device ID from CN=device:{deviceID}
			var deviceID string
			if strings.HasPrefix(subject.CommonName, "device:") {
				deviceID = strings.TrimPrefix(subject.CommonName, "device:")
			}

			// Extract workspace ID from OU=workspace:{workspaceID}
			var workspaceID string
			for _, ou := range subject.OrganizationalUnit {
				if strings.HasPrefix(ou, "workspace:") {
					workspaceID = strings.TrimPrefix(ou, "workspace:")
					break
				}
			}

			if deviceID == "" || workspaceID == "" {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized: invalid client certificate subject structure")
				return
			}

			// Database Lookup
			dev, err := deviceRepo.GetByID(r.Context(), deviceID)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized: device not found")
				return
			}

			// Status Check (reject if disabled or decommissioned)
			status := strings.ToUpper(dev.Status)
			if status == "DISABLED" || status == "DECOMMISSIONED" {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized: device is "+dev.Status)
				return
			}

			// Workspace Verification (assert that device.workspace_id == workspaceID)
			if dev.WorkspaceID == nil || *dev.WorkspaceID != workspaceID {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized: workspace mismatch")
				return
			}

			// Inject authenticated device and workspace details
			ctx := r.Context()
			ctx = context.WithValue(ctx, "mtls_device_id", dev.ID)
			ctx = context.WithValue(ctx, "mtls_workspace_id", workspaceID)
			ctx = common.WithWorkspaceID(ctx, workspaceID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
