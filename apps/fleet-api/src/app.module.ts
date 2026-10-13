import { Module } from '@nestjs/common';
import { DbModule } from './common/db.module';
import { FleetsModule } from './fleets/fleets.module';
import { GeofencesModule } from './geofences/geofences.module';
import { AlertsModule } from './alerts/alerts.module';
import { TenantsModule } from './tenants/tenants.service';
import { DevAuthModule } from './dev-auth/dev-auth.module';

const devOnlyModules = process.env.NODE_ENV === 'production' ? [] : [DevAuthModule];

@Module({
  imports: [DbModule, TenantsModule, FleetsModule, GeofencesModule, AlertsModule, ...devOnlyModules],
})
export class AppModule {}
