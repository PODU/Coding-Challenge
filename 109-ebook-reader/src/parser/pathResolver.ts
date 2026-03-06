import { EpubParseError } from './types.js';

/**
 * Normalizes the relative hrefs found across the OPF, NCX, nav and chapter
 * documents into clean, archive-relative resource paths.
 *
 * Rules: backslashes -> `/`, URL-decode, strip `#fragment`, resolve against a
 * base directory, collapse `.`/`..`, and reject zip-slip escapes above the
 * archive root.
 */
export class PathResolver {
  /** Directory portion of a path (`OEBPS/text/c.xhtml` -> `OEBPS/text`). */
  static dirname(path: string): string {
    const norm = path.replace(/\\/g, '/');
    const i = norm.lastIndexOf('/');
    return i < 0 ? '' : norm.slice(0, i);
  }

  /** Split a raw href into its path part and optional fragment (no `#`). */
  static splitFragment(href: string): { path: string; fragment?: string } {
    const hash = href.indexOf('#');
    if (hash < 0) return { path: href };
    return { path: href.slice(0, hash), fragment: href.slice(hash + 1) };
  }

  /**
   * Resolve `href` (path only; any fragment is stripped) against `baseDir`.
   * Throws {@link EpubParseError} on a zip-slip escape above the root.
   */
  static resolve(baseDir: string, href: string): string {
    const { path } = PathResolver.splitFragment(href);
    let h = path.replace(/\\/g, '/');
    try {
      h = decodeURIComponent(h);
    } catch {
      // Leave as-is when the href is not valid percent-encoding.
    }
    if (h === '') return '';

    let combined: string;
    if (h.startsWith('/')) {
      combined = h.slice(1); // archive-absolute
    } else {
      const base = baseDir.replace(/\\/g, '/').replace(/\/+$/, '');
      combined = base ? `${base}/${h}` : h;
    }

    const out: string[] = [];
    for (const seg of combined.split('/')) {
      if (seg === '' || seg === '.') continue;
      if (seg === '..') {
        if (out.length === 0) {
          throw new EpubParseError(
            `Path escapes archive root: ${href}`,
            'ZIP_SLIP',
          );
        }
        out.pop();
        continue;
      }
      out.push(seg);
    }
    return out.join('/');
  }

  /** Like {@link resolve} but preserves a trailing `#fragment` in the result. */
  static resolveWithFragment(baseDir: string, href: string): string {
    const { path, fragment } = PathResolver.splitFragment(href);
    const resolved = PathResolver.resolve(baseDir, path);
    return fragment ? `${resolved}#${fragment}` : resolved;
  }
}
