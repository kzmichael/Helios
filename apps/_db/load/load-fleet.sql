-- Opt-in load fleet: 350 extra vehicles and devices so device-sim's 500 devices all
-- resolve to provisioned vehicles (150 seeded + 350 here = 500).
--
-- NOT part of the Flyway directories on purpose: the default seed stays at 150
-- vehicles. Apply it deliberately before a 500-device run:
--
--   psql "$DATABASE_URL" -f apps/_db/load/load-fleet.sql
--
-- Idempotent (ON CONFLICT DO NOTHING). Ids follow the seed's scheme, which
-- apps/device-sim/internal/identity mirrors:
--   vehicle  c0000000-0000-4000-8000-<12 digits of fleet_ord*10000 + n>
--   device   de000000-0000-4000-8000-<same 12 digits>
--
-- Northwind (free) ends at exactly its 50-device quota, so creating one more
-- vehicle there returns the documented 422 quota-exceeded response.

BEGIN;

INSERT INTO fleet (id, tenant_id, name, description, depot_lat, depot_lon) VALUES
  ('1f000000-0000-4000-8000-000000000002', '11111111-1111-4111-8111-111111111111', 'Load Test',  'Opt-in load fleet',  12.9716, 77.5946),
  ('2f000000-0000-4000-8000-000000000003', '22222222-2222-4222-8222-222222222222', 'Load Test',  'Opt-in load fleet',  19.0760, 72.8777),
  ('3f000000-0000-4000-8000-000000000005', '33333333-3333-4333-8333-333333333333', 'Load Test',  'Opt-in load fleet',  28.6139, 77.2090)
ON CONFLICT (id) DO NOTHING;

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
        ('1f000000-0000-4000-8000-000000000002'::uuid, '11111111-1111-4111-8111-111111111111'::uuid,  8,  45),
        ('2f000000-0000-4000-8000-000000000003'::uuid, '22222222-2222-4222-8222-222222222222'::uuid,  9, 175),
        ('3f000000-0000-4000-8000-000000000005'::uuid, '33333333-3333-4333-8333-333333333333'::uuid, 10, 130)
     ) AS f(id, tenant_id, ord, qty),
     LATERAL generate_series(1, f.qty) AS n
ON CONFLICT DO NOTHING;

INSERT INTO device (id, tenant_id, vehicle_id, serial, model, firmware, auth_token_hash, status)
SELECT
    ('de000000-0000-4000-8000-' || substr(replace(v.id::text, '-', ''), 21, 12))::uuid,
    v.tenant_id,
    v.id,
    'TLD-' || upper(substr(replace(v.id::text, '-', ''), 21, 12)),
    CASE WHEN v.vehicle_type = 'trailer' THEN 'TL-50' ELSE 'TL-100' END,
    (ARRAY['1.4.2', '1.5.0', '1.5.1'])[1 + abs(hashtext(v.id::text) % 3)],
    encode(digest('dev-token-' || v.id::text, 'sha256'), 'hex'),
    'active'
FROM vehicle v
WHERE v.fleet_id IN ('1f000000-0000-4000-8000-000000000002',
                     '2f000000-0000-4000-8000-000000000003',
                     '3f000000-0000-4000-8000-000000000005')
ON CONFLICT DO NOTHING;

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
JOIN fleet f ON f.id = v.fleet_id
WHERE v.fleet_id IN ('1f000000-0000-4000-8000-000000000002',
                     '2f000000-0000-4000-8000-000000000003',
                     '3f000000-0000-4000-8000-000000000005')
ON CONFLICT DO NOTHING;

COMMIT;

SELECT t.tier, t.max_devices,
       (SELECT count(*) FROM device d WHERE d.tenant_id = t.id) AS devices
FROM tenant t ORDER BY devices;
