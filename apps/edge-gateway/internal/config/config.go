// Package config loads edge-gateway's runtime configuration from the environment.
// No file ever contains a secret; how a value reaches the environment is a platform
// decision (see apps/_contracts/auth.md §6).
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	// Address this service listens on.
	ListenAddr string

	// Upstream base URL for fleet-api. The gateway proxies all /v1/* traffic here.
	FleetAPIURL        string
	CORSAllowedOrigins []string

	// OIDC issuer and JWKS endpoint used to validate access tokens.
	// See apps/_contracts/auth.md §2.
	OIDCIssuer   string
	JWKSURL      string
	ExpectedAud  string
	JWKSCacheTTL time.Duration

	// Redis backs the rate limiter's token buckets (apps/_contracts/limits.md §4).
	RedisAddr string

	// Bounded clock skew allowance when validating exp/iat/nbf.
	ClockSkew time.Duration
}

func FromEnv() (Config, error) {
	cfg := Config{
		ListenAddr:         getEnv("LISTEN_ADDR", ":8080"),
		FleetAPIURL:        getEnv("FLEET_API_URL", "http://fleet-api:3000"),
		CORSAllowedOrigins: splitCommaSeparated(os.Getenv("CORS_ALLOWED_ORIGINS")),
		OIDCIssuer:         getEnv("OIDC_ISSUER", "https://auth.trackline.local/"),
		JWKSURL:            getEnv("JWKS_URL", "https://auth.trackline.local/.well-known/jwks.json"),
		ExpectedAud:        getEnv("EXPECTED_AUD", "trackline-api"),
		RedisAddr:          getEnv("REDIS_ADDR", "redis:6379"),
		JWKSCacheTTL:       10 * time.Minute,
		ClockSkew:          60 * time.Second,
	}

	if cfg.FleetAPIURL == "" {
		return cfg, fmt.Errorf("FLEET_API_URL must be set")
	}
	return cfg, nil
}

func splitCommaSeparated(value string) []string {
	var values []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	return values
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// getEnvInt is unused today but kept for the next numeric setting — avoids a
// one-off parser being written under time pressure later.
func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
