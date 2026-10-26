// Usage and invoices (openapi.yaml getUsage / listInvoices). Billing is the one
// area auth.md §4 reserves for the owner role.
//
// Read-only here: usage_daily is written by the pipeline's usage-collector, and
// invoices by the billing-batch job.
import { BadRequestException, Controller, Get, Inject, Injectable, Module, Query, Req, UseGuards } from '@nestjs/common';
import { IsDateString } from 'class-validator';
import { Request } from 'express';
import { Pool } from 'pg';
import { PG_POOL } from '../common/db.module';
import { MinRole, PageQuery, RolesGuard } from '../common/http';
import { TenantGuard } from '../common/tenant-guard';

class UsageQuery {
  @IsDateString({ strict: false })
  from!: string;

  @IsDateString({ strict: false })
  to!: string;
}

@Injectable()
export class BillingService {
  constructor(@Inject(PG_POOL) private readonly pool: Pool) {}

  async usage(tenantId: string, from: string, to: string) {
    if (from > to) throw new BadRequestException({ detail: '"from" must not be after "to".' });
    const { rows } = await this.pool.query(
      `SELECT usage_date::text AS "usageDate", telemetry_points AS "telemetryPoints",
              active_devices AS "activeDevices", api_requests AS "apiRequests",
              storage_bytes AS "storageBytes"
       FROM usage_daily
       WHERE tenant_id = $1 AND usage_date BETWEEN $2::date AND $3::date
       ORDER BY usage_date`,
      [tenantId, from.slice(0, 10), to.slice(0, 10)],
    );
    return { data: rows };
  }

  async invoices(tenantId: string, page: number, pageSize: number) {
    const offset = (page - 1) * pageSize;
    const { rows } = await this.pool.query(
      `SELECT i.id, i.number, i.period_start::text AS "periodStart", i.period_end::text AS "periodEnd",
              trim(i.currency) AS currency, i.subtotal_minor AS "subtotalMinor", i.tax_minor AS "taxMinor",
              i.total_minor AS "totalMinor", i.status, i.issued_at AS "issuedAt",
              COALESCE((SELECT json_agg(json_build_object(
                          'description', l.description, 'quantity', l.quantity,
                          'unitPriceMinor', l.unit_price_minor, 'amountMinor', l.amount_minor)
                        ORDER BY l.line_no)
                        FROM invoice_line l WHERE l.invoice_id = i.id), '[]'::json) AS lines
       FROM invoice i WHERE i.tenant_id = $1
       ORDER BY i.period_start DESC, i.created_at DESC LIMIT $2 OFFSET $3`,
      [tenantId, pageSize, offset],
    );
    const { rows: countRows } = await this.pool.query(
      `SELECT count(*)::int AS total FROM invoice WHERE tenant_id = $1`,
      [tenantId],
    );
    return { data: rows, page, pageSize, total: countRows[0].total };
  }
}

@Controller('v1')
@UseGuards(TenantGuard, RolesGuard)
@MinRole('owner')
export class BillingController {
  constructor(private readonly billing: BillingService) {}

  @Get('usage')
  usage(@Req() req: Request, @Query() query: UsageQuery) {
    return this.billing.usage(req.tenant!.tenantId, query.from, query.to);
  }

  @Get('invoices')
  invoices(@Req() req: Request, @Query() query: PageQuery) {
    return this.billing.invoices(req.tenant!.tenantId, query.page, query.pageSize);
  }
}

@Module({
  controllers: [BillingController],
  providers: [BillingService],
})
export class BillingModule {}
