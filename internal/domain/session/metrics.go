package session

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	SessionsCreatedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "argus_sessions_created_total",
		Help: "The total number of sessions created",
	})
	SessionsStartedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "argus_sessions_started_total",
		Help: "The total number of sessions started",
	})
	SessionsCompletedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "argus_sessions_completed_total",
		Help: "The total number of sessions successfully completed",
	})
	SessionsFailedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "argus_sessions_failed_total",
		Help: "The total number of sessions marked as failed",
	})
	SessionTransitionErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "argus_session_transition_errors_total",
		Help: "The total number of invalid or concurrent session transition errors",
	})
	SessionRecoveryTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "argus_session_recovery_total",
		Help: "The total number of sessions recovered into Redis hot-state on startup",
	})
)
