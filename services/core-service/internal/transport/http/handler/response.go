package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/vishalss1/argus/core/internal/transport/http/dto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, dto.ErrorResponse{Error: message})
}

func isGrpcOrBreakerError(err error) bool {
	if err == nil {
		return false
	}
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.Unavailable, codes.DeadlineExceeded, codes.Internal, codes.ResourceExhausted:
			return true
		}
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "circuit breaker") ||
		strings.Contains(errStr, "open") ||
		strings.Contains(errStr, "unavailable") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "desc = transport") ||
		strings.Contains(errStr, "503")
}

