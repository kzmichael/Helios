// Audit trail (auth.md §8): every state-changing operation writes to audit_log with
// actor kind, actor id, action, target and the request's trace id.
//
// Handlers opt in with @Audited(action, targetType). The entry is written after the
// operation succeeded, from the gateway-asserted identity — never from the request body.
// Credentials are never recorded; the metadata holds only the route and the trace id.
import { CallHandler, ExecutionContext, Inject, Injectable, Logger, NestInterceptor, SetMetadata } from '@nestjs/common';
import { Reflector } from '@nestjs/core';
import { Request } from 'express';
import { Observable, mergeMap } from 'rxjs';
import { Pool } from 'pg';
import { PG_POOL } from './db.module';

const AUDIT = 'audit';

export const Audited = (action: string, targetType: string) => SetMetadata(AUDIT, { action, targetType });

@Injectable()
export class AuditInterceptor implements NestInterceptor {
  private readonly log = new Logger(AuditInterceptor.name);

  constructor(
    private readonly reflector: Reflector,
    @Inject(PG_POOL) private readonly pool: Pool,
  ) {}

  intercept(context: ExecutionContext, next: CallHandler): Observable<unknown> {
    const meta = this.reflector.get<{ action: string; targetType: string } | undefined>(AUDIT, context.getHandler());
    if (!meta) return next.handle();

    const req = context.switchToHttp().getRequest<Request>();
    return next.handle().pipe(
      mergeMap(async (result: unknown) => {
        const tenant = req.tenant;
        if (tenant) {
          const targetId = (result as { id?: string } | undefined)?.id ?? (req.params as Record<string, string>)[Object.keys(req.params)[0]];
          try {
            await this.pool.query(
              `INSERT INTO audit_log (tenant_id, actor_kind, actor_id, action, target_type, target_id, metadata)
               VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)`,
              [
                tenant.tenantId,
                tenant.subjectId.startsWith('apikey:') ? 'service' : 'user',
                tenant.subjectId,
                meta.action,
                meta.targetType,
                targetId ?? null,
                JSON.stringify({ method: req.method, route: req.route?.path, traceId: req.header('X-Trackline-Request-Id') ?? null }),
              ],
            );
          } catch (err) {
            // The operation has already committed; failing the response would mislead the
            // caller. Say so loudly instead of dropping the entry silently.
            this.log.error(`audit entry lost for ${meta.action} on ${targetId}: ${(err as Error).message}`);
          }
        }
        return result;
      }),
    );
  }
}
