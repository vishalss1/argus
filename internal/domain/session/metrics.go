package session

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	TelemetryStageCacheHitRatio = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "telemetry_stage_cache_hit_ratio_total",
		Help: "Cache hit/miss counter for workspace and session lookups",
	}, []string{"cache", "result"})
	TelemetryConsumerMessageProcessingDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "telemetry_consumer_message_processing_duration_seconds",
		Help:    "Total time to process one message in the consumer loop",
		Buckets: prometheus.DefBuckets,
	})
	TelemetryConsumerChanDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "telemetry_consumer_chan_depth",
		Help: "Current depth of the msgChan buffer",
	})
	TelemetryConsumerBatchSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "telemetry_consumer_batch_size",
		Help:    "Size of committed batches",
		Buckets: prometheus.ExponentialBuckets(1, 2, 12),
	})
	TelemetryConsumerRedisBatchSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "telemetry_consumer_redis_batch_size",
		Help:    "Size of Redis pipeline batches",
		Buckets: prometheus.ExponentialBuckets(1, 2, 10),
	})

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

	TelemetryConsumerMessagesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "telemetry_consumer_messages_total",
		Help: "Total number of telemetry messages processed by the consumer",
	})
	TelemetryConsumerBatchCommitsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "telemetry_consumer_batch_commits_total",
		Help: "Total number of batch commits executed by the telemetry consumer",
	})
	TelemetryConsumerCommitDurationSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "telemetry_consumer_commit_duration_seconds",
		Help:    "Time spent performing Kafka batch commits",
		Buckets: prometheus.DefBuckets,
	})
	TelemetryRedisPipelineDurationSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "telemetry_redis_pipeline_duration_seconds",
		Help:    "Time spent executing Redis pipelines for telemetry",
		Buckets: prometheus.DefBuckets,
	})
	TelemetryPipelineMessagesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "telemetry_pipeline_messages_total",
		Help: "Total number of messages processed in Redis pipelines",
	})
	SessionArtifactGenerationDurationSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "session_artifact_generation_duration_seconds",
		Help:    "Time spent compiling the session artifact payload",
		Buckets: prometheus.DefBuckets,
	})
	SessionArtifactSizeBytes = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "session_artifact_size_bytes",
		Help:    "Size of the generated session artifact in bytes",
		Buckets: []float64{1024, 10240, 102400, 1048576, 5242880, 10485760, 20971520},
	})
	SessionStopDurationSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "session_stop_duration_seconds",
		Help:    "Total duration of StopSession execution in seconds",
		Buckets: prometheus.DefBuckets,
	})
	RedisPipelineCommandsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "redis_pipeline_commands_total",
		Help: "Total number of commands executed in Redis pipelines",
	})
	RedisPipelineBatchesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "redis_pipeline_batches_total",
		Help: "Total number of Redis pipeline batches executed",
	})
	TelemetryConsumerProcessingFailuresTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "telemetry_consumer_processing_failures_total",
		Help: "Total number of telemetry consumer processing failures",
	})
	TelemetryConsumerDroppedMessagesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "telemetry_consumer_dropped_messages_total",
		Help: "Total number of telemetry messages dropped by the consumer",
	})
	TelemetryConsumerDuplicateMessagesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "telemetry_consumer_duplicate_messages_total",
		Help: "Total number of duplicate telemetry messages detected by the consumer",
	})

	// Worker pool metrics
	TelemetryConsumerWorkerCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "telemetry_consumer_worker_count",
		Help: "Number of active worker goroutines processing messages",
	})
	TelemetryConsumerWorkerQueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "telemetry_consumer_worker_queue_depth",
		Help: "Current depth of the worker input queue",
	})
	TelemetryConsumerWorkerUtilization = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "telemetry_consumer_worker_utilization",
		Help: "Worker utilization percentage (0-100)",
	})
	TelemetryConsumerCommitQueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "telemetry_consumer_commit_queue_depth",
		Help: "Current depth of the commit queue",
	})
	TelemetryConsumerCommitBatchSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "telemetry_consumer_commit_batch_size",
		Help:    "Size of async commit batches",
		Buckets: prometheus.ExponentialBuckets(1, 2, 12),
	})
)

