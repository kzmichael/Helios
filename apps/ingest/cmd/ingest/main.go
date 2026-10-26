// Command ingest is the telemetry sink. Devices stream GPS points here over
// gRPC (MQTT arrives in Phase 2); every accepted point is published to Kafka
// and forgotten immediately — this service holds no state of its own.
package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"

	"github.com/helios/trackline/apps/ingest/internal/backpressure"
	"github.com/helios/trackline/apps/ingest/internal/grpcserver"
	kafkaproducer "github.com/helios/trackline/apps/ingest/internal/kafka"
	"github.com/helios/trackline/apps/ingest/internal/mqtt"
	"github.com/helios/trackline/apps/ingest/internal/registry"
	tracklinev1 "github.com/helios/trackline/gen/go/trackline/v1"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	listenAddr := getEnv("LISTEN_ADDR", ":9090")
	brokers := strings.Split(getEnv("KAFKA_BROKERS", "kafka:9092"), ",")

	producer := kafkaproducer.New(brokers)
	defer producer.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Everything that needs shared state is opt-in: without REDIS_ADDR ingest is the
	// plain, unauthenticated, in-memory sink it has always been.
	opts := grpcserver.Options{}
	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		rdb := redis.NewClient(&redis.Options{Addr: addr})
		defer rdb.Close()
		opts.Registry = registry.New(rdb)
		opts.Dedupe = grpcserver.NewRedisDeduper(rdb)
		opts.AuthRequired = getEnv("DEVICE_AUTH", "off") == "required"
	}
	if opts.AuthRequired && opts.Registry == nil {
		logger.Error("DEVICE_AUTH=required needs REDIS_ADDR (the device registry)")
		os.Exit(1)
	}
	if getEnv("LOAD_SHEDDING", "off") == "on" {
		opts.Shedder = backpressure.New(getEnvInt("SHED_CAPACITY", 2000))
		go opts.Shedder.Run(ctx.Done())
	}

	server := grpcserver.New(producer, logger, opts)

	if url := os.Getenv("MQTT_BROKER_URL"); url != "" {
		go func() {
			err := mqtt.Run(ctx, mqtt.Config{
				BrokerURL:   url,
				ClientID:    getEnv("MQTT_CLIENT_ID", "trackline-ingest"),
				Username:    os.Getenv("MQTT_USERNAME"),
				Password:    os.Getenv("MQTT_PASSWORD"),
				SharedGroup: os.Getenv("MQTT_SHARED_GROUP"),
			}, server, logger)
			if err != nil {
				logger.Error("mqtt transport stopped", "error", err)
			}
		}()
	}

	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		logger.Error("failed to listen", "error", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	tracklinev1.RegisterTelemetryIngestServer(grpcServer, server)

	go func() {
		logger.Info("ingest listening", "addr", listenAddr, "brokers", brokers,
			"registry", opts.Registry != nil, "device_auth", opts.AuthRequired, "shedding", opts.Shedder != nil,
			"mqtt", os.Getenv("MQTT_BROKER_URL") != "")
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("grpc server stopped", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	logger.Info("shutting down gracefully")
	grpcServer.GracefulStop()
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
