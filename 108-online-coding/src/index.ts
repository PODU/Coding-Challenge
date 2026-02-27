import path from 'node:path';
import { createApp } from './api/app';
import { config } from './config';
import { ProblemStore } from './problems/store';

async function main(): Promise<void> {
  const problemsDir = path.resolve(__dirname, '../../problems');
  const problems = await ProblemStore.fromDir(problemsDir);

  const { app } = createApp({ problems, serveWeb: true });

  app.listen(config.port, () => {
    // eslint-disable-next-line no-console
    console.log(
      `Online Coding Platform v${config.version} listening on http://localhost:${config.port}`,
    );
    // eslint-disable-next-line no-console
    console.log(
      `  Loaded ${problems.size()} problem(s). Default timeout ${config.defaultTimeoutMs}ms, ` +
        `output cap ${config.outputLimitBytes} bytes, concurrency ${config.maxConcurrency}.`,
    );
  });
}

main().catch((err) => {
  // eslint-disable-next-line no-console
  console.error('Fatal startup error:', err);
  process.exit(1);
});
