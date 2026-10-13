// Tenant isolation guard. Implements apps/_contracts/auth.md §4:
//
//   Cross-tenant access returns 404, never 403. A 403 confirms the resource
//   exists, which is an information leak.
//
// Every service-layer query must be scoped by tenantId taken from here — never
// from a path parameter, query string, or request body. The tenant id arrives
// via the X-Trackline-Tenant header, which is only ever set by edge-gateway
// after it has validated the caller's JWT (see edge-gateway/internal/auth).
import {
  CanActivate,
  ExecutionContext,
  Injectable,
  UnauthorizedException,
} from '@nestjs/common';
import { Request } from 'express';

export interface TenantContext {
  tenantId: string;
  subjectId: string;
  roles: string[];
}

declare module 'express' {
  interface Request {
    tenant?: TenantContext;
  }
}

@Injectable()
export class TenantGuard implements CanActivate {
  canActivate(context: ExecutionContext): boolean {
    const req = context.switchToHttp().getRequest<Request>();

    const tenantId = req.header('X-Trackline-Tenant');
    const subjectId = req.header('X-Trackline-Subject');
    const rolesHeader = req.header('X-Trackline-Roles') ?? '';

    // fleet-api does not re-validate the JWT — edge-gateway is the only trust
    // boundary (auth.md §3). It DOES require these headers to be present,
    // because a request with no gateway-asserted context should never reach
    // here in a properly network-isolated deployment. Its presence is a cheap
    // defence-in-depth check, not the primary control.
    if (!tenantId || !subjectId) {
      throw new UnauthorizedException({
        type: 'https://trackline.local/errors/missing-tenant-context',
        title: 'Missing tenant context',
        status: 401,
      });
    }

    req.tenant = {
      tenantId,
      subjectId,
      roles: rolesHeader.split(',').map((r) => r.trim()).filter(Boolean),
    };
    return true;
  }
}

/** Role hierarchy from auth.md §4. Higher index implies every lower privilege. */
const ROLE_ORDER = ['viewer', 'operator', 'admin', 'owner'];

export function hasAtLeastRole(roles: string[], required: string): boolean {
  const requiredIdx = ROLE_ORDER.indexOf(required);
  return roles.some((r) => ROLE_ORDER.indexOf(r) >= requiredIdx);
}
