import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const here = dirname(fileURLToPath(import.meta.url));

/** Load a committed fixture as bytes. */
export function fixture(name: string): Uint8Array {
  const buf = readFileSync(join(here, 'fixtures', name));
  return new Uint8Array(buf.buffer, buf.byteOffset, buf.byteLength);
}

export const FIXTURE_DIR = join(here, 'fixtures');
