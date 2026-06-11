package common

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// ActiveSessions tracks active session counts (Core Service)
	ActiveSessions = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "argus_active_sessions",
		Help: "Current number of active telemetry aggregation sessions",
	})

	// ConnectedDevices tracks the number of registered/active devices (Core Service)
	ConnectedDevices = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "argus_connected_devices",
		Help: "Current number of active/connected devices in the registry",
	})

	// WSConnections tracks real-time WebSocket client count (Core Service)
	WSConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "argus_websocket_connections",
		Help: "Current number of active WebSocket connections on the gateway",
	})

	// GRPCRequestsTotal tracks total inter-service gRPC requests made/received
	GRPCRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "argus_grpc_requests_total",
		Help: "Total number of gRPC requests handled",
	}, []string{"service", "method", "status"})

	// GRPCRequestDuration tracks latency of gRPC request execution
	GRPCRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "argus_grpc_request_duration_seconds",
		Help:    "Duration of gRPC request execution in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"service", "method"})



	// AIQueryDuration tracks AI RAG queries latency (Telemetry Service)
	AIQueryDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "argus_ai_query_duration_seconds",
		Help:    "Latency of AI query execution in seconds",
		Buckets: []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 20.0},
	})

	// RuleEvaluationDuration tracks telemetry rule-engine evaluation latency (Telemetry Service)
	RuleEvaluationDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "argus_rule_evaluation_duration_seconds",
		Help:    "Latency of rule engine evaluation runs in seconds",
		Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
	})

	// RedisLockContention tracks lock acquisition contention events (both services)
	RedisLockContention = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "argus_redis_lock_contention_total",
		Help: "Total number of failed or contested Redis distributed lock acquisitions",
	}, []string{"lock_name"})
)
