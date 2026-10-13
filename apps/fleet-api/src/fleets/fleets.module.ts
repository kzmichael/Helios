import { Module } from '@nestjs/common';
import { TenantsModule } from '../tenants/tenants.service';
import { FleetsService, VehiclesService } from './fleets.service';
import { FleetsController, VehiclesController } from './fleets.controller';

@Module({
  imports: [TenantsModule],
  controllers: [FleetsController, VehiclesController],
  providers: [FleetsService, VehiclesService],
  exports: [FleetsService, VehiclesService],
})
export class FleetsModule {}
