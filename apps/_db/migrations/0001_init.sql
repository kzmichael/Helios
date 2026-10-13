-- Trackline initial schema.
-- Forward-only. Multi-tenant by tenant_id on every table.

BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ─────────────────────────────── tenancy ───────────────────────────────

CREATE TABLE tenant (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            text        NOT NULL UNIQUE,
    name            text        NOT NULL,
    tier            text        NOT NULL CHECK (tier IN ('free', 'pro', 'enterprise')),
    status          text        NOT NULL DEFAULT 'active'
                                CHECK (status IN ('active', 'suspended', 'closed')),
    country_code    char(2)     NOT NULL DEFAULT 'IN',
    max_devices     integer     NOT NULL,
    max_hz          numeric(4,1) NOT NULL,
    retention_days  integer     NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE app_user (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid        NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
    email           text        NOT NULL,
    display_name    text        NOT NULL,
    role            text        NOT NULL CHECK (role IN ('owner', 'admin', 'operator', 'viewer')),
    password_hash   text,
    status          text        NOT NULL DEFAULT 'active'
                                CHECK (status IN ('active', 'invited', 'disabled')),
    last_login_at   timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX app_user_email_uniq ON app_user (lower(email));
CREATE INDEX app_user_tenant_idx ON app_user (tenant_id);

CREATE TABLE api_key (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid        NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
    name            text        NOT NULL,
    prefix          text        NOT NULL UNIQUE,
    secret_hash     text        NOT NULL,
    scopes          text[]      NOT NULL DEFAULT '{}',
    last_used_at    timestamptz,
    expires_at      timestamptz,
    revoked_at      timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX api_key_tenant_idx ON api_key (tenant_id);

-- ──────────────────────────────── fleet ────────────────────────────────

CREATE TABLE fleet (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid        NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
    name            text        NOT NULL,
    description     text,
    depot_lat       double precision,
    depot_lon       double precision,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX fleet_tenant_idx ON fleet (tenant_id);

CREATE TABLE driver (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid        NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
    full_name       text        NOT NULL,
    licence_number  text        NOT NULL,
    phone           text,
    email           text,
    status          text        NOT NULL DEFAULT 'active'
                                CHECK (status IN ('active', 'inactive')),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX driver_tenant_idx ON driver (tenant_id);

CREATE TABLE vehicle (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid        NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
    fleet_id        uuid        NOT NULL REFERENCES fleet(id) ON DELETE CASCADE,
    external_ref    text        NOT NULL,
    registration    text        NOT NULL,
    make            text,
    model           text,
    year            smallint,
    vehicle_type    text        NOT NULL DEFAULT 'truck'
                                CHECK (vehicle_type IN ('truck', 'van', 'car', 'bike', 'trailer')),
    status          text        NOT NULL DEFAULT 'active'
                                CHECK (status IN ('active', 'maintenance', 'retired')),
    speed_limit_kph numeric(5,1) NOT NULL DEFAULT 80.0,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX vehicle_tenant_ref_uniq ON vehicle (tenant_id, external_ref);
CREATE INDEX vehicle_tenant_idx ON vehicle (tenant_id);
CREATE INDEX vehicle_fleet_idx ON vehicle (fleet_id);

CREATE TABLE device (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid        NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
    vehicle_id      uuid        REFERENCES vehicle(id) ON DELETE SET NULL,
    serial          text        NOT NULL UNIQUE,
    model           text        NOT NULL DEFAULT 'TL-100',
    firmware        text        NOT NULL DEFAULT '1.0.0',
    auth_token_hash text        NOT NULL,
    status          text        NOT NULL DEFAULT 'provisioned'
                                CHECK (status IN ('provisioned', 'active', 'offline', 'decommissioned')),
    last_seen_at    timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX device_tenant_idx ON device (tenant_id);
CREATE UNIQUE INDEX device_vehicle_uniq ON device (vehicle_id) WHERE vehicle_id IS NOT NULL;

CREATE TABLE vehicle_driver_assignment (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid        NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
    vehicle_id      uuid        NOT NULL REFERENCES vehicle(id) ON DELETE CASCADE,
    driver_id       uuid        NOT NULL REFERENCES driver(id) ON DELETE CASCADE,
    assigned_from   timestamptz NOT NULL DEFAULT now(),
    assigned_until  timestamptz
);

CREATE INDEX vda_vehicle_idx ON vehicle_driver_assignment (vehicle_id, assigned_from DESC);

-- ───────────────────────────── geofencing ──────────────────────────────

-- Boundary stored as GeoJSON. PostGIS is a Phase 2 upgrade; point-in-polygon is
-- currently evaluated in application code.
CREATE TABLE geofence (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid        NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
    fleet_id        uuid        REFERENCES fleet(id) ON DELETE CASCADE,
    name            text        NOT NULL,
    kind            text        NOT NULL CHECK (kind IN ('inclusion', 'exclusion')),
    boundary        jsonb       NOT NULL,
    centroid_lat    double precision NOT NULL,
    centroid_lon    double precision NOT NULL,
    radius_m        integer     NOT NULL,
    dwell_threshold_s integer   NOT NULL DEFAULT 300,
    enabled         boolean     NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX geofence_tenant_idx ON geofence (tenant_id) WHERE enabled;

-- ─────────────────────────── alerting ──────────────────────────────────

CREATE TABLE alert_rule (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid        NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
    fleet_id        uuid        REFERENCES fleet(id) ON DELETE CASCADE,
    name            text        NOT NULL,
    alert_type      text        NOT NULL CHECK (alert_type IN
                                ('geofence_enter', 'geofence_exit', 'geofence_dwell',
                                 'speeding', 'idle', 'offline')),
    severity        text        NOT NULL DEFAULT 'warning'
                                CHECK (severity IN ('info', 'warning', 'critical')),
    params          jsonb       NOT NULL DEFAULT '{}'::jsonb,
    cooldown_s      integer     NOT NULL DEFAULT 600,
    enabled         boolean     NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX alert_rule_tenant_idx ON alert_rule (tenant_id) WHERE enabled;

CREATE TABLE alert (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid        NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
    rule_id         uuid        REFERENCES alert_rule(id) ON DELETE SET NULL,
    vehicle_id      uuid        NOT NULL REFERENCES vehicle(id) ON DELETE CASCADE,
    geofence_id     uuid        REFERENCES geofence(id) ON DELETE SET NULL,
    alert_type      text        NOT NULL,
    severity        text        NOT NULL,
    summary         text        NOT NULL,
    payload         jsonb       NOT NULL DEFAULT '{}'::jsonb,
    position_lat    double precision,
    position_lon    double precision,
    triggered_at    timestamptz NOT NULL DEFAULT now(),
    acknowledged_at timestamptz,
    acknowledged_by uuid        REFERENCES app_user(id) ON DELETE SET NULL,
    resolved_at     timestamptz
);

CREATE INDEX alert_tenant_time_idx ON alert (tenant_id, triggered_at DESC);
CREATE INDEX alert_vehicle_idx ON alert (vehicle_id, triggered_at DESC);
CREATE INDEX alert_open_idx ON alert (tenant_id) WHERE acknowledged_at IS NULL;

-- ──────────────────────────── notification ─────────────────────────────

CREATE TABLE notification_channel (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid        NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
    name            text        NOT NULL,
    kind            text        NOT NULL CHECK (kind IN ('email', 'webhook', 'sms')),
    config          jsonb       NOT NULL,
    min_severity    text        NOT NULL DEFAULT 'warning'
                                CHECK (min_severity IN ('info', 'warning', 'critical')),
    enabled         boolean     NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX notification_channel_tenant_idx ON notification_channel (tenant_id) WHERE enabled;

CREATE TABLE notification_delivery (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid        NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
    alert_id        uuid        NOT NULL REFERENCES alert(id) ON DELETE CASCADE,
    channel_id      uuid        NOT NULL REFERENCES notification_channel(id) ON DELETE CASCADE,
    idempotency_key text        NOT NULL,
    status          text        NOT NULL DEFAULT 'pending'
                                CHECK (status IN ('pending', 'sent', 'failed', 'dead')),
    attempts        integer     NOT NULL DEFAULT 0,
    status_code     integer,
    last_error      text,
    latency_ms      integer,
    first_attempt_at timestamptz,
    last_attempt_at timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX nd_alert_idx ON notification_delivery (alert_id);
CREATE INDEX nd_pending_idx ON notification_delivery (status, created_at) WHERE status = 'pending';

-- ───────────────────────────── telemetry ───────────────────────────────

-- Hot path writes. Becomes a Timescale hypertable in Phase 2.
CREATE TABLE telemetry (
    id              bigserial PRIMARY KEY,
    tenant_id       uuid        NOT NULL,
    vehicle_id      uuid        NOT NULL,
    device_id       uuid        NOT NULL,
    recorded_at     timestamptz NOT NULL,
    ingested_at     timestamptz NOT NULL DEFAULT now(),
    lat             double precision NOT NULL,
    lon             double precision NOT NULL,
    speed_kph       numeric(6,2) NOT NULL DEFAULT 0,
    heading_deg     numeric(5,2) NOT NULL DEFAULT 0,
    altitude_m      numeric(7,2),
    accuracy_m      numeric(6,2),
    odometer_km     numeric(10,2),
    fuel_level_pct  numeric(5,2),
    engine_temp_c   numeric(5,2),
    ignition_on     boolean     NOT NULL DEFAULT false,
    sensors         jsonb
);

CREATE INDEX telemetry_vehicle_time_idx ON telemetry (vehicle_id, recorded_at DESC);

-- Denormalised last-known position, so the map does not query the history table.
CREATE TABLE vehicle_position_latest (
    vehicle_id      uuid PRIMARY KEY REFERENCES vehicle(id) ON DELETE CASCADE,
    tenant_id       uuid        NOT NULL,
    lat             double precision NOT NULL,
    lon             double precision NOT NULL,
    speed_kph       numeric(6,2) NOT NULL DEFAULT 0,
    heading_deg     numeric(5,2) NOT NULL DEFAULT 0,
    ignition_on     boolean     NOT NULL DEFAULT false,
    recorded_at     timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX vpl_tenant_idx ON vehicle_position_latest (tenant_id);

-- ──────────────────────────── billing ──────────────────────────────────

CREATE TABLE usage_daily (
    tenant_id       uuid        NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
    usage_date      date        NOT NULL,
    telemetry_points bigint     NOT NULL DEFAULT 0,
    active_devices  integer     NOT NULL DEFAULT 0,
    api_requests    bigint      NOT NULL DEFAULT 0,
    storage_bytes   bigint      NOT NULL DEFAULT 0,
    computed_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, usage_date)
);

CREATE TABLE invoice (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       uuid        NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
    number          text        NOT NULL UNIQUE,
    period_start    date        NOT NULL,
    period_end      date        NOT NULL,
    currency        char(3)     NOT NULL DEFAULT 'INR',
    subtotal_minor  bigint      NOT NULL DEFAULT 0,
    tax_minor       bigint      NOT NULL DEFAULT 0,
    total_minor     bigint      NOT NULL DEFAULT 0,
    status          text        NOT NULL DEFAULT 'draft'
                                CHECK (status IN ('draft', 'issued', 'paid', 'void')),
    issued_at       timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX invoice_tenant_period_uniq ON invoice (tenant_id, period_start, period_end);

CREATE TABLE invoice_line (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id      uuid        NOT NULL REFERENCES invoice(id) ON DELETE CASCADE,
    description     text        NOT NULL,
    quantity        numeric(14,3) NOT NULL,
    unit_price_minor bigint     NOT NULL,
    amount_minor    bigint      NOT NULL,
    line_no         smallint    NOT NULL
);

CREATE INDEX invoice_line_invoice_idx ON invoice_line (invoice_id, line_no);

-- ───────────────────────────── audit ───────────────────────────────────

CREATE TABLE audit_log (
    id              bigserial PRIMARY KEY,
    tenant_id       uuid        NOT NULL,
    actor_kind      text        NOT NULL CHECK (actor_kind IN ('user', 'device', 'service')),
    actor_id        text        NOT NULL,
    action          text        NOT NULL,
    target_type     text        NOT NULL,
    target_id       text,
    metadata        jsonb       NOT NULL DEFAULT '{}'::jsonb,
    occurred_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_tenant_time_idx ON audit_log (tenant_id, occurred_at DESC);

COMMIT;
