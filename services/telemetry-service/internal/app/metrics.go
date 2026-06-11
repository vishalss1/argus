package app

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	TelemetryConsumerMessagesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "telemetry_consumer_messages_total",
		Help: "Total number of telemetry messages processed by the consumer",
	})
	TelemetryConsumerDroppedMessagesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "telemetry_consumer_dropped_messages_total",
		Help: "Total number of telemetry messages dropped by the consumer",
	})
	TelemetryConsumerProcessingFailuresTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "telemetry_consumer_processing_failures_total",
		Help: "Total number of telemetry consumer processing failures",
	})
	TelemetryConsumerDuplicateMessagesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "telemetry_consumer_duplicate_messages_total",
		Help: "Total number of duplicate telemetry messages detected by the consumer",
	})
	TelemetryConsumerMessageProcessingDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "telemetry_consumer_message_processing_duration_seconds",
		Help:    "Total time to process one message in the consumer loop",
		Buckets: prometheus.DefBuckets,
	})
	TelemetryConsumerCommitDurationSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "telemetry_consumer_commit_duration_seconds",
		Help:    "Time spent performing Kafka batch commits",
		Buckets: prometheus.DefBuckets,
	})
	TelemetryStageFetchDurationSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "telemetry_stage_fetch_duration_seconds",
		Help:    "Time spent fetching messages from Kafka",
		Buckets: prometheus.DefBuckets,
	})
)
