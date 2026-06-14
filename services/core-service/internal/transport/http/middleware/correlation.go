package middleware

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/vishalss1/argus/shared/common"
)

// CorrelationID is middleware that extracts/generates a Correlation ID
func CorrelationID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := r.Header.Get("X-Correlation-ID")
		if corrID == "" {
			corrID = uuid.New().String()
		}

		w.Header().Set("X-Correlation-ID", corrID)

		ctx := common.WithCorrelationID(r.Context(), corrID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
