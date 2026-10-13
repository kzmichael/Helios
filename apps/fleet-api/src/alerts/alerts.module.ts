// Alert list/acknowledge. Backs the dashboard's alert feed
// (apps/_contracts/openapi.yaml operationIds listAlerts/acknowledgeAlert).
import {
  Body,
  ConflictException,
  Controller,
  Get,
  Inject,
  Injectable,
  Module,
  NotFoundException,
  Param,
  Post,
  Query,
  Req,
  UseGuards,
} from '@nestjs/common';
import { Request } from 'express';
import { Pool } from 'pg';
import { PG_POOL } from '../common/db.module';
import { TenantGuard } from '../common/tenant-guard';

@Injectable()
export class AlertsService {
  constructor(@Inject(PG_POOL) private readonly pool: Pool) {}

  async list(
    tenantId: string,
    page: number,
    pageSize: number,
    severity?: string,
    acknowledged?: boolean,
    vehicleId?: string,
  ) {
    const params: unknown[] = [tenantId];
    let where = 'tenant_id = $1';

    if (severity) {
      params.push(severity);
      where += ` AND severity = $${params.length}`;
    }
    if (acknowledged !== undefined) {
      where += acknowledged ? ' AND acknowledged_at IS NOT NULL' : ' AND acknowledged_at IS NULL';
    }
    if (vehicleId) {
      params.push(vehicleId);
      where += ` AND vehicle_id = $${params.length}`;
    }

    const offset = (page - 1) * pageSize;
    params.push(pageSize, offset);

    const { rows } = await this.pool.query(
      `SELECT id, rule_id AS "ruleId", vehicle_id AS "vehicleId", geofence_id AS "geofenceId",
              alert_type AS "alertType", severity, summary, payload,
              position_lat AS "positionLat", position_lon AS "positionLon",
              triggered_at AS "triggeredAt", acknowledged_at AS "acknowledgedAt",
              resolved_at AS "resolvedAt"
       FROM alert WHERE ${where}
       ORDER BY triggered_at DESC LIMIT $${params.length - 1} OFFSET $${params.length}`,
      params,
    );
    return { data: rows, page, pageSize };
  }

  /**
   * Idempotent-in-intent: acknowledging an already-acknowledged alert is a 409,
   * not a silent success — the caller likely raced another operator and should
   * know that.
   */
  async acknowledge(tenantId: string, alertId: string, subjectId: string) {
    const { rows: existing } = await this.pool.query(
      `SELECT id, acknowledged_at FROM alert WHERE tenant_id = $1 AND id = $2`,
      [tenantId, alertId],
    );
    if (!existing[0]) throw new NotFoundException();
    if (existing[0].acknowledged_at) {
      throw new ConflictException({
        type: 'https://trackline.local/errors/already-acknowledged',
        title: 'Already acknowledged',
        status: 409,
      });
    }

    const { rows } = await this.pool.query(
      `UPDATE alert SET acknowledged_at = now(), acknowledged_by = $3
       WHERE tenant_id = $1 AND id = $2
       RETURNING id, alert_type AS "alertType", severity, summary,
                 triggered_at AS "triggeredAt", acknowledged_at AS "acknowledgedAt"`,
      [tenantId, alertId, subjectId],
    );
    return rows[0];
  }
}

@Controller('v1/alerts')
@UseGuards(TenantGuard)
export class AlertsController {
  constructor(private readonly alerts: AlertsService) {}

  @Get()
  list(
    @Req() req: Request,
    @Query('page') page = '1',
    @Query('pageSize') pageSize = '50',
    @Query('severity') severity?: string,
    @Query('acknowledged') acknowledged?: string,
    @Query('vehicleId') vehicleId?: string,
  ) {
    return this.alerts.list(
      req.tenant!.tenantId,
      Number(page),
      Number(pageSize),
      severity,
      acknowledged === undefined ? undefined : acknowledged === 'true',
      vehicleId,
    );
  }

  @Post(':alertId/acknowledge')
  acknowledge(@Req() req: Request, @Param('alertId') alertId: string) {
    return this.alerts.acknowledge(req.tenant!.tenantId, alertId, req.tenant!.subjectId);
  }
}

@Module({
  controllers: [AlertsController],
  providers: [AlertsService],
})
export class AlertsModule {}
