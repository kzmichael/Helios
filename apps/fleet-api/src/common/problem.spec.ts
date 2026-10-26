import { ArgumentsHost, BadRequestException, ConflictException, Logger, NotFoundException } from '@nestjs/common';
import { ProblemFilter, validationPipe } from './problem';
import { VehicleInput } from '../fleets/dto';

function run(exception: unknown) {
  const sent: { status?: number; type?: string; body?: string } = {};
  const res = {
    status(s: number) {
      sent.status = s;
      return this;
    },
    type(t: string) {
      sent.type = t;
      return this;
    },
    send(b: string) {
      sent.body = b;
    },
  };
  const host = {
    switchToHttp: () => ({
      getResponse: () => res,
      getRequest: () => ({ header: (n: string) => (n === 'X-Trackline-Request-Id' ? 'req-1' : undefined) }),
    }),
  } as unknown as ArgumentsHost;
  new ProblemFilter().catch(exception, host);
  return { ...sent, problem: JSON.parse(sent.body ?? '{}') };
}

describe('ProblemFilter', () => {
  it('turns a bare 404 into problem+json', () => {
    const r = run(new NotFoundException());
    expect(r.status).toBe(404);
    expect(r.type).toBe('application/problem+json');
    expect(r.problem).toMatchObject({ type: 'https://trackline.local/errors/not-found', status: 404, traceId: 'req-1' });
  });

  it('keeps a body that is already a problem document', () => {
    const r = run(new ConflictException({ type: 'https://trackline.local/errors/already-acknowledged', title: 'Already acknowledged', status: 409 }));
    expect(r.problem).toMatchObject({ type: 'https://trackline.local/errors/already-acknowledged', title: 'Already acknowledged', status: 409 });
  });

  it('maps a Postgres unique violation to 409 and a bad uuid to 400', () => {
    expect(run({ code: '23505' }).status).toBe(409);
    expect(run({ code: '22P02' }).status).toBe(400);
  });

  it('does not leak the message of an unexpected error', () => {
    jest.spyOn(Logger.prototype, 'error').mockImplementation(() => undefined);
    const r = run(new Error('connection to 10.0.0.5:5432 refused'));
    expect(r.status).toBe(500);
    expect(r.body).not.toContain('10.0.0.5');
  });
});

describe('validationPipe', () => {
  const pipe = validationPipe();
  const meta = { type: 'body' as const, metatype: VehicleInput };

  it('accepts a valid vehicle', async () => {
    const out = await pipe.transform(
      { fleetId: '1f000000-0000-4000-8000-000000000001', externalRef: 'V1', registration: 'KA01', unknown: 'dropped' },
      meta,
    );
    expect(out).not.toHaveProperty('unknown');
  });

  it('rejects with a 400 problem naming every failing field', async () => {
    await expect(pipe.transform({ fleetId: 'nope', externalRef: '', speedLimitKph: 500 }, meta)).rejects.toBeInstanceOf(
      BadRequestException,
    );
    try {
      await pipe.transform({ fleetId: 'nope', externalRef: '', speedLimitKph: 500 }, meta);
    } catch (e) {
      const detail = (e as BadRequestException).getResponse() as { detail: string };
      expect(detail.detail).toContain('fleetId');
      expect(detail.detail).toContain('externalRef');
      expect(detail.detail).toContain('speedLimitKph');
    }
  });
});
