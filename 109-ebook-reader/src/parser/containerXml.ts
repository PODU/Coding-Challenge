import { EpubParseError } from './types.js';
import { createXmlParser, toArray } from './xml.js';

const decoder = new TextDecoder('utf-8');
const OPF_MEDIA_TYPE = 'application/oebps-package+xml';

/** Reads `META-INF/container.xml` to locate the OPF package document. */
export class ContainerXml {
  /**
   * Resolve the OPF package path from the container.
   * @returns `{ opfPath, warnings }`
   * @throws {EpubParseError} when the container is missing or has no rootfile.
   */
  static opfPath(resources: Map<string, Uint8Array>): {
    opfPath: string;
    warnings: string[];
  } {
    const raw = resources.get('META-INF/container.xml');
    if (!raw) {
      throw new EpubParseError(
        'Missing META-INF/container.xml (not a valid EPUB)',
        'NO_CONTAINER',
      );
    }

    const warnings: string[] = [];
    let parsed: Record<string, unknown>;
    try {
      parsed = createXmlParser().parse(decoder.decode(raw));
    } catch (err) {
      throw new EpubParseError(
        `Malformed container.xml: ${(err as Error).message}`,
        'BAD_CONTAINER',
      );
    }

    const container = parsed['container'] as Record<string, unknown> | undefined;
    const rootfilesEl = container?.['rootfiles'] as
      | Record<string, unknown>
      | undefined;
    const rootfiles = toArray(rootfilesEl?.['rootfile']) as Array<
      Record<string, string>
    >;

    const oebps = rootfiles.filter(
      (rf) => rf['@_media-type'] === OPF_MEDIA_TYPE,
    );
    const chosen = oebps[0] ?? rootfiles[0];
    const fullPath = chosen?.['@_full-path'];

    if (!fullPath) {
      throw new EpubParseError(
        'container.xml has no valid <rootfile> (not a valid EPUB)',
        'NO_ROOTFILE',
      );
    }

    if (oebps.length > 1) {
      warnings.push(
        `container.xml declares ${oebps.length} renditions; using the first`,
      );
    }

    let decoded = fullPath;
    try {
      decoded = decodeURIComponent(fullPath);
    } catch {
      /* keep raw path */
    }
    return { opfPath: decoded.replace(/\\/g, '/'), warnings };
  }
}
