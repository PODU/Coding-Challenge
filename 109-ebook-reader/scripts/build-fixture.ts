/**
 * Reproducibly builds the committed test fixtures under `test/fixtures/`.
 *
 * Every fixture is assembled here from readable source strings (no opaque
 * binaries), so the suite's exact assertions are reviewable. Run via:
 *   npm run fixtures
 */
import { mkdir, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { zipSync, Zippable } from 'fflate';

const __dirname = dirname(fileURLToPath(import.meta.url));
const OUT = join(__dirname, '..', 'test', 'fixtures');
const enc = new TextEncoder();

// 1x1 transparent PNG (valid image bytes, tiny and deterministic).
const PNG = Uint8Array.from(
  Buffer.from(
    'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAC0lEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==',
    'base64',
  ),
);

/** Build an EPUB with `mimetype` stored first and uncompressed (OCF rule). */
function buildEpub(files: Record<string, Uint8Array>): Uint8Array {
  const zippable: Zippable = {
    // mimetype MUST be first and uncompressed.
    mimetype: [enc.encode('application/epub+zip'), { level: 0 }],
  };
  for (const [path, data] of Object.entries(files)) {
    zippable[path] = data;
  }
  return zipSync(zippable, { level: 6 });
}

const CONTAINER = (opf: string) => `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="${opf}" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`;

const CSS = `body { font-family: serif; line-height: 1.6; margin: 1em; }
h1 { font-size: 1.5em; }`;

function chapter(title: string, body: string): string {
  return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
<head><title>${title}</title><link rel="stylesheet" href="../style.css"/></head>
<body>
${body}
</body>
</html>`;
}

// Fixture 1: minimal-v3.epub (EPUB 3, nav, cover, inline image, non-linear item)
function minimalV3(): Uint8Array {
  const opf = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="bookid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
    <dc:title>The Test Book</dc:title>
    <dc:creator opf:role="aut" opf:file-as="Lovelace, Ada">Ada Lovelace</dc:creator>
    <dc:language>en</dc:language>
    <dc:identifier id="bookid">urn:uuid:TESTBOOK-V3-0001</dc:identifier>
    <dc:publisher>Coding Challenges Press</dc:publisher>
    <dc:date>2024-01-01</dc:date>
    <meta property="dcterms:modified">2024-01-01T00:00:00Z</meta>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="css" href="style.css" media-type="text/css"/>
    <item id="cover-img" href="images/cover.png" media-type="image/png" properties="cover-image"/>
    <item id="inline-img" href="images/inline.png" media-type="image/png"/>
    <item id="chap1" href="text/chap1.xhtml" media-type="application/xhtml+xml"/>
    <item id="chap2" href="text/chap2.xhtml" media-type="application/xhtml+xml"/>
    <item id="chap3" href="text/chap3.xhtml" media-type="application/xhtml+xml"/>
    <item id="notes" href="text/notes.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine>
    <itemref idref="chap1"/>
    <itemref idref="chap2"/>
    <itemref idref="chap3"/>
    <itemref idref="notes" linear="no"/>
  </spine>
</package>`;

  const nav = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
<head><title>Contents</title></head>
<body>
  <nav epub:type="toc" id="toc">
    <h1>Table of Contents</h1>
    <ol>
      <li><a href="text/chap1.xhtml">Chapter One</a></li>
      <li><a href="text/chap2.xhtml">Chapter Two</a>
        <ol>
          <li><a href="text/chap2.xhtml#sec2">Section 2.1</a></li>
        </ol>
      </li>
      <li><a href="text/chap3.xhtml">Chapter Three</a></li>
    </ol>
  </nav>
</body>
</html>`;

  return buildEpub({
    'META-INF/container.xml': enc.encode(CONTAINER('OEBPS/content.opf')),
    'OEBPS/content.opf': enc.encode(opf),
    'OEBPS/nav.xhtml': enc.encode(nav),
    'OEBPS/style.css': enc.encode(CSS),
    'OEBPS/images/cover.png': PNG,
    'OEBPS/images/inline.png': PNG,
    'OEBPS/text/chap1.xhtml': enc.encode(
      chapter(
        'Chapter One',
        '<h1>Chapter One</h1><p>MARKER-CHAPTER-ONE Call me Ishmael.</p><p><img src="../images/inline.png" alt="inline"/></p>',
      ),
    ),
    'OEBPS/text/chap2.xhtml': enc.encode(
      chapter(
        'Chapter Two',
        '<h1>Chapter Two</h1><p>MARKER-CHAPTER-TWO The second chapter.</p><h2 id="sec2">Section 2.1</h2><p>Subsection text.</p>',
      ),
    ),
    'OEBPS/text/chap3.xhtml': enc.encode(
      chapter('Chapter Three', '<h1>Chapter Three</h1><p>MARKER-CHAPTER-THREE The end.</p>'),
    ),
    'OEBPS/text/notes.xhtml': enc.encode(
      chapter('Notes', '<h1>Notes</h1><p>MARKER-NOTES Non-linear popup content.</p>'),
    ),
  });
}

// Fixture 2: minimal-v2.epub (EPUB 2, NCX, <meta name="cover">)
function minimalV2(): Uint8Array {
  const opf = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="bookid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
    <dc:title>The Test Book V2</dc:title>
    <dc:creator opf:role="aut">Grace Hopper</dc:creator>
    <dc:language>en</dc:language>
    <dc:identifier id="bookid">urn:uuid:TESTBOOK-V2-0002</dc:identifier>
    <meta name="cover" content="cover-img"/>
  </metadata>
  <manifest>
    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
    <item id="css" href="style.css" media-type="text/css"/>
    <item id="cover-img" href="images/cover.png" media-type="image/png"/>
    <item id="chap1" href="text/chap1.xhtml" media-type="application/xhtml+xml"/>
    <item id="chap2" href="text/chap2.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine toc="ncx">
    <itemref idref="chap1"/>
    <itemref idref="chap2"/>
  </spine>
</package>`;

  const ncx = `<?xml version="1.0" encoding="UTF-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
  <head><meta name="dtb:uid" content="urn:uuid:TESTBOOK-V2-0002"/></head>
  <docTitle><text>The Test Book V2</text></docTitle>
  <navMap>
    <navPoint id="np1" playOrder="1">
      <navLabel><text>Chapter One</text></navLabel>
      <content src="text/chap1.xhtml"/>
      <navPoint id="np1a" playOrder="2">
        <navLabel><text>Part A</text></navLabel>
        <content src="text/chap1.xhtml#a"/>
      </navPoint>
    </navPoint>
    <navPoint id="np2" playOrder="5">
      <navLabel><text>Chapter Two</text></navLabel>
      <content src="text/chap2.xhtml"/>
    </navPoint>
  </navMap>
</ncx>`;

  return buildEpub({
    'META-INF/container.xml': enc.encode(CONTAINER('OEBPS/content.opf')),
    'OEBPS/content.opf': enc.encode(opf),
    'OEBPS/toc.ncx': enc.encode(ncx),
    'OEBPS/style.css': enc.encode(CSS),
    'OEBPS/images/cover.png': PNG,
    'OEBPS/text/chap1.xhtml': enc.encode(
      chapter('Chapter One', '<h1>Chapter One</h1><p id="a">MARKER-V2-ONE Hello.</p>'),
    ),
    'OEBPS/text/chap2.xhtml': enc.encode(
      chapter('Chapter Two', '<h1>Chapter Two</h1><p>MARKER-V2-TWO Goodbye.</p>'),
    ),
  });
}

// Fixture 3: no-toc.epub (valid spine, no nav and no NCX -> synthesis)
function noToc(): Uint8Array {
  const opf = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="bookid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>No TOC Book</dc:title>
    <dc:creator>Anonymous</dc:creator>
    <dc:identifier id="bookid">urn:uuid:NOTOC-0003</dc:identifier>
  </metadata>
  <manifest>
    <item id="css" href="style.css" media-type="text/css"/>
    <item id="chap1" href="text/chap1.xhtml" media-type="application/xhtml+xml"/>
    <item id="chap2" href="text/chap2.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine>
    <itemref idref="chap1"/>
    <itemref idref="chap2"/>
  </spine>
</package>`;

  return buildEpub({
    'META-INF/container.xml': enc.encode(CONTAINER('OEBPS/content.opf')),
    'OEBPS/content.opf': enc.encode(opf),
    'OEBPS/style.css': enc.encode(CSS),
    'OEBPS/text/chap1.xhtml': enc.encode(
      chapter('First', '<h1>First</h1><p>MARKER-NOTOC-ONE</p>'),
    ),
    'OEBPS/text/chap2.xhtml': enc.encode(
      chapter('Second', '<h1>Second</h1><p>MARKER-NOTOC-TWO</p>'),
    ),
  });
}

// Fixture 4: bad-idref.epub (spine references a missing manifest id)
function badIdref(): Uint8Array {
  const opf = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="bookid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Bad Idref Book</dc:title>
    <dc:identifier id="bookid">urn:uuid:BADIDREF-0004</dc:identifier>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="chap1" href="text/chap1.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine>
    <itemref idref="chap1"/>
    <itemref idref="does-not-exist"/>
  </spine>
</package>`;

  const nav = `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
<head><title>Contents</title></head>
<body><nav epub:type="toc"><ol><li><a href="text/chap1.xhtml">One</a></li></ol></nav></body>
</html>`;

  return buildEpub({
    'META-INF/container.xml': enc.encode(CONTAINER('OEBPS/content.opf')),
    'OEBPS/content.opf': enc.encode(opf),
    'OEBPS/nav.xhtml': enc.encode(nav),
    'OEBPS/text/chap1.xhtml': enc.encode(
      chapter('One', '<h1>One</h1><p>MARKER-BADIDREF</p>'),
    ),
  });
}

async function main(): Promise<void> {
  await mkdir(OUT, { recursive: true });

  const fixtures: Record<string, Uint8Array> = {
    'minimal-v3.epub': minimalV3(),
    'minimal-v2.epub': minimalV2(),
    'no-toc.epub': noToc(),
    'bad-idref.epub': badIdref(),
    // Not a zip, to exercise the error path.
    'broken.zip': enc.encode('this is definitely not a valid zip archive'),
  };

  for (const [name, data] of Object.entries(fixtures)) {
    await writeFile(join(OUT, name), data);
    console.log(`wrote ${name} (${data.length} bytes)`);
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
