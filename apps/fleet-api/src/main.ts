import 'reflect-metadata';
import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { ProblemFilter, validationPipe } from './common/problem';

async function bootstrap() {
  const app = await NestFactory.create(AppModule, { logger: ['log', 'warn', 'error'] });
  app.enableShutdownHooks();
  app.useGlobalPipes(validationPipe());
  app.useGlobalFilters(new ProblemFilter());

  app.getHttpAdapter().get('/healthz', (_req: unknown, res: { json: (b: unknown) => void }) =>
    res.json({ ready: true }),
  );

  const port = Number(process.env.PORT ?? 3000);
  await app.listen(port);
  // eslint-disable-next-line no-console
  console.log(`fleet-api listening on :${port}`);
}

bootstrap();
