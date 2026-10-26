// Command pipeline hosts the Kafka consumer groups behind Trackline's read side:
// position-updater, telemetry-archiver, the Phase 0 detector, alert-engine,
// delivery-recorder and usage-collector. Pick which to run with ROLES; each role
// is its own consumer group, so splitting them across deployments later needs no
// code change.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/helios/trackline/apps/pipeline/internal/config"
	"github.com/helios/trackline/apps/pipeline/internal/kafkax"
	"github.com/helios/trackline/apps/pipeline/internal/registry"
	"github.com/helios/trackline/apps/pipeline/internal/roles"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.FromEnv()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("invalid DATABASE_URL", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer rdb.Close()

	prod := kafkax.NewProducer(cfg.Brokers)
	defer prod.Close()

	reg := registry.New(pool, cfg.RegistryTTL)
	deps := roles.Deps{Cfg: cfg, Pool: pool, Redis: rdb, Reg: reg, Prod: prod, Log: logger}

	go serveHealth(cfg.HealthAddr, pool, logger)

	runners := map[string]func(context.Context, roles.Deps) error{
		"registry-sync":      roles.RegistrySync,
		"position-updater":   roles.PositionUpdater,
		"telemetry-archiver": roles.TelemetryArchiver,
		"detector":           roles.DetectorRole,
		"alert-engine":       roles.AlertEngine,
		"delivery-recorder":  roles.DeliveryRecorder,
		"usage-collector":    roles.UsageCollector,
	}

	logger.Info("pipeline starting", "roles", cfg.Roles, "brokers", cfg.Brokers)

	var wg sync.WaitGroup
	for _, name := range cfg.Roles {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := runners[name](ctx, deps); err != nil {
				logger.Error("role failed", "role", name, "error", err)
				stop() // a role that dies takes the process down so the platform restarts it
			}
		}()
	}

	wg.Wait()
	logger.Info("pipeline stopped")
}

// serveHealth exposes /healthz (liveness) and /readyz (Postgres reachable).
func serveHealth(addr string, pool *pgxpool.Pool, logger *slog.Logger) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ready":true}`))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			http.Error(w, `{"ready":false}`, http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"ready":true}`))
	})
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("health server failed", "error", err)
	}
}
