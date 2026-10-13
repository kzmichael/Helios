// Phase 0 only: there is no real identity provider yet (auth.md §1 table).
// fleet-api mints dev tokens signed with a local RS256 key pair so edge-gateway
// has something real to validate against. This entire file is deleted the
// moment Dex/Keycloak (Phase 1) or Entra ID (Phase 5) exists — do not build on
// top of it.
import { Controller, Get, Injectable, Module, Param } from '@nestjs/common';
import * as jwt from 'jsonwebtoken';
import * as fs from 'fs';
import * as path from 'path';

const DEV_KEY_DIR = path.join(__dirname, '..', '..', '.dev-keys');

@Injectable()
export class DevTokenService {
  private privateKey: string;
  private publicJwk: Record<string, unknown>;

  constructor() {
    // A real keypair is generated once by `scripts/dev-token-keys.sh` (operator
    // owned) — this service refuses to fabricate one silently in code, since a
    // key generated implicitly and never rotated is exactly the anti-pattern
    // auth.md warns against.
    const keyPath = path.join(DEV_KEY_DIR, 'private.pem');
    if (!fs.existsSync(keyPath)) {
      throw new Error(
        `Dev signing key not found at ${keyPath}. Generate one before starting fleet-api in dev mode.`,
      );
    }
    this.privateKey = fs.readFileSync(keyPath, 'utf8');
    this.publicJwk = JSON.parse(fs.readFileSync(path.join(DEV_KEY_DIR, 'jwks.json'), 'utf8'));
  }

  mint(tenantId: string, subjectId: string, tier: string, roles: string[]): string {
    return jwt.sign(
      {
        tid: tenantId,
        tier,
        roles,
        scp: 'fleet:read fleet:write telemetry:write alerts:read',
      },
      this.privateKey,
      {
        algorithm: 'RS256',
        issuer: 'https://auth.trackline.local/',
        audience: 'trackline-api',
        subject: subjectId,
        expiresIn: '15m',
        keyid: 'dev-1',
      },
    );
  }

  jwks() {
    return this.publicJwk;
  }
}

/** Dev-only endpoints. Never enable this module outside local development. */
@Controller()
export class DevTokenController {
  constructor(private readonly tokens: DevTokenService) {}

  @Get('.well-known/jwks.json')
  getJwks() {
    return this.tokens.jwks();
  }

  // Convenience for manual testing: GET /dev/token/:tenantId/:subjectId/:tier
  @Get('dev/token/:tenantId/:subjectId/:tier')
  mint(
    @Param('tenantId') tenantId: string,
    @Param('subjectId') subjectId: string,
    @Param('tier') tier: string,
  ) {
    return { access_token: this.tokens.mint(tenantId, subjectId, tier, ['operator']) };
  }
}

@Module({
  controllers: [DevTokenController],
  providers: [DevTokenService],
})
export class DevAuthModule {}
