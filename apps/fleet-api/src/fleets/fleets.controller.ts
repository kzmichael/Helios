// HTTP surface for fleets and vehicles, matching apps/_contracts/openapi.yaml
// operationIds listFleets/createFleet/getFleet, listVehicles/createVehicle/getVehicle.
import {
  Body,
  Controller,
  Get,
  Param,
  Post,
  Query,
  Req,
  UseGuards,
} from '@nestjs/common';
import { Request } from 'express';
import { TenantGuard } from '../common/tenant-guard';
import { FleetsService, VehiclesService, FleetInput, VehicleInput } from './fleets.service';

@Controller('v1/fleets')
@UseGuards(TenantGuard)
export class FleetsController {
  constructor(private readonly fleets: FleetsService) {}

  @Get()
  list(@Req() req: Request, @Query('page') page = '1', @Query('pageSize') pageSize = '50') {
    return this.fleets.list(req.tenant!.tenantId, Number(page), Number(pageSize));
  }

  @Post()
  create(@Req() req: Request, @Body() body: FleetInput) {
    return this.fleets.create(req.tenant!.tenantId, body);
  }

  @Get(':fleetId')
  get(@Req() req: Request, @Param('fleetId') fleetId: string) {
    return this.fleets.getOrThrow(req.tenant!.tenantId, fleetId);
  }
}

@Controller('v1/vehicles')
@UseGuards(TenantGuard)
export class VehiclesController {
  constructor(private readonly vehicles: VehiclesService) {}

  @Get()
  list(
    @Req() req: Request,
    @Query('page') page = '1',
    @Query('pageSize') pageSize = '50',
    @Query('fleetId') fleetId?: string,
  ) {
    return this.vehicles.list(req.tenant!.tenantId, Number(page), Number(pageSize), fleetId);
  }

  @Post()
  create(@Req() req: Request, @Body() body: VehicleInput) {
    return this.vehicles.create(req.tenant!.tenantId, body);
  }

  @Get(':vehicleId')
  get(@Req() req: Request, @Param('vehicleId') vehicleId: string) {
    return this.vehicles.getOrThrow(req.tenant!.tenantId, vehicleId);
  }
}
