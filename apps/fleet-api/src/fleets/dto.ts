// Request shapes for fleets and vehicles; the constraints are the ones in
// apps/_contracts/openapi.yaml (FleetInput, VehicleInput and the list parameters).
import { Type } from 'class-transformer';
import {
  IsDateString,
  IsIn,
  IsInt,
  IsNumber,
  IsOptional,
  IsString,
  IsUUID,
  Max,
  MaxLength,
  Min,
  MinLength,
} from 'class-validator';
import { PageQuery } from '../common/http';

export const VEHICLE_TYPES = ['truck', 'van', 'car', 'bike', 'trailer'] as const;
export const VEHICLE_STATUSES = ['active', 'maintenance', 'retired'] as const;

export class FleetInput {
  @IsString() @MinLength(1) @MaxLength(120)
  name!: string;

  @IsOptional() @IsString() @MaxLength(500)
  description?: string;

  @IsOptional() @IsNumber() @Min(-90) @Max(90)
  depotLat?: number;

  @IsOptional() @IsNumber() @Min(-180) @Max(180)
  depotLon?: number;
}

/** PATCH: the same fields, none required. */
export class FleetPatch {
  @IsOptional() @IsString() @MinLength(1) @MaxLength(120)
  name?: string;

  @IsOptional() @IsString() @MaxLength(500)
  description?: string;

  @IsOptional() @IsNumber() @Min(-90) @Max(90)
  depotLat?: number;

  @IsOptional() @IsNumber() @Min(-180) @Max(180)
  depotLon?: number;
}

export class VehicleInput {
  @IsUUID()
  fleetId!: string;

  @IsString() @MinLength(1) @MaxLength(64)
  externalRef!: string;

  @IsString() @MinLength(1) @MaxLength(32)
  registration!: string;

  @IsOptional() @IsString() @MaxLength(64)
  make?: string;

  @IsOptional() @IsString() @MaxLength(64)
  model?: string;

  @IsOptional() @IsInt() @Min(1950) @Max(2100)
  year?: number;

  @IsOptional() @IsIn(VEHICLE_TYPES)
  vehicleType?: string;

  @IsOptional() @IsIn(VEHICLE_STATUSES)
  status?: string;

  @IsOptional() @IsNumber() @Min(5) @Max(200)
  speedLimitKph?: number;
}

export class VehiclePatch {
  @IsOptional() @IsUUID()
  fleetId?: string;

  @IsOptional() @IsString() @MinLength(1) @MaxLength(64)
  externalRef?: string;

  @IsOptional() @IsString() @MinLength(1) @MaxLength(32)
  registration?: string;

  @IsOptional() @IsString() @MaxLength(64)
  make?: string;

  @IsOptional() @IsString() @MaxLength(64)
  model?: string;

  @IsOptional() @IsInt() @Min(1950) @Max(2100)
  year?: number;

  @IsOptional() @IsIn(VEHICLE_TYPES)
  vehicleType?: string;

  @IsOptional() @IsIn(VEHICLE_STATUSES)
  status?: string;

  @IsOptional() @IsNumber() @Min(5) @Max(200)
  speedLimitKph?: number;
}

export class VehicleListQuery extends PageQuery {
  @IsOptional() @IsUUID()
  fleetId?: string;

  @IsOptional() @IsIn(VEHICLE_STATUSES)
  status?: string;

  @IsOptional() @IsString() @MaxLength(64)
  q?: string;

  @IsOptional() @IsIn(['updatedAt', 'registration', 'externalRef'])
  sort: string = 'updatedAt';
}

export class HistoryQuery {
  @IsDateString()
  from!: string;

  @IsDateString()
  to!: string;

  @IsOptional() @Type(() => Number) @IsInt() @Min(1) @Max(5000)
  limit: number = 1000;
}
