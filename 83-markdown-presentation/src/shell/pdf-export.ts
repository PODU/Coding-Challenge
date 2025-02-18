/**
 * PDF export through headless Chromium.
 *
 * Puppeteer is an optional dependency and Chromium may not be able to run, so the HTML
 * build always happens first and puppeteer is imported lazily. If it can't run we throw
 * PdfUnavailableError and the caller can point the user at the HTML instead.
 */
import { pathToFileURL } from 'node:url';
import { existsSync } from 'node:fs';
import { buildStatic } from './build.js';

export interface ExportPdfOptions {
  source: string;
  out: string;
  templateDir?: string;
  assetsRoot?: string;
  /** Where to put the intermediate HTML build. Defaults to `.mdpresent-build`. */
  workDir?: string;
}

export interface ExportResult {
  pdfPath: string;
  slideCount: number;
}

export class PdfUnavailableError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'PdfUnavailableError';
  }
}

/** Whether puppeteer can be imported. Chromium may still fail to launch. */
export async function isPdfAvailable(): Promise<boolean> {
  try {
    await import('puppeteer');
    return true;
  } catch {
    return false;
  }
}

/**
 * Build the deck and print it to PDF. Throws {@link PdfUnavailableError} if puppeteer or
 * Chromium can't run.
 */
export async function exportPdf(options: ExportPdfOptions): Promise<ExportResult> {
  const workDir = options.workDir ?? '.mdpresent-build';
  const build = buildStatic({
    source: options.source,
    outDir: workDir,
    ...(options.templateDir !== undefined ? { templateDir: options.templateDir } : {}),
    ...(options.assetsRoot !== undefined ? { assetsRoot: options.assetsRoot } : {}),
  });

  let puppeteer: typeof import('puppeteer');
  try {
    puppeteer = (await import('puppeteer')).default as unknown as typeof import('puppeteer');
  } catch (err) {
    throw new PdfUnavailableError(
      'puppeteer is not installed. Install it (`npm install puppeteer`) to enable PDF export. ' +
        'The HTML deck was still written to ' +
        build.htmlPath +
        '. Original error: ' +
        (err instanceof Error ? err.message : String(err)),
    );
  }

  if (!existsSync(build.htmlPath)) {
    throw new PdfUnavailableError('Static build did not produce ' + build.htmlPath);
  }

  let browser: Awaited<ReturnType<typeof puppeteer.launch>>;
  try {
    browser = await puppeteer.launch({
      headless: true,
      args: ['--no-sandbox', '--disable-setuid-sandbox'],
    });
  } catch (err) {
    throw new PdfUnavailableError(
      'Failed to launch headless Chromium for PDF export. The HTML deck is at ' +
        build.htmlPath +
        '. Original error: ' +
        (err instanceof Error ? err.message : String(err)),
    );
  }

  try {
    const page = await browser.newPage();
    await page.goto(pathToFileURL(build.htmlPath).href, { waitUntil: 'networkidle0' });
    await page.pdf({
      path: options.out,
      printBackground: true,
      preferCSSPageSize: true,
      format: 'A4',
      landscape: true,
    });
  } finally {
    await browser.close();
  }

  return { pdfPath: options.out, slideCount: build.slideCount };
}
