import { unzipSync } from 'fflate';
import { EpubParseError } from './types.js';

const decoder = new TextDecoder('utf-8');

/**
 * OCF / ZIP container reader. Unzips the archive into an addressable
 * `path -> bytes` map and surfaces fatal errors as {@link EpubParseError}.
 */
export class OcfReader {
  /** Unzip `bytes` into a resource map. Rejects on a non-zip / truncated file. */
  static open(bytes: Uint8Array): Map<string, Uint8Array> {
    let entries: Record<string, Uint8Array>;
    try {
      entries = unzipSync(bytes);
    } catch (err) {
      throw new EpubParseError(
        `Not a valid ZIP/EPUB archive: ${(err as Error).message}`,
        'BAD_ZIP',
      );
    }

    const resources = new Map<string, Uint8Array>();
    for (const [name, data] of Object.entries(entries)) {
      // Normalize separators and reject zip-slip escapes.
      const norm = name.replace(/\\/g, '/');
      if (norm.split('/').some((s) => s === '..')) {
        throw new EpubParseError(
          `Zip entry escapes archive root: ${name}`,
          'ZIP_SLIP',
        );
      }
      if (norm.endsWith('/')) continue; // directory entry
      resources.set(norm, data);
    }

    if (resources.size === 0) {
      throw new EpubParseError('Archive contains no files', 'EMPTY_ARCHIVE');
    }

    // DRM / encryption detection (fatal, with a clear message).
    if (resources.has('META-INF/encryption.xml')) {
      throw new EpubParseError(
        'Encrypted EPUB (DRM) is not supported',
        'DRM_NOT_SUPPORTED',
      );
    }

    return resources;
  }

  /** Validate the OCF `mimetype` rule. Returns a warning string, or null. */
  static checkMimetype(resources: Map<string, Uint8Array>): string | null {
    const mt = resources.get('mimetype');
    if (!mt) return 'OCF: missing `mimetype` entry';
    const value = decoder.decode(mt).trim();
    if (value !== 'application/epub+zip') {
      return `OCF: unexpected mimetype "${value}"`;
    }
    return null;
  }
}
