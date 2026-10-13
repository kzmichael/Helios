// Geofence CRUD. Validates GeoJSON polygons on write per
// apps/_contracts/openapi.yaml's GeoJsonPolygon schema.
import {
  Body,
  Controller,
  Get,
  Inject,
  Injectable,
  Module,
  NotFoundException,
  Post,
  Req,
  UnprocessableEntityException,
  UseGuards,
} from '@nestjs/common';
import { Request } from 'express';
import { Pool } from 'pg';
import { PG_POOL } from '../common/db.module';
import { TenantGuard } from '../common/tenant-guard';

interface GeoJsonPolygon {
  type: 'Polygon';
  coordinates: number[][][];
}

interface GeofenceInput {
  name: string;
  fleetId?: string;
  kind: 'inclusion' | 'exclusion';
  boundary: GeoJsonPolygon;
  dwellThresholdS?: number;
}

/** Rejects degenerate polygons before they reach the database or stream-proc. */
function validatePolygon(boundary: GeoJsonPolygon): void {
  if (boundary?.type !== 'Polygon') {
    throw new UnprocessableEntityException('boundary.type must be "Polygon"');
  }
  const ring = boundary.coordinates?.[0];
  if (!ring || ring.length < 4) {
    throw new UnprocessableEntityException('boundary must have a closed ring of at least 4 points');
  }
  const [firstLon, firstLat] = ring[0];
  const [lastLon, lastLat] = ring[ring.length - 1];
  if (firstLon !== lastLon || firstLat !== lastLat) {
    throw new UnprocessableEntityException('boundary ring must be closed (first point === last point)');
  }
}

function centroidOf(boundary: GeoJsonPolygon): { lat: number; lon: number } {
  const ring = boundary.coordinates[0];
  const points = ring.slice(0, -1); // last point duplicates the first
  const sum = points.reduce(
    (acc, [lon, lat]) => ({ lon: acc.lon + lon, lat: acc.lat + lat }),
    { lon: 0, lat: 0 },
  );
  return { lat: sum.lat / points.length, lon: sum.lon / points.length };
}

@Injectable()
export class GeofencesService {
  constructor(@Inject(PG_POOL) private readonly pool: Pool) {}

  async list(tenantId: string) {
    const { rows } = await this.pool.query(
      `SELECT id, fleet_id AS "fleetId", name, kind, boundary,
              centroid_lat AS "centroidLat", centroid_lon AS "centroidLon",
              radius_m AS "radiusM", dwell_threshold_s AS "dwellThresholdS", enabled
       FROM geofence WHERE tenant_id = $1 AND enabled ORDER BY name`,
      [tenantId],
    );
    return { data: rows };
  }

  async create(tenantId: string, input: GeofenceInput) {
    validatePolygon(input.boundary);
    const centroid = centroidOf(input.boundary);

    const { rows } = await this.pool.query(
      `INSERT INTO geofence (tenant_id, fleet_id, name, kind, boundary,
                              centroid_lat, centroid_lon, radius_m, dwell_threshold_s)
       VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
       RETURNING id, fleet_id AS "fleetId", name, kind, boundary,
                 centroid_lat AS "centroidLat", centroid_lon AS "centroidLon",
                 radius_m AS "radiusM", dwell_threshold_s AS "dwellThresholdS", enabled`,
      [
        tenantId,
        input.fleetId ?? null,
        input.name,
        input.kind,
        JSON.stringify(input.boundary),
        centroid.lat,
        centroid.lon,
        800, // Phase 0 default; a real radius derivation is a Phase 2 refinement.
        input.dwellThresholdS ?? 300,
      ],
    );
    return rows[0];
  }

  async getOrThrow(tenantId: string, geofenceId: string) {
    const { rows } = await this.pool.query(
      `SELECT id FROM geofence WHERE tenant_id = $1 AND id = $2`,
      [tenantId, geofenceId],
    );
    if (!rows[0]) throw new NotFoundException();
    return rows[0];
  }
}

@Controller('v1/geofences')
@UseGuards(TenantGuard)
export class GeofencesController {
  constructor(private readonly geofences: GeofencesService) {}

  @Get()
  list(@Req() req: Request) {
    return this.geofences.list(req.tenant!.tenantId);
  }

  @Post()
  create(@Req() req: Request, @Body() body: GeofenceInput) {
    return this.geofences.create(req.tenant!.tenantId, body);
  }
}

@Module({
  controllers: [GeofencesController],
  providers: [GeofencesService],
})
export class GeofencesModule {}
