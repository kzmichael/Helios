// Command gateway is the public entrypoint for Trackline. It authenticates,
// rate-limits, and proxies REST traffic to fleet-api. See:
//   - apps/_contracts/auth.md   (identity, tenant context)
//   - apps/_contracts/limits.md (quotas, rate limits, shedding)
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/helios/trackline/apps/edge-gateway/internal/auth"
	"github.com/helios/trackline/apps/edge-gateway/internal/config"
	"github.com/helios/trackline/apps/edge-gateway/internal/proxy"
	"github.com/helios/trackline/apps/edge-gateway/internal/ratelimit"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.FromEnv()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	validator := auth.NewValidator(cfg.JWKSURL, cfg.OIDCIssuer, cfg.ExpectedAud, cfg.JWKSCacheTTL, cfg.ClockSkew)

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer rdb.Close()
	limiter := ratelimit.New(rdb)
	keyVerifier := auth.NewAPIKeyVerifier(cfg.FleetAPIURL)

	fleetProxy, err := proxy.NewFleetAPIProxy(cfg.FleetAPIURL)
	if err != nil {
		logger.Error("invalid FLEET_API_URL", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.Handle("/v1/", withCORS(cfg.CORSAllowedOrigins, withMiddleware(fleetProxy, validator, keyVerifier, limiter, logger)))

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("edge-gateway listening", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	waitForShutdown(srv, logger)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ready":true}`))
}

func waitForShutdown(srv *http.Server, logger *slog.Logger) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}
