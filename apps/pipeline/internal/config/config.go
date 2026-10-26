// Package config loads the pipeline's runtime configuration from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// AllRoles are the consumer groups this binary can host. Each is a separate
// Kafka consumer group so it can be scaled independently on its own lag.
var AllRoles = []string{
	"registry-sync",
	"position-updater",
	"telemetry-archiver",
	"detector",
	"alert-engine",
	"delivery-recorder",
	"usage-collector",
}

type Config struct {
	Brokers     []string
	DatabaseURL string
	RedisAddr   string
	HealthAddr  string

	Roles []string

	// GroupSuffix is appended to every consumer group id (for side-by-side test runs).
	GroupSuffix string

	RegistryTTL time.Duration
}

func FromEnv() (Config, error) {
	c := Config{
		Brokers:     split(getEnv("KAFKA_BROKERS", "kafka:9092")),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisAddr:   getEnv("REDIS_ADDR", "redis:6379"),
		HealthAddr:  getEnv("HEALTH_ADDR", ":8082"),
		GroupSuffix: os.Getenv("GROUP_SUFFIX"),
		RegistryTTL: time.Duration(getInt("REGISTRY_TTL_SECONDS", 60)) * time.Second,
	}
	if v := os.Getenv("ROLES"); v != "" {
		c.Roles = split(v)
	} else {
		c.Roles = AllRoles
	}

	valid := map[string]bool{}
	for _, r := range AllRoles {
		valid[r] = true
	}
	for _, r := range c.Roles {
		if !valid[r] {
			return c, fmt.Errorf("unknown role %q (valid: %s)", r, strings.Join(AllRoles, ", "))
		}
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL must be set")
	}
	return c, nil
}

func (c Config) HasRole(role string) bool {
	for _, r := range c.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// Group returns the consumer group id for a role.
func (c Config) Group(role string) string { return role + c.GroupSuffix }

func split(v string) []string {
	var out []string
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
