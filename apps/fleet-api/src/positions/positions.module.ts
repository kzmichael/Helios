// Live positions. Two ways to read them, both tenant-scoped:
//
//   GET /v1/positions          last-known position of every vehicle (the map's first paint)
//   WS  /v1/positions/stream   every position as it arrives, for as long as the socket is open
//
// The stream is fed by the pipeline's position-updater, which publishes each new
// position to the Redis channel positions:{tenantId}. fleet-api never queries the
// telemetry history table for the map; vehicle_position_latest exists for that.
import {
  Controller,
  Get,
  Inject,
  Injectable,
  Logger,
  Module,
  OnApplicationBootstrap,
  OnApplicationShutdown,
  Query,
  Req,
  UseGuards,
} from '@nestjs/common';
import { HttpAdapterHost } from '@nestjs/core';
import { IsOptional, IsUUID, Matches } from 'class-validator';
import { Request } from 'express';
import { IncomingMessage, Server } from 'http';
import Redis from 'ioredis';
import { Duplex } from 'stream';
import { Pool } from 'pg';
import { WebSocket, WebSocketServer } from 'ws';
import { PG_POOL } from '../common/db.module';
import { RolesGuard } from '../common/http';
import { TenantGuard } from '../common/tenant-guard';

const STREAM_PATH = '/v1/positions/stream';

/** A position older than this is still returned, but flagged stale (openapi.yaml, Position). */
const STALE_AFTER_SECONDS = Number(process.env.POSITION_STALE_SECONDS ?? 300);

class PositionsQuery {
  @IsOptional()
  @Matches(/^-?\d+(\.\d+)?(,-?\d+(\.\d+)?){3}$/, { message: 'bbox must be minLon,minLat,maxLon,maxLat' })
  bbox?: string;

  @IsOptional() @IsUUID()
  fleetId?: string;
}

@Injectable()
export class PositionsService {
  constructor(@Inject(PG_POOL) private readonly pool: Pool) {}

  async list(tenantId: string, query: PositionsQuery) {
    const params: unknown[] = [tenantId, STALE_AFTER_SECONDS];
    let where = 'p.tenant_id = $1';

    if (query.fleetId) {
      params.push(query.fleetId);
      where += ` AND v.fleet_id = $${params.length}`;
    }
    if (query.bbox) {
      const [minLon, minLat, maxLon, maxLat] = query.bbox.split(',').map(Number);
      params.push(minLon, maxLon, minLat, maxLat);
      const n = params.length;
      where += ` AND p.lon BETWEEN $${n - 3} AND $${n - 2} AND p.lat BETWEEN $${n - 1} AND $${n}`;
    }

    const { rows } = await this.pool.query(
      `SELECT p.vehicle_id AS "vehicleId", v.registration, p.lat, p.lon,
              p.speed_kph AS "speedKph", p.heading_deg AS "headingDeg", p.ignition_on AS "ignitionOn",
              p.recorded_at AS "recordedAt",
              (p.recorded_at < now() - ($2 * interval '1 second')) AS stale
       FROM vehicle_position_latest p
       JOIN vehicle v ON v.id = p.vehicle_id
       WHERE ${where}
       ORDER BY v.registration`,
      params,
    );
    return { data: rows, generatedAt: new Date().toISOString() };
  }
}

@Controller('v1/positions')
@UseGuards(TenantGuard, RolesGuard)
export class PositionsController {
  constructor(private readonly positions: PositionsService) {}

  @Get()
  list(@Req() req: Request, @Query() query: PositionsQuery) {
    return this.positions.list(req.tenant!.tenantId, query);
  }
}

/**
 * The WebSocket half. Authentication already happened at edge-gateway, which
 * validated the credential and asserted X-Trackline-Tenant on the upgrade
 * request; like every other fleet-api route, this trusts those headers and
 * requires them to be present.
 */
@Injectable()
export class PositionsStream implements OnApplicationBootstrap, OnApplicationShutdown {
  private readonly log = new Logger(PositionsStream.name);
  private readonly wss = new WebSocketServer({ noServer: true });
  private readonly clients = new Map<string, Set<WebSocket>>();
  private readonly alive = new WeakSet<WebSocket>();
  private sub?: Redis;
  private heartbeat?: NodeJS.Timeout;

  constructor(private readonly host: HttpAdapterHost) {}

  onApplicationBootstrap(): void {
    const [redisHost, redisPort] = (process.env.REDIS_ADDR ?? 'redis:6379').split(':');
    this.sub = new Redis({ host: redisHost, port: Number(redisPort ?? 6379), maxRetriesPerRequest: null });
    // Redis being down must not take fleet-api down; ioredis reconnects and resubscribes.
    this.sub.on('error', (err) => this.log.warn(`redis: ${err.message}`));
    this.sub.on('message', (channel, message) => this.broadcast(channel.slice('positions:'.length), message));

    (this.host.httpAdapter.getHttpServer() as Server).on('upgrade', (req, socket, head) =>
      this.onUpgrade(req, socket, head),
    );

    this.heartbeat = setInterval(() => {
      for (const set of this.clients.values()) {
        for (const ws of set) {
          if (!this.alive.has(ws)) {
            ws.terminate();
            continue;
          }
          this.alive.delete(ws);
          ws.ping();
        }
      }
    }, 30_000);
  }

  async onApplicationShutdown(): Promise<void> {
    if (this.heartbeat) clearInterval(this.heartbeat);
    for (const set of this.clients.values()) for (const ws of set) ws.close(1001, 'server shutting down');
    await this.sub?.quit().catch(() => undefined);
  }

  private onUpgrade(req: IncomingMessage, socket: Duplex, head: Buffer): void {
    const path = new URL(req.url ?? '/', 'http://localhost').pathname;
    if (path !== STREAM_PATH) {
      socket.destroy();
      return;
    }

    const tenantId = req.headers['x-trackline-tenant'];
    const subject = req.headers['x-trackline-subject'];
    if (typeof tenantId !== 'string' || !tenantId || typeof subject !== 'string' || !subject) {
      socket.write('HTTP/1.1 401 Unauthorized\r\nConnection: close\r\n\r\n');
      socket.destroy();
      return;
    }

    this.wss.handleUpgrade(req, socket, head, (ws) => this.attach(tenantId, ws));
  }

  private attach(tenantId: string, ws: WebSocket): void {
    let set = this.clients.get(tenantId);
    if (!set) {
      set = new Set();
      this.clients.set(tenantId, set);
      void this.sub?.subscribe(`positions:${tenantId}`).catch((e) => this.log.warn(`subscribe: ${e.message}`));
    }
    set.add(ws);
    this.alive.add(ws);
    ws.on('pong', () => this.alive.add(ws));
    ws.on('error', () => ws.terminate());
    ws.on('close', () => {
      set!.delete(ws);
      if (set!.size === 0) {
        this.clients.delete(tenantId);
        void this.sub?.unsubscribe(`positions:${tenantId}`).catch(() => undefined);
      }
    });
  }

  private broadcast(tenantId: string, message: string): void {
    for (const ws of this.clients.get(tenantId) ?? []) {
      // A slow reader loses frames rather than growing this process's memory;
      // positions are superseded by the next one for the same vehicle anyway.
      if (ws.readyState === WebSocket.OPEN && ws.bufferedAmount < 1 << 20) ws.send(message);
    }
  }
}

@Module({
  controllers: [PositionsController],
  providers: [PositionsService, PositionsStream],
})
export class PositionsModule {}
