// Postgres connection pool. This is the module in which a pool-exhaustion bug
// will eventually surface under load — that's expected, and it's why
// docs/05_STUDY_GUIDE.md lists connection pooling as a Phase 1 study topic.
import { Global, Injectable, Module, OnApplicationShutdown } from '@nestjs/common';
import { Pool } from 'pg';

export const PG_POOL = 'PG_POOL';

@Injectable()
class FleetApiPool extends Pool implements OnApplicationShutdown {
  constructor() {
    super({
      connectionString: process.env.DATABASE_URL,
      // Deliberately small in Phase 0. limits.md §7 caps per-tenant
      // concurrent queries at ceil(pool/4) — a small pool makes this cap
      // visible during load testing.
      max: Number(process.env.PG_POOL_MAX ?? 10),
      idleTimeoutMillis: 30_000,
      connectionTimeoutMillis: 5_000,
    });
  }

  async onApplicationShutdown(): Promise<void> {
    await this.end();
  }
}
@Global()
@Module({
  providers: [
    {
      provide: PG_POOL,
      useClass: FleetApiPool,
    },
  ],
  exports: [PG_POOL],
})
export class DbModule {}
