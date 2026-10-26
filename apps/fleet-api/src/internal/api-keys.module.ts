// Internal endpoint for edge-gateway's API-key authentication (auth.md §1, machine
// clients). Not part of the public API and never proxied: edge-gateway forwards
// only /v1/*, and the platform must keep this service unreachable from outside the
// cluster (auth.md §3).
//
// The gateway sends SHA-256(secret), never the secret, and this endpoint answers
// only "who does this key belong to" — or 401. Which of "no such key", "wrong
// secret", "revoked", "expired" and "tenant suspended" applied is not revealed.
import { Body, Controller, HttpCode, Inject, Injectable, Module, Post, UnauthorizedException } from '@nestjs/common';
import { IsString, Matches, MaxLength, MinLength } from 'class-validator';
import { timingSafeEqual } from 'crypto';
import { Pool } from 'pg';
import { PG_POOL } from '../common/db.module';

class VerifyKeyBody {
  @IsString() @MinLength(1) @MaxLength(64) @Matches(/^[A-Za-z0-9_-]+$/)
  prefix!: string;

  @Matches(/^[0-9a-f]{64}$/)
  secretHash!: string;
}

@Injectable()
export class ApiKeysService {
  constructor(@Inject(PG_POOL) private readonly pool: Pool) {}

  async verify(prefix: string, secretHash: string) {
    const { rows } = await this.pool.query(
      `SELECT k.id, k.tenant_id AS "tenantId", k.secret_hash AS "secretHash", k.scopes, t.tier
       FROM api_key k JOIN tenant t ON t.id = k.tenant_id
       WHERE k.prefix = $1
         AND k.revoked_at IS NULL
         AND (k.expires_at IS NULL OR k.expires_at > now())
         AND t.status = 'active'`,
      [prefix],
    );
    const key = rows[0];

    // Compare even when the key is unknown, so the response time does not say whether the prefix exists.
    const stored = Buffer.from(key?.secretHash ?? '0'.repeat(64), 'hex');
    const presented = Buffer.from(secretHash, 'hex');
    const match = stored.length === presented.length && timingSafeEqual(stored, presented);
    if (!key || !match) throw new UnauthorizedException();

    await this.pool.query(
      `UPDATE api_key SET last_used_at = now()
       WHERE id = $1 AND (last_used_at IS NULL OR last_used_at < now() - interval '1 minute')`,
      [key.id],
    );

    return {
      tenantId: key.tenantId,
      tier: key.tier,
      subject: `apikey:${prefix}`,
      roles: key.scopes.includes('fleet:write') ? ['operator'] : ['viewer'],
    };
  }
}

@Controller('internal/api-keys')
export class ApiKeysController {
  constructor(private readonly keys: ApiKeysService) {}

  @Post('verify')
  @HttpCode(200)
  verify(@Body() body: VerifyKeyBody) {
    return this.keys.verify(body.prefix, body.secretHash);
  }
}

@Module({
  controllers: [ApiKeysController],
  providers: [ApiKeysService],
})
export class ApiKeysModule {}
