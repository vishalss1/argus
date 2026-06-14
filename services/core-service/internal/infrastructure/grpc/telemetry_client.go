package grpc

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/vishalss1/argus/shared/common"
	pb "github.com/vishalss1/argus/shared/proto/telemetry"
)

type TelemetryClient struct {
	conn   *grpc.ClientConn
	client pb.TelemetryIntelligenceServiceClient
}

var cb *gobreaker.CircuitBreaker

func init() {
	cb = gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name: "Core-to-Telemetry-gRPC",
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 10 && failureRatio >= 0.5
		},
		Timeout: 5 * time.Second,
	})
}

func clientInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	// 1. Inject correlation ID
	if corrID, ok := common.GetCorrelationID(ctx); ok {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-correlation-id", corrID)
	}

	// 2. Track metrics & wrap in circuit breaker
	startTime := time.Now()
	_, err := cb.Execute(func() (interface{}, error) {
		err := invoker(ctx, method, req, reply, cc, opts...)
		return nil, err
	})

	if err != nil && err == gobreaker.ErrOpenState {
		err = status.Error(codes.Unavailable, "circuit breaker is open")
	}

	duration := time.Since(startTime).Seconds()
	statusVal := "success"
	if err != nil {
		statusVal = "failure"
	}
	common.GRPCRequestsTotal.WithLabelValues("telemetry-client", method, statusVal).Inc()
	common.GRPCRequestDuration.WithLabelValues("telemetry-client", method).Observe(duration)

	return err
}

func NewTelemetryClient(addr string) (*TelemetryClient, error) {
	conn, err := grpc.Dial(addr, 
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(clientInterceptor),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc dial telemetry service at %s: %w", addr, err)
	}

	// Monitor connection state in the background
	go func() {
		state := conn.GetState()
		log.Printf("[gRPC Client] Telemetry Service connection state: %s", state)
		ctx := context.Background()
		for {
			if !conn.WaitForStateChange(ctx, state) {
				return
			}
			newState := conn.GetState()
			log.Printf("[gRPC Client] Telemetry Service connection state changed: %s -> %s", state, newState)
			state = newState
		}
	}()

	client := pb.NewTelemetryIntelligenceServiceClient(conn)
	return &TelemetryClient{
		conn:   conn,
		client: client,
	}, nil
}

func (c *TelemetryClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *TelemetryClient) Client() pb.TelemetryIntelligenceServiceClient {
	return c.client
}
