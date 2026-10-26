import './common/pg-types';
import { Module } from '@nestjs/common';
import { APP_INTERCEPTOR } from '@nestjs/core';
import { AuditInterceptor } from './common/audit';
import { DbModule } from './common/db.module';
import { FleetsModule } from './fleets/fleets.module';
import { GeofencesModule } from './geofences/geofences.module';
import { AlertsModule } from './alerts/alerts.module';
import { TenantsModule } from './tenants/tenants.service';
import { DevAuthModule } from './dev-auth/dev-auth.module';
import { PositionsModule } from './positions/positions.module';
import { DriversModule } from './drivers/drivers.module';
import { BillingModule } from './billing/billing.module';
import { ApiKeysModule } from './internal/api-keys.module';

const devOnlyModules = process.env.NODE_ENV === 'production' ? [] : [DevAuthModule];

@Module({
  imports: [
    DbModule,
    TenantsModule,
    FleetsModule,
    GeofencesModule,
    AlertsModule,
    PositionsModule,
    DriversModule,
    BillingModule,
    ApiKeysModule,
    ...devOnlyModules,
  ],
  providers: [{ provide: APP_INTERCEPTOR, useClass: AuditInterceptor }],
})
export class AppModule {}
