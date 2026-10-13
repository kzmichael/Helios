// Command ingest is the telemetry sink. Devices stream GPS points here over
// gRPC (MQTT arrives in Phase 2); every accepted point is published to Kafka
// and forgotten immediately — this service holds no state of its own.
package main

import (
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"google.golang.org/grpc"

	tracklinev1 "github.com/helios/trackline/gen/go/trackline/v1"
	"github.com/helios/trackline/apps/ingest/internal/grpcserver"
	kafkaproducer "github.com/helios/trackline/apps/ingest/internal/kafka"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	listenAddr := getEnv("LISTEN_ADDR", ":9090")
	brokers := strings.Split(getEnv("KAFKA_BROKERS", "kafka:9092"), ",")

	producer := kafkaproducer.New(brokers)
	defer producer.Close()

	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		logger.Error("failed to listen", "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	tracklinev1.RegisterTelemetryIngestServer(grpcServer, grpcserver.New(producer, logger))

	go func() {
		logger.Info("ingest listening", "addr", listenAddr, "brokers", brokers)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("grpc server stopped", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down gracefully")
	grpcServer.GracefulStop()
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
