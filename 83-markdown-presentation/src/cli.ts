#!/usr/bin/env node
/**
 * `mdpresent <init|serve|build|export> [args]`
 *
 * Exits with 1 for user errors and 2 for internal errors.
 */
import { existsSync } from 'node:fs';
import { resolve } from 'node:path';
import { startServer } from './shell/server.js';
import { buildStatic } from './shell/build.js';
import { scaffold } from './shell/scaffold.js';
import { exportPdf, PdfUnavailableError } from './shell/pdf-export.js';

interface ParsedArgs {
  _: string[];
  flags: Record<string, string | boolean>;
}

function parseArgs(argv: string[]): ParsedArgs {
  const _: string[] = [];
  const flags: Record<string, string | boolean> = {};
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i]!;
    if (a.startsWith('--')) {
      const key = a.slice(2);
      const next = argv[i + 1];
      if (next !== undefined && !next.startsWith('--')) {
        flags[key] = next;
        i++;
      } else {
        flags[key] = true;
      }
    } else {
      _.push(a);
    }
  }
  return { _, flags };
}

const USAGE = `mdpresent — Markdown presentation tool

Usage:
  mdpresent init <dir>              Scaffold a new presentation project
  mdpresent serve [file|dir]        Dev server with live reload (default ./slides)
      --port <n>                    Port (default 3000)
      --template-dir <path>         Custom templates (falls back to defaults)
      --no-open                     Do not auto-open the browser
  mdpresent build [file|dir]        Render a static deck
      --out <dir>                   Output dir (default ./dist)
      --template-dir <path>
  mdpresent export [file|dir]       Render to PDF
      --out <file.pdf>              Output PDF (default ./out.pdf)
      --template-dir <path>
`;

function defaultSource(arg: string | undefined): string {
  if (arg) return arg;
  if (existsSync('slides')) return 'slides';
  if (existsSync('slides.md')) return 'slides.md';
  return 'slides';
}

function fail(message: string, code: number): never {
  process.stderr.write(message + '\n');
  process.exit(code);
}

async function main(): Promise<void> {
  const { _, flags } = parseArgs(process.argv.slice(2));
  const command = _[0];

  if (!command || command === 'help' || flags.help) {
    process.stdout.write(USAGE);
    return;
  }

  const templateDir = typeof flags['template-dir'] === 'string' ? flags['template-dir'] : undefined;

  switch (command) {
    case 'init': {
      const dir = _[1];
      if (!dir) fail('Error: `init` requires a target directory.\n\n' + USAGE, 1);
      const result = scaffold({ dir: dir as string });
      process.stdout.write(
        `Scaffolded presentation in ${resolve(dir as string)}\n` +
          `  created ${result.created.length} item(s); git ${result.gitInitialized ? 'initialized' : 'not initialized'}\n` +
          `Next: cd ${dir} && mdpresent serve\n`,
      );
      return;
    }

    case 'serve': {
      const source = defaultSource(_[1]);
      if (!existsSync(source)) fail(`Error: source not found: ${source}`, 1);
      const port = typeof flags.port === 'string' ? Number(flags.port) : 3000;
      if (Number.isNaN(port)) fail('Error: --port must be a number.', 1);
      const handle = await startServer({
        source,
        port,
        ...(templateDir !== undefined ? { templateDir } : {}),
        watch: templateDir ? [templateDir] : [],
      });
      process.stdout.write(`mdpresent serving ${source} at ${handle.url}\n`);
      process.stdout.write('Press Ctrl+C to stop.\n');
      process.on('SIGINT', () => {
        void handle.close().then(() => process.exit(0));
      });
      return; // the server keeps the process alive
    }

    case 'build': {
      const source = defaultSource(_[1]);
      if (!existsSync(source)) fail(`Error: source not found: ${source}`, 1);
      const out = typeof flags.out === 'string' ? flags.out : 'dist';
      const result = buildStatic({
        source,
        outDir: out,
        ...(templateDir !== undefined ? { templateDir } : {}),
      });
      process.stdout.write(`Built ${result.slideCount} slide(s) → ${result.htmlPath}\n`);
      return;
    }

    case 'export': {
      const source = defaultSource(_[1]);
      if (!existsSync(source)) fail(`Error: source not found: ${source}`, 1);
      const out = typeof flags.out === 'string' ? flags.out : 'out.pdf';
      try {
        const result = await exportPdf({
          source,
          out,
          ...(templateDir !== undefined ? { templateDir } : {}),
        });
        process.stdout.write(`Exported ${result.slideCount} slide(s) → ${result.pdfPath}\n`);
      } catch (err) {
        if (err instanceof PdfUnavailableError) {
          fail(
            'PDF export unavailable:\n  ' +
              err.message +
              '\nA static HTML deck was still produced; open it in a browser and print to PDF as a fallback.',
            1,
          );
        }
        throw err;
      }
      return;
    }

    default:
      fail(`Error: unknown command "${command}".\n\n` + USAGE, 1);
  }
}

main().catch((err) => {
  process.stderr.write('Internal error: ' + (err instanceof Error ? err.stack ?? err.message : String(err)) + '\n');
  process.exit(2);
});
