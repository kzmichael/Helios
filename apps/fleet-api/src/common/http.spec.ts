import { ExecutionContext, ForbiddenException, NotFoundException } from '@nestjs/common';
import { Reflector } from '@nestjs/core';
import { plainToInstance } from 'class-transformer';
import { validate } from 'class-validator';
import { MIN_ROLE, PageQuery, RolesGuard, UuidParam, isUuid } from './http';

function ctx(roles: string[], minRole?: string): ExecutionContext {
  const handler = () => undefined;
  if (minRole) Reflect.defineMetadata(MIN_ROLE, minRole, handler);
  return {
    getHandler: () => handler,
    getClass: () => class {},
    switchToHttp: () => ({ getRequest: () => ({ tenant: { tenantId: 't', subjectId: 's', roles } }) }),
  } as unknown as ExecutionContext;
}

describe('RolesGuard', () => {
  const guard = new RolesGuard(new Reflector());

  it('lets any role through when the handler states no minimum', () => {
    expect(guard.canActivate(ctx(['viewer']))).toBe(true);
  });

  it('follows the auth.md §4 hierarchy', () => {
    expect(guard.canActivate(ctx(['operator'], 'operator'))).toBe(true);
    expect(guard.canActivate(ctx(['admin'], 'operator'))).toBe(true);
    expect(guard.canActivate(ctx(['owner'], 'owner'))).toBe(true);
  });

  it('refuses a role below the minimum with 403', () => {
    expect(() => guard.canActivate(ctx(['viewer'], 'operator'))).toThrow(ForbiddenException);
    expect(() => guard.canActivate(ctx(['operator'], 'owner'))).toThrow(ForbiddenException);
    expect(() => guard.canActivate(ctx([], 'operator'))).toThrow(ForbiddenException);
  });
});

describe('UuidParam', () => {
  const pipe = new UuidParam();

  it('passes a uuid through', () => {
    const id = '1f000000-0000-4000-8000-000000000001';
    expect(pipe.transform(id)).toBe(id);
  });

  it('answers 404, not 400, for anything that cannot be an id', () => {
    for (const bad of ['', 'abc', '../etc/passwd', "1' OR '1'='1"]) {
      expect(() => pipe.transform(bad)).toThrow(NotFoundException);
    }
    expect(isUuid(undefined)).toBe(false);
  });
});

describe('PageQuery', () => {
  const check = async (raw: Record<string, string>) => validate(plainToInstance(PageQuery, raw));

  it('defaults to page 1 of 50', async () => {
    const q = plainToInstance(PageQuery, {});
    expect([q.page, q.pageSize]).toEqual([1, 50]);
    expect(await check({})).toHaveLength(0);
  });

  it('coerces query strings', () => {
    const q = plainToInstance(PageQuery, { page: '3', pageSize: '25' });
    expect([q.page, q.pageSize]).toEqual([3, 25]);
  });

  it('enforces the limits.md page-size cap of 200', async () => {
    expect(await check({ pageSize: '200' })).toHaveLength(0);
    expect(await check({ pageSize: '201' })).toHaveLength(1);
    expect(await check({ pageSize: '0' })).toHaveLength(1);
    expect(await check({ page: '0' })).toHaveLength(1);
    expect(await check({ page: 'abc' })).toHaveLength(1);
  });
});
