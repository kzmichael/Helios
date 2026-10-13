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

	fleetProxy, err := proxy.NewFleetAPIProxy(cfg.FleetAPIURL)
	if err != nil {
		logger.Error("invalid FLEET_API_URL", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.Handle("/v1/", withMiddleware(fleetProxy, validator, limiter, logger))

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

// withMiddleware wires the chain in the order auth.md requires: strip inbound
// trust headers, authenticate, rate-limit, then assert trusted context onto the
// proxied request. Reordering this chain is a security bug.
func withMiddleware(next http.Handler, validator *auth.Validator, limiter *ratelimit.Limiter, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Step 1 — never trust headers the caller set themselves.
		auth.StripInboundTrust(r)

		// Step 2 — authenticate.
		token := bearerToken(r)
		if token == "" {
			http.Error(w, `{"title":"missing credential","status":401}`, http.StatusUnauthorized)
			return
		}
		claims, err := validator.Validate(r.Context(), token)
		if err != nil {
			logger.Warn("token rejected", "error", err)
			http.Error(w, `{"title":"invalid credential","status":401}`, http.StatusUnauthorized)
			return
		}

		// Step 3 — rate limit, per tenant per tier (limits.md §4).
		decision := limiter.Allow(r.Context(), claims.TenantID, claims.Tier)
		if decision.FailedOpen {
			logger.Warn("rate limiter failed open — redis unreachable", "tenant", claims.TenantID)
		}
		if !decision.Allowed {
			w.Header().Set("Retry-After", "5")
			http.Error(w, `{"title":"rate limit exceeded","status":429}`, http.StatusTooManyRequests)
			return
		}

		// Step 4 — only now assert trusted context onto the outbound request.
		auth.SetAssertedContext(r, claims)

		next.ServeHTTP(w, r)
	})
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) > len(prefix) && h[:len(prefix)] == prefix {
		return h[len(prefix):]
	}
	return ""
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
