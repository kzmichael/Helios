-- Deterministic seed data. Safe to run once against a clean database.
--
-- Three tenants with deliberately uneven sizes — 5 / 25 / 120 vehicles — because
-- equally sized tenants teach nothing about noisy neighbours or per-tenant cost.
--
-- Credentials here are development-only and are generated at seed time.

BEGIN;

-- ─────────────────────────────── tenants ───────────────────────────────

INSERT INTO tenant (id, slug, name, tier, max_devices, max_hz, retention_days) VALUES
  ('11111111-1111-4111-8111-111111111111', 'northwind',  'Northwind Logistics',  'free',           50,  1.0,   7),
  ('22222222-2222-4222-8222-222222222222', 'meridian',   'Meridian Freight',     'pro',          5000,  1.0,  90),
  ('33333333-3333-4333-8333-333333333333', 'atlas',      'Atlas Continental',    'enterprise',  500000, 10.0, 730);

-- ──────────────────────────────── users ────────────────────────────────

INSERT INTO app_user (id, tenant_id, email, display_name, role, password_hash) VALUES
  ('11111111-0000-4000-8000-000000000001', '11111111-1111-4111-8111-111111111111', 'owner@northwind.test', 'Ravi Menon',      'owner',    crypt('trackline-dev', gen_salt('bf', 10))),
  ('11111111-0000-4000-8000-000000000002', '11111111-1111-4111-8111-111111111111', 'ops@northwind.test',   'Sara Iyer',       'operator', crypt('trackline-dev', gen_salt('bf', 10))),

  ('22222222-0000-4000-8000-000000000001', '22222222-2222-4222-8222-222222222222', 'owner@meridian.test',  'Daniel Osei',     'owner',    crypt('trackline-dev', gen_salt('bf', 10))),
  ('22222222-0000-4000-8000-000000000002', '22222222-2222-4222-8222-222222222222', 'ops@meridian.test',    'Priya Nair',      'operator', crypt('trackline-dev', gen_salt('bf', 10))),
  ('22222222-0000-4000-8000-000000000003', '22222222-2222-4222-8222-222222222222', 'view@meridian.test',   'Tom Whelan',      'viewer',   crypt('trackline-dev', gen_salt('bf', 10))),

  ('33333333-0000-4000-8000-000000000001', '33333333-3333-4333-8333-333333333333', 'owner@atlas.test',     'Ingrid Halvorsen','owner',    crypt('trackline-dev', gen_salt('bf', 10))),
  ('33333333-0000-4000-8000-000000000002', '33333333-3333-4333-8333-333333333333', 'admin@atlas.test',     'Marcus Reed',     'admin',    crypt('trackline-dev', gen_salt('bf', 10))),
  ('33333333-0000-4000-8000-000000000003', '33333333-3333-4333-8333-333333333333', 'ops@atlas.test',       'Lena Fischer',    'operator', crypt('trackline-dev', gen_salt('bf', 10)));

-- ─────────────────────────────── api keys ──────────────────────────────

INSERT INTO api_key (tenant_id, name, prefix, secret_hash, scopes)
SELECT id,
       'default',
       'tl_dev_' || substr(replace(id::text, '-', ''), 1, 8),
       encode(digest('dev-secret-' || slug, 'sha256'), 'hex'),
       ARRAY['fleet:read', 'fleet:write', 'telemetry:write', 'alerts:read']
FROM tenant;

-- ──────────────────────────────── fleets ───────────────────────────────

INSERT INTO fleet (id, tenant_id, name, description, depot_lat, depot_lon) VALUES
  ('1f000000-0000-4000-8000-000000000001', '11111111-1111-4111-8111-111111111111', 'City Runs',        'Intra-city deliveries',       12.9716, 77.5946),

  ('2f000000-0000-4000-8000-000000000001', '22222222-2222-4222-8222-222222222222', 'Western Corridor', 'Mumbai to Pune line haul',    19.0760, 72.8777),
  ('2f000000-0000-4000-8000-000000000002', '22222222-2222-4222-8222-222222222222', 'Last Mile',        'Metro last-mile vans',        19.1136, 72.8697),

  ('3f000000-0000-4000-8000-000000000001', '33333333-3333-4333-8333-333333333333', 'North Trunk',      'Delhi NCR trunk routes',      28.6139, 77.2090),
  ('3f000000-0000-4000-8000-000000000002', '33333333-3333-4333-8333-333333333333', 'West Trunk',       'Pune and Nashik',             18.5204, 73.8567),
  ('3f000000-0000-4000-8000-000000000003', '33333333-3333-4333-8333-333333333333', 'South Trunk',      'Bengaluru and Chennai',       12.9716, 77.5946),
  ('3f000000-0000-4000-8000-000000000004', '33333333-3333-4333-8333-333333333333', 'Cold Chain',       'Temperature-controlled fleet', 17.3850, 78.4867);

-- ─────────────────────────────── drivers ───────────────────────────────

INSERT INTO driver (id, tenant_id, full_name, licence_number, phone)
SELECT
    ('d0000000-0000-4000-8000-' || lpad((t.ord * 1000 + n)::text, 12, '0'))::uuid,
    t.id,
    (ARRAY['Arun','Bhavna','Chetan','Deepa','Farhan','Geeta','Harsh','Ishani',
           'Jatin','Kavya','Lakshmi','Manoj','Nisha','Omkar','Pooja','Rahul'])[1 + (n % 16)]
      || ' ' ||
    (ARRAY['Sharma','Patel','Reddy','Singh','Das','Kulkarni','Menon','Joshi'])[1 + (n % 8)],
    'DL' || lpad((t.ord * 100000 + n)::text, 10, '0'),
    '+9198' || lpad((t.ord * 100000 + n)::text, 8, '0')
FROM (VALUES
        ('11111111-1111-4111-8111-111111111111'::uuid, 1,  6),
        ('22222222-2222-4222-8222-222222222222'::uuid, 2, 30),
        ('33333333-3333-4333-8333-333333333333'::uuid, 3,140)
     ) AS t(id, ord, qty),
     LATERAL generate_series(1, t.qty) AS n;

-- ─────────────────────────────── vehicles ──────────────────────────────

INSERT INTO vehicle (id, tenant_id, fleet_id, external_ref, registration, make, model, year, vehicle_type, speed_limit_kph)
SELECT
    ('c0000000-0000-4000-8000-' || lpad((f.ord * 10000 + n)::text, 12, '0'))::uuid,
    f.tenant_id,
    f.id,
    'VEH-' || lpad((f.ord * 10000 + n)::text, 6, '0'),
    (ARRAY['KA','MH','DL','TN','TS'])[1 + (f.ord % 5)] || '-' ||
      lpad(((f.ord * 7 + n) % 99)::text, 2, '0') || '-' ||
      chr(65 + (n % 26)) || chr(65 + ((n * 3) % 26)) || '-' ||
      lpad(((f.ord * 131 + n * 17) % 9999)::text, 4, '0'),
    (ARRAY['Tata','Ashok Leyland','Mahindra','Eicher','BharatBenz'])[1 + (n % 5)],
    (ARRAY['Prima','Boss','Blazo','Pro 2049','1015R'])[1 + (n % 5)],
    2018 + (n % 7),
    (ARRAY['truck','truck','truck','van','trailer'])[1 + (n % 5)],
    (ARRAY[60.0, 70.0, 80.0, 90.0])[1 + (n % 4)]
FROM (VALUES
        ('1f000000-0000-4000-8000-000000000001'::uuid, '11111111-1111-4111-8111-111111111111'::uuid, 1,   5),
        ('2f000000-0000-4000-8000-000000000001'::uuid, '22222222-2222-4222-8222-222222222222'::uuid, 2,  15),
        ('2f000000-0000-4000-8000-000000000002'::uuid, '22222222-2222-4222-8222-222222222222'::uuid, 3,  10),
        ('3f000000-0000-4000-8000-000000000001'::uuid, '33333333-3333-4333-8333-333333333333'::uuid, 4,  40),
        ('3f000000-0000-4000-8000-000000000002'::uuid, '33333333-3333-4333-8333-333333333333'::uuid, 5,  30),
        ('3f000000-0000-4000-8000-000000000003'::uuid, '33333333-3333-4333-8333-333333333333'::uuid, 6,  30),
        ('3f000000-0000-4000-8000-000000000004'::uuid, '33333333-3333-4333-8333-333333333333'::uuid, 7,  20)
     ) AS f(id, tenant_id, ord, qty),
     LATERAL generate_series(1, f.qty) AS n;

-- ──────────────────────────────── devices ──────────────────────────────

INSERT INTO device (id, tenant_id, vehicle_id, serial, model, firmware, auth_token_hash, status)
SELECT
    ('de000000-0000-4000-8000-' || substr(replace(v.id::text, '-', ''), 21, 12))::uuid,
    v.tenant_id,
    v.id,
    'TLD-' || upper(substr(replace(v.id::text, '-', ''), 21, 10)),
    CASE WHEN v.vehicle_type = 'trailer' THEN 'TL-50' ELSE 'TL-100' END,
    (ARRAY['1.4.2', '1.5.0', '1.5.1'])[1 + abs(hashtext(v.id::text) % 3)],
    encode(digest('dev-token-' || v.id::text, 'sha256'), 'hex'),
    'active'
FROM vehicle v;

-- ────────────────────────── driver assignments ─────────────────────────

INSERT INTO vehicle_driver_assignment (tenant_id, vehicle_id, driver_id)
SELECT v.tenant_id, v.id, d.id
FROM (
    SELECT id, tenant_id, row_number() OVER (PARTITION BY tenant_id ORDER BY external_ref) AS rn
    FROM vehicle
) v
JOIN (
    SELECT id, tenant_id, row_number() OVER (PARTITION BY tenant_id ORDER BY licence_number) AS rn
    FROM driver
) d ON d.tenant_id = v.tenant_id AND d.rn = v.rn;

-- ────────────────────────────── geofences ──────────────────────────────

INSERT INTO geofence (id, tenant_id, fleet_id, name, kind, centroid_lat, centroid_lon, radius_m, dwell_threshold_s, boundary) VALUES
  ('9f000000-0000-4000-8000-000000000001', '11111111-1111-4111-8111-111111111111', '1f000000-0000-4000-8000-000000000001',
   'Bengaluru Depot', 'inclusion', 12.9716, 77.5946, 800, 300,
   '{"type":"Polygon","coordinates":[[[77.5866,12.9646],[77.6026,12.9646],[77.6026,12.9786],[77.5866,12.9786],[77.5866,12.9646]]]}'),

  ('9f000000-0000-4000-8000-000000000002', '22222222-2222-4222-8222-222222222222', '2f000000-0000-4000-8000-000000000001',
   'Mumbai Hub', 'inclusion', 19.0760, 72.8777, 1200, 600,
   '{"type":"Polygon","coordinates":[[[72.8657,19.0660],[72.8897,19.0660],[72.8897,19.0860],[72.8657,19.0860],[72.8657,19.0660]]]}'),

  ('9f000000-0000-4000-8000-000000000003', '22222222-2222-4222-8222-222222222222', NULL,
   'Restricted Port Area', 'exclusion', 18.9480, 72.8420, 1500, 120,
   '{"type":"Polygon","coordinates":[[[72.8320,18.9380],[72.8520,18.9380],[72.8520,18.9580],[72.8320,18.9580],[72.8320,18.9380]]]}'),

  ('9f000000-0000-4000-8000-000000000004', '33333333-3333-4333-8333-333333333333', '3f000000-0000-4000-8000-000000000001',
   'Delhi NCR Yard', 'inclusion', 28.6139, 77.2090, 2000, 900,
   '{"type":"Polygon","coordinates":[[[77.1890,28.5939],[77.2290,28.5939],[77.2290,28.6339],[77.1890,28.6339],[77.1890,28.5939]]]}'),

  ('9f000000-0000-4000-8000-000000000005', '33333333-3333-4333-8333-333333333333', '3f000000-0000-4000-8000-000000000004',
   'Hyderabad Cold Store', 'inclusion', 17.3850, 78.4867, 600, 1800,
   '{"type":"Polygon","coordinates":[[[78.4767,17.3750],[78.4967,17.3750],[78.4967,17.3950],[78.4767,17.3950],[78.4767,17.3750]]]}');

-- ───────────────────────────── alert rules ─────────────────────────────

INSERT INTO alert_rule (tenant_id, fleet_id, name, alert_type, severity, params, cooldown_s) VALUES
  ('11111111-1111-4111-8111-111111111111', NULL, 'Speeding',            'speeding',       'warning',  '{"tolerance_kph": 10, "min_duration_s": 30}', 600),
  ('11111111-1111-4111-8111-111111111111', NULL, 'Device offline',      'offline',        'critical', '{"silent_seconds": 900}', 1800),

  ('22222222-2222-4222-8222-222222222222', NULL, 'Speeding',            'speeding',       'warning',  '{"tolerance_kph": 5, "min_duration_s": 20}', 300),
  ('22222222-2222-4222-8222-222222222222', NULL, 'Device offline',      'offline',        'critical', '{"silent_seconds": 600}', 1800),
  ('22222222-2222-4222-8222-222222222222', NULL, 'Left the hub',        'geofence_exit',  'info',     '{"geofence_id": "9f000000-0000-4000-8000-000000000002"}', 120),
  ('22222222-2222-4222-8222-222222222222', NULL, 'Entered port area',   'geofence_enter', 'critical', '{"geofence_id": "9f000000-0000-4000-8000-000000000003"}', 60),

  ('33333333-3333-4333-8333-333333333333', NULL, 'Speeding',            'speeding',       'warning',  '{"tolerance_kph": 5, "min_duration_s": 15}', 300),
  ('33333333-3333-4333-8333-333333333333', NULL, 'Device offline',      'offline',        'critical', '{"silent_seconds": 300}', 900),
  ('33333333-3333-4333-8333-333333333333', NULL, 'Excessive idling',    'idle',           'warning',  '{"idle_seconds": 1200}', 3600),
  ('33333333-3333-4333-8333-333333333333', '3f000000-0000-4000-8000-000000000004', 'Cold store dwell', 'geofence_dwell', 'info', '{"geofence_id": "9f000000-0000-4000-8000-000000000005"}', 1800);

-- ────────────────────────── notification channels ──────────────────────
-- Email points at the local Mailpit sink, webhooks at the local inspector.
-- Nothing in development should ever reach a real recipient.

INSERT INTO notification_channel (tenant_id, name, kind, config, min_severity) VALUES
  ('11111111-1111-4111-8111-111111111111', 'Ops mailbox',   'email',   '{"to": "ops@northwind.test", "from": "alerts@trackline.local"}', 'warning'),

  ('22222222-2222-4222-8222-222222222222', 'Ops mailbox',   'email',   '{"to": "ops@meridian.test", "from": "alerts@trackline.local"}', 'warning'),
  ('22222222-2222-4222-8222-222222222222', 'Dispatch hook', 'webhook', '{"url": "http://webhook-sink:8080/meridian", "secret": "dev-webhook-secret"}', 'info'),

  ('33333333-3333-4333-8333-333333333333', 'Ops mailbox',   'email',   '{"to": "ops@atlas.test", "from": "alerts@trackline.local"}', 'info'),
  ('33333333-3333-4333-8333-333333333333', 'SIEM hook',     'webhook', '{"url": "http://webhook-sink:8080/atlas", "secret": "dev-webhook-secret"}', 'warning'),
  ('33333333-3333-4333-8333-333333333333', 'On-call SMS',   'sms',     '{"to": "+919800000000"}', 'critical');

-- ─────────────────────── initial positions (map) ───────────────────────
-- Scattered deterministically around each fleet depot so the map is not empty
-- before device-sim starts producing.

INSERT INTO vehicle_position_latest (vehicle_id, tenant_id, lat, lon, speed_kph, heading_deg, ignition_on, recorded_at)
SELECT
    v.id,
    v.tenant_id,
    f.depot_lat + ((abs(hashtext(v.id::text)        % 2000) - 1000) / 20000.0),
    f.depot_lon + ((abs(hashtext(v.id::text || 'x') % 2000) - 1000) / 20000.0),
    abs(hashtext(v.id::text || 'spd') % 90)::numeric,
    abs(hashtext(v.id::text || 'hdg') % 360)::numeric,
    abs(hashtext(v.id::text || 'ign') % 4) <> 0,
    now() - (abs(hashtext(v.id::text) % 300) || ' seconds')::interval
FROM vehicle v
JOIN fleet f ON f.id = v.fleet_id;

COMMIT;

-- Sanity summary. Fleet and vehicle counts are aggregated separately; joining both
-- to tenant in one query multiplies the rows.
SELECT
    t.tier,
    t.name,
    (SELECT count(*) FROM fleet   f WHERE f.tenant_id   = t.id) AS fleets,
    (SELECT count(*) FROM vehicle v WHERE v.tenant_id   = t.id) AS vehicles,
    (SELECT count(*) FROM device  d WHERE d.tenant_id   = t.id) AS devices,
    (SELECT count(*) FROM driver  r WHERE r.tenant_id   = t.id) AS drivers
FROM tenant t
ORDER BY vehicles;
