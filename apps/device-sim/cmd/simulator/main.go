// Command simulator drives a fleet of simulated devices against ingest. This is
// the platform's own load generator, chaos driver, and cost driver — see
// docs/01_APPLICATION.md §2.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	tracklinev1 "github.com/helios/trackline/gen/go/trackline/v1"
	"github.com/helios/trackline/apps/device-sim/internal/profiles"
	"github.com/helios/trackline/apps/device-sim/internal/simulator"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	ingestAddr := getEnv("INGEST_ADDR", "ingest:9090")
	deviceCount := getEnvInt("DEVICE_COUNT", 500)
	startLat := getEnvFloat("START_LAT", 12.9716)
	startLon := getEnvFloat("START_LON", 77.5946)
	profileMix := strings.Split(getEnv("PROFILE_MIX", "commuter,long-haul,idle,erratic"), ",")

	conn, err := grpc.NewClient(ingestAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error("failed to dial ingest", "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	client := tracklinev1.NewTelemetryIngestClient(conn)

	ctx, cancel := context.WithCancel(context.Background())
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() { <-stop; cancel() }()

	loadedProfiles := make(map[string]profiles.Profile, len(profileMix))
	for _, name := range profileMix {
		p, err := profiles.Load(strings.TrimSpace(name))
		if err != nil {
			logger.Error("failed to load profile", "profile", name, "error", err)
			os.Exit(1)
		}
		loadedProfiles[name] = p
	}

	logger.Info("device-sim starting", "devices", deviceCount, "ingest", ingestAddr, "profiles", profileMix)

	var wg sync.WaitGroup
	for i := 0; i < deviceCount; i++ {
		profileName := profileMix[i%len(profileMix)]
		p := loadedProfiles[strings.TrimSpace(profileName)]
		deviceID := fmt.Sprintf("sim-device-%05d", i)

		// Scatter starting positions ~5km around the given origin so the map
		// isn't a single overlapping dot.
		jitter := func() float64 { return (rand.Float64() - 0.5) * 0.05 }
		v := simulator.NewVehicle(deviceID, p, startLat+jitter(), startLon+jitter(), int64(i))

		wg.Add(1)
		go func() {
			defer wg.Done()
			runVehicle(ctx, client, v, logger)
		}()
	}

	wg.Wait()
	logger.Info("device-sim stopped")
}

// runVehicle owns one device's gRPC stream lifecycle, including reconnecting
// after a transient failure — a single vehicle's connection dropping must never
// take down the rest of the simulated fleet.
func runVehicle(ctx context.Context, client tracklinev1.TelemetryIngestClient, v *simulator.Vehicle, logger *slog.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		stream, err := client.StreamTelemetry(ctx)
		if err != nil {
			logger.Warn("stream open failed, retrying", "device_id", v.DeviceID, "error", err)
			time.Sleep(2 * time.Second)
			continue
		}

		err = v.StreamTo(ctx, func(point *tracklinev1.TelemetryPoint) error {
			return stream.Send(&tracklinev1.StreamTelemetryRequest{
				Points: []*tracklinev1.TelemetryPoint{point},
			})
		})

		if err != nil && ctx.Err() == nil {
			logger.Warn("stream broken, reconnecting", "device_id", v.DeviceID, "error", err)
			time.Sleep(2 * time.Second)
		}
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v, ok := os.LookupEnv(key); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}
