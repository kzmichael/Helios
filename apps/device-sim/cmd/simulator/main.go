// Command simulator drives a fleet of simulated devices against ingest. This is
// the platform's own load generator, chaos driver, and cost driver — see
// docs/01_APPLICATION.md §2.
package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/helios/trackline/apps/device-sim/internal/chaos"
	"github.com/helios/trackline/apps/device-sim/internal/identity"
	"github.com/helios/trackline/apps/device-sim/internal/profiles"
	"github.com/helios/trackline/apps/device-sim/internal/scenario"
	"github.com/helios/trackline/apps/device-sim/internal/simulator"
	tracklinev1 "github.com/helios/trackline/gen/go/trackline/v1"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	ingestAddr := getEnv("INGEST_ADDR", "ingest:9090")
	deviceCount := getEnvInt("DEVICE_COUNT", 500)
	startLat := getEnvFloat("START_LAT", 12.9716)
	startLon := getEnvFloat("START_LON", 77.5946)
	profileMix := strings.Split(getEnv("PROFILE_MIX", "commuter,long-haul,idle,erratic"), ",")
	emissionHz := getEnvFloat("EMISSION_HZ", 0)
	startSpread := time.Duration(getEnvFloat("START_SPREAD_SECONDS", 10) * float64(time.Second))
	fleetPlacement := getEnv("FLEET_PLACEMENT", "true") == "true"
	honorPacing := getEnv("HONOR_BACKPRESSURE", "true") == "true"
	chaosCfg := chaosFromEnv()

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

	if name := os.Getenv("SCENARIO"); name != "" {
		if err := runScenario(ctx, client, name, logger); err != nil {
			logger.Error("scenario failed", "scenario", name, "error", err)
			os.Exit(1)
		}
		return
	}

	logger.Info("device-sim starting", "devices", deviceCount, "ingest", ingestAddr, "profiles", profileMix,
		"emission_hz", emissionHz, "chaos", chaosCfg.Enabled())

	identities := identity.Build(deviceCount, startLat, startLon, fleetPlacement)

	var wg sync.WaitGroup
	for i, id := range identities {
		p := loadedProfiles[strings.TrimSpace(profileMix[i%len(profileMix)])]
		v := simulator.NewVehicle(id, p, int64(i), chaosCfg.For(int64(i)))
		v.EmissionHz = emissionHz

		// Stagger stream opens so a large fleet does not connect in one instant.
		delay := time.Duration(0)
		if deviceCount > 1 {
			delay = startSpread * time.Duration(i) / time.Duration(deviceCount)
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			runVehicle(ctx, client, v, honorPacing, logger)
		}()
	}

	wg.Wait()
	logger.Info("device-sim stopped")
}

func chaosFromEnv() chaos.Config {
	return chaos.Config{
		ClockSkew:      time.Duration(getEnvFloat("CHAOS_CLOCK_SKEW_SECONDS", 0) * float64(time.Second)),
		SkewFraction:   getEnvFloat("CHAOS_SKEW_FRACTION", 0.1),
		PacketLossPct:  getEnvFloat("CHAOS_PACKET_LOSS_PCT", 0),
		DuplicatePct:   getEnvFloat("CHAOS_DUPLICATE_PCT", 0),
		DisconnectMean: time.Duration(getEnvFloat("CHAOS_DISCONNECT_MEAN_SECONDS", 0) * float64(time.Second)),
		Seed:           int64(getEnvInt("CHAOS_SEED", 1)),
	}
}

// runScenario plays one scripted drive and returns when it (and any silence) ends.
func runScenario(ctx context.Context, client tracklinev1.TelemetryIngestClient, name string, logger *slog.Logger) error {
	s, err := scenario.Get(name)
	if err != nil {
		return err
	}
	id, err := s.Identity()
	if err != nil {
		return err
	}

	streamCtx := metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+id.Token)
	stream, err := client.StreamTelemetry(streamCtx)
	if err != nil {
		return err
	}
	go drain(stream, nil)

	logger.Info("scenario starting", "scenario", s.Name, "description", s.Description, "device_id", id.DeviceID, "seconds", int(s.Duration.Seconds()))

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for _, point := range s.Points(id.DeviceID, time.Now()) {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		// Re-stamp at send time so the point is fresh when it arrives.
		point.RecordedAt = time.Now().UnixMilli()
		if err := stream.Send(&tracklinev1.StreamTelemetryRequest{Points: []*tracklinev1.TelemetryPoint{point}}); err != nil {
			return err
		}
	}
	_ = stream.CloseSend()

	if s.SilentFor > 0 {
		logger.Info("scenario silent", "scenario", s.Name, "seconds", int(s.SilentFor.Seconds()))
		select {
		case <-ctx.Done():
		case <-time.After(s.SilentFor):
		}
	}
	logger.Info("scenario finished", "scenario", s.Name)
	return nil
}

// drain reads server responses so the stream's flow-control window never fills,
// and forwards any server-requested pacing to the vehicle.
func drain(stream tracklinev1.TelemetryIngest_StreamTelemetryClient, v *simulator.Vehicle) {
	for {
		resp, err := stream.Recv()
		if err != nil {
			return
		}
		if v != nil {
			v.Pace(resp.GetRetryAfterMs())
		}
	}
}

// runVehicle owns one device's gRPC stream lifecycle, including reconnecting
// after a transient failure — a single vehicle's connection dropping must never
// take down the rest of the simulated fleet.
func runVehicle(ctx context.Context, client tracklinev1.TelemetryIngestClient, v *simulator.Vehicle, honorPacing bool, logger *slog.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		streamCtx, cancelStream := context.WithCancel(ctx)
		if v.Token != "" {
			streamCtx = metadata.AppendToOutgoingContext(streamCtx, "authorization", "Bearer "+v.Token)
		}

		stream, err := client.StreamTelemetry(streamCtx)
		if err != nil {
			cancelStream()
			logger.Warn("stream open failed, retrying", "device_id", v.DeviceID, "error", err)
			time.Sleep(2 * time.Second)
			continue
		}

		var pacer *simulator.Vehicle
		if honorPacing {
			pacer = v
		}
		go drain(stream, pacer)

		err = v.StreamTo(streamCtx, func(point *tracklinev1.TelemetryPoint) error {
			return stream.Send(&tracklinev1.StreamTelemetryRequest{
				Points: []*tracklinev1.TelemetryPoint{point},
			})
		})
		cancelStream()

		if err != nil && ctx.Err() == nil {
			if !errors.Is(err, simulator.ErrInjectedDisconnect) && !errors.Is(err, io.EOF) {
				logger.Warn("stream broken, reconnecting", "device_id", v.DeviceID, "error", err)
			}
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
