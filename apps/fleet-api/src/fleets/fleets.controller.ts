// HTTP surface for fleets and vehicles, matching apps/_contracts/openapi.yaml
// operationIds listFleets/createFleet/getFleet, listVehicles/createVehicle/getVehicle.
import {
  Body,
  Controller,
  Delete,
  Get,
  HttpCode,
  Param,
  Patch,
  Post,
  Query,
  Req,
  UseGuards,
} from '@nestjs/common';
import { Request } from 'express';
import { Audited } from '../common/audit';
import { MinRole, PageQuery, RolesGuard, UuidParam } from '../common/http';
import { TenantGuard } from '../common/tenant-guard';
import { FleetsService, VehiclesService } from './fleets.service';
import { FleetInput, FleetPatch, HistoryQuery, VehicleInput, VehicleListQuery, VehiclePatch } from './dto';

@Controller('v1/fleets')
@UseGuards(TenantGuard, RolesGuard)
export class FleetsController {
  constructor(private readonly fleets: FleetsService) {}

  @Get()
  list(@Req() req: Request, @Query() query: PageQuery) {
    return this.fleets.list(req.tenant!.tenantId, query.page, query.pageSize);
  }

  @Post()
  @MinRole('operator')
  @Audited('fleet.created', 'fleet')
  create(@Req() req: Request, @Body() body: FleetInput) {
    return this.fleets.create(req.tenant!.tenantId, body);
  }

  @Get(':fleetId')
  get(@Req() req: Request, @Param('fleetId', UuidParam) fleetId: string) {
    return this.fleets.getOrThrow(req.tenant!.tenantId, fleetId);
  }

  @Patch(':fleetId')
  @MinRole('operator')
  @Audited('fleet.updated', 'fleet')
  update(@Req() req: Request, @Param('fleetId', UuidParam) fleetId: string, @Body() body: FleetPatch) {
    return this.fleets.update(req.tenant!.tenantId, fleetId, body);
  }

  @Delete(':fleetId')
  @HttpCode(204)
  @MinRole('operator')
  @Audited('fleet.deleted', 'fleet')
  async remove(@Req() req: Request, @Param('fleetId', UuidParam) fleetId: string): Promise<void> {
    await this.fleets.remove(req.tenant!.tenantId, fleetId);
  }
}

@Controller('v1/vehicles')
@UseGuards(TenantGuard, RolesGuard)
export class VehiclesController {
  constructor(private readonly vehicles: VehiclesService) {}

  @Get()
  list(@Req() req: Request, @Query() query: VehicleListQuery) {
    return this.vehicles.list(req.tenant!.tenantId, query);
  }

  @Post()
  @MinRole('operator')
  @Audited('vehicle.created', 'vehicle')
  create(@Req() req: Request, @Body() body: VehicleInput) {
    return this.vehicles.create(req.tenant!.tenantId, body);
  }

  @Get(':vehicleId')
  get(@Req() req: Request, @Param('vehicleId', UuidParam) vehicleId: string) {
    return this.vehicles.getOrThrow(req.tenant!.tenantId, vehicleId);
  }

  @Patch(':vehicleId')
  @MinRole('operator')
  @Audited('vehicle.updated', 'vehicle')
  update(@Req() req: Request, @Param('vehicleId', UuidParam) vehicleId: string, @Body() body: VehiclePatch) {
    return this.vehicles.update(req.tenant!.tenantId, vehicleId, body);
  }

  @Get(':vehicleId/history')
  history(@Req() req: Request, @Param('vehicleId', UuidParam) vehicleId: string, @Query() query: HistoryQuery) {
    return this.vehicles.history(req.tenant!.tenantId, vehicleId, query);
  }
}
