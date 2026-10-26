// Package registry is the pipeline's read-through view of who owns what: which
// vehicle and tenant a device belongs to, and which geofences apply. Consumers
// call Lookup on the hot path; it refreshes on a TTL and, at most every few
// seconds, on a miss (a device provisioned after the last refresh).
package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/helios/trackline/apps/pipeline/internal/geo"
)

type Device struct {
	DeviceID     string
	VehicleID    string
	TenantID     string
	Tier         string
	FleetID      string
	Registration string
	MaxHz        float64
	SpeedLimit   float64
	TokenHash    string
	LastSeenAt   time.Time
}

type Geofence struct {
	ID        string
	TenantID  string
	FleetID   string // empty: applies to every fleet of the tenant
	Name      string
	Kind      string
	DwellSecs int
	Boundary  geo.Polygon
}

type snapshot struct {
	byDevice   map[string]Device
	byVehicle  map[string]Device
	geofences  map[string][]Geofence // by tenant
	geofenceBy map[string]Geofence
}

type Registry struct {
	pool *pgxpool.Pool
	ttl  time.Duration

	mu          sync.RWMutex
	snap        snapshot
	loadedAt    time.Time
	lastMissTry time.Time
}

func New(pool *pgxpool.Pool, ttl time.Duration) *Registry {
	return &Registry{pool: pool, ttl: ttl}
}

// Refresh reloads everything from Postgres.
func (r *Registry) Refresh(ctx context.Context) error {
	s := snapshot{
		byDevice:   map[string]Device{},
		byVehicle:  map[string]Device{},
		geofences:  map[string][]Geofence{},
		geofenceBy: map[string]Geofence{},
	}

	rows, err := r.pool.Query(ctx, `
		SELECT d.id::text, v.id::text, v.tenant_id::text, t.tier, v.fleet_id::text,
		       v.registration, t.max_hz::float8, v.speed_limit_kph::float8,
		       d.auth_token_hash, coalesce(d.last_seen_at, 'epoch'::timestamptz)
		FROM device d
		JOIN vehicle v ON v.id = d.vehicle_id
		JOIN tenant  t ON t.id = v.tenant_id
		WHERE d.status <> 'decommissioned' AND t.status = 'active'`)
	if err != nil {
		return fmt.Errorf("registry: load devices: %w", err)
	}
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.DeviceID, &d.VehicleID, &d.TenantID, &d.Tier, &d.FleetID,
			&d.Registration, &d.MaxHz, &d.SpeedLimit, &d.TokenHash, &d.LastSeenAt); err != nil {
			rows.Close()
			return fmt.Errorf("registry: scan device: %w", err)
		}
		s.byDevice[d.DeviceID] = d
		s.byVehicle[d.VehicleID] = d
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	grows, err := r.pool.Query(ctx, `
		SELECT id::text, tenant_id::text, coalesce(fleet_id::text, ''), name, kind,
		       dwell_threshold_s, boundary::text
		FROM geofence WHERE enabled`)
	if err != nil {
		return fmt.Errorf("registry: load geofences: %w", err)
	}
	defer grows.Close()
	for grows.Next() {
		var g Geofence
		var boundary string
		if err := grows.Scan(&g.ID, &g.TenantID, &g.FleetID, &g.Name, &g.Kind, &g.DwellSecs, &boundary); err != nil {
			return fmt.Errorf("registry: scan geofence: %w", err)
		}
		poly, err := parsePolygon(boundary)
		if err != nil {
			continue // a malformed boundary must not take the registry down
		}
		g.Boundary = poly
		s.geofences[g.TenantID] = append(s.geofences[g.TenantID], g)
		s.geofenceBy[g.ID] = g
	}
	if err := grows.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	r.snap = s
	r.loadedAt = time.Now()
	r.mu.Unlock()
	return nil
}

func parsePolygon(raw string) (geo.Polygon, error) {
	var gj struct {
		Type        string         `json:"type"`
		Coordinates [][][2]float64 `json:"coordinates"`
	}
	if err := json.Unmarshal([]byte(raw), &gj); err != nil {
		return nil, err
	}
	if gj.Type != "Polygon" || len(gj.Coordinates) == 0 {
		return nil, fmt.Errorf("not a polygon")
	}
	return geo.Polygon(gj.Coordinates[0]), nil
}

func (r *Registry) stale() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return time.Since(r.loadedAt) > r.ttl
}

// maybeRefresh reloads when the snapshot is past its TTL, or (force) on a lookup
// miss, rate-limited so a stream of unknown devices cannot hammer Postgres.
func (r *Registry) maybeRefresh(ctx context.Context, force bool) {
	if !r.stale() {
		if !force {
			return
		}
		r.mu.Lock()
		if time.Since(r.lastMissTry) < 5*time.Second {
			r.mu.Unlock()
			return
		}
		r.lastMissTry = time.Now()
		r.mu.Unlock()
	}
	_ = r.Refresh(ctx) // on failure keep serving the previous snapshot
}

// Lookup resolves a device id.
func (r *Registry) Lookup(ctx context.Context, deviceID string) (Device, bool) {
	r.maybeRefresh(ctx, false)
	r.mu.RLock()
	d, ok := r.snap.byDevice[deviceID]
	r.mu.RUnlock()
	if ok {
		return d, true
	}
	r.maybeRefresh(ctx, true)
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok = r.snap.byDevice[deviceID]
	return d, ok
}

// Vehicle resolves a vehicle id to its device record.
func (r *Registry) Vehicle(ctx context.Context, vehicleID string) (Device, bool) {
	r.maybeRefresh(ctx, false)
	r.mu.RLock()
	d, ok := r.snap.byVehicle[vehicleID]
	r.mu.RUnlock()
	if ok {
		return d, true
	}
	r.maybeRefresh(ctx, true)
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok = r.snap.byVehicle[vehicleID]
	return d, ok
}

// GeofencesFor returns the geofences that apply to a vehicle (its tenant's, narrowed by fleet).
func (r *Registry) GeofencesFor(d Device) []Geofence {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Geofence
	for _, g := range r.snap.geofences[d.TenantID] {
		if g.FleetID == "" || g.FleetID == d.FleetID {
			out = append(out, g)
		}
	}
	return out
}

// Geofence resolves a geofence by id.
func (r *Registry) Geofence(id string) (Geofence, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	g, ok := r.snap.geofenceBy[id]
	return g, ok
}

// Devices returns a copy of every known device.
func (r *Registry) Devices() []Device {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Device, 0, len(r.snap.byDevice))
	for _, d := range r.snap.byDevice {
		out = append(out, d)
	}
	return out
}

// RedisDevicesKey is the hash ingest reads: device id -> JSON of CachedDevice.
const RedisDevicesKey = "registry:devices"

// CachedDevice is what ingest needs on its hot path and nothing more.
type CachedDevice struct {
	Tenant    string  `json:"t"`
	Vehicle   string  `json:"v"`
	Tier      string  `json:"tier"`
	MaxHz     float64 `json:"hz"`
	TokenHash string  `json:"h"`
}

// PublishToRedis replaces the shared device hash atomically (write a new hash,
// then RENAME over the live one) so ingest never reads a half-built registry.
func (r *Registry) PublishToRedis(ctx context.Context, rdb *redis.Client) error {
	devices := r.Devices()
	if len(devices) == 0 {
		return nil
	}
	tmp := RedisDevicesKey + ":next"
	pipe := rdb.Pipeline()
	pipe.Del(ctx, tmp)
	for _, d := range devices {
		b, _ := json.Marshal(CachedDevice{Tenant: d.TenantID, Vehicle: d.VehicleID, Tier: d.Tier, MaxHz: d.MaxHz, TokenHash: d.TokenHash})
		pipe.HSet(ctx, tmp, d.DeviceID, b)
	}
	pipe.Rename(ctx, tmp, RedisDevicesKey)
	_, err := pipe.Exec(ctx)
	return err
}
