// Small pieces every controller shares: role checks, id params, paging.
import {
  CanActivate,
  ExecutionContext,
  ForbiddenException,
  Injectable,
  NotFoundException,
  PipeTransform,
  SetMetadata,
} from '@nestjs/common';
import { Reflector } from '@nestjs/core';
import { Transform, Type } from 'class-transformer';
import { IsInt, IsOptional, Max, Min } from 'class-validator';
import { Request } from 'express';
import { hasAtLeastRole } from './tenant-guard';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function isUuid(v: unknown): v is string {
  return typeof v === 'string' && UUID.test(v);
}

/**
 * A malformed id cannot name a row, so it is "not found" — the same answer as for
 * another tenant's id. Answering 400 would let a caller tell the two cases apart.
 */
@Injectable()
export class UuidParam implements PipeTransform<string, string> {
  transform(value: string): string {
    if (!isUuid(value)) throw new NotFoundException();
    return value;
  }
}

export const MIN_ROLE = 'minRole';
/** Minimum role for a handler, from the auth.md §4 table. Reads need none. */
export const MinRole = (role: 'viewer' | 'operator' | 'admin' | 'owner') => SetMetadata(MIN_ROLE, role);

/** Runs after TenantGuard, which is what populates req.tenant. */
@Injectable()
export class RolesGuard implements CanActivate {
  constructor(private readonly reflector: Reflector) {}

  canActivate(context: ExecutionContext): boolean {
    const required = this.reflector.getAllAndOverride<string>(MIN_ROLE, [context.getHandler(), context.getClass()]);
    if (!required) return true;

    const req = context.switchToHttp().getRequest<Request>();
    if (!hasAtLeastRole(req.tenant?.roles ?? [], required)) {
      throw new ForbiddenException({
        type: 'https://trackline.local/errors/forbidden',
        title: 'Insufficient role',
        status: 403,
        detail: `This action needs the ${required} role.`,
      });
    }
    return true;
  }
}

/** page / pageSize from openapi.yaml: pageSize 1..200, default 50. */
export class PageQuery {
  @IsOptional()
  @Type(() => Number)
  @IsInt()
  @Min(1)
  page: number = 1;

  @IsOptional()
  @Type(() => Number)
  @IsInt()
  @Min(1)
  @Max(200)
  pageSize: number = 50;
}

/** "true"/"false" query strings to booleans; anything else is left for validation to reject. */
export const ToBoolean = () =>
  Transform(({ value }) => (value === 'true' ? true : value === 'false' ? false : value));
