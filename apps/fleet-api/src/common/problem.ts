// RFC 9457 problem responses (openapi.yaml, schema Problem). Every error leaves
// fleet-api as application/problem+json, whatever raised it.
import {
  ArgumentsHost,
  BadRequestException,
  Catch,
  ExceptionFilter,
  HttpException,
  Logger,
  ValidationError,
  ValidationPipe,
} from '@nestjs/common';
import { Request, Response } from 'express';

const TYPE_BASE = 'https://trackline.local/errors/';

const DEFAULTS: Record<number, [slug: string, title: string]> = {
  400: ['bad-request', 'Malformed request'],
  401: ['unauthenticated', 'Missing or invalid credential'],
  403: ['forbidden', 'Insufficient role'],
  404: ['not-found', 'Not found'],
  409: ['conflict', 'Conflicts with existing state'],
  413: ['payload-too-large', 'Payload too large'],
  422: ['unprocessable', 'Unprocessable request'],
};

/** Postgres errors that mean "the caller sent something wrong", not "we broke". */
function statusForPgError(code: unknown): number | undefined {
  switch (code) {
    case '23505': // unique_violation
      return 409;
    case '22P02': // invalid_text_representation (e.g. a malformed uuid)
    case '22003': // numeric_value_out_of_range
    case '22007': // invalid_datetime_format
      return 400;
    default:
      return undefined;
  }
}

@Catch()
export class ProblemFilter implements ExceptionFilter {
  private readonly log = new Logger('ProblemFilter');

  catch(exception: unknown, host: ArgumentsHost): void {
    const http = host.switchToHttp();
    const res = http.getResponse<Response>();
    const req = http.getRequest<Request>();

    let status = 500;
    let body: Record<string, unknown> = {};

    if (exception instanceof HttpException) {
      status = exception.getStatus();
      const raw = exception.getResponse();
      if (typeof raw === 'string') {
        body = { detail: raw };
      } else {
        const { statusCode: _s, error: _e, message, ...rest } = raw as Record<string, unknown>;
        body = { ...rest };
        if (message !== undefined && body.detail === undefined) {
          body.detail = Array.isArray(message) ? message.join('; ') : message;
        }
      }
    } else {
      const pgStatus = statusForPgError((exception as { code?: unknown })?.code);
      if (pgStatus) {
        status = pgStatus;
        body = { detail: pgStatus === 409 ? 'The request conflicts with an existing resource.' : 'A value in the request is not valid.' };
      } else {
        this.log.error(exception instanceof Error ? (exception.stack ?? exception.message) : String(exception));
      }
    }

    const [slug, title] = DEFAULTS[status] ?? ['internal-error', 'Internal error'];
    const problem = {
      type: `${TYPE_BASE}${slug}`,
      title,
      ...body,
      status,
      traceId: req.header('X-Trackline-Request-Id'),
    };

    res.status(status).type('application/problem+json').send(JSON.stringify(problem));
  }
}

function flatten(errors: ValidationError[], prefix = ''): string[] {
  return errors.flatMap((e) => {
    const path = prefix ? `${prefix}.${e.property}` : e.property;
    const own = Object.values(e.constraints ?? {}).map((m) => m);
    return [...own, ...flatten(e.children ?? [], path)];
  });
}

/** Validates request bodies and queries against the class-validator DTOs. */
export function validationPipe(): ValidationPipe {
  return new ValidationPipe({
    transform: true,
    whitelist: true,
    exceptionFactory: (errors) =>
      new BadRequestException({
        type: `${TYPE_BASE}bad-request`,
        title: 'Malformed request',
        status: 400,
        detail: flatten(errors).join('; '),
      }),
  });
}
