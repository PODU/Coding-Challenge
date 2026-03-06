#!/usr/bin/env node
import { readFile } from 'node:fs/promises';
import { parseEpub, EpubParseError, TocNode } from './parser/index.js';
import { htmlToText } from './reader/index.js';

const USAGE = `epubreader — inspect EPUB files (Coding Challenge #109)

Usage:
  epubreader info   <file.epub>        Title, author, version, #spine, #toc, cover
  epubreader toc    <file.epub>        Print the TOC tree
  epubreader spine  <file.epub>        Print the reading order
  epubreader text   <file.epub> <n>    Dump chapter n (0-based) as plain text
`;

async function load(path: string) {
  const bytes = await readFile(path);
  return parseEpub(new Uint8Array(bytes.buffer, bytes.byteOffset, bytes.byteLength));
}

function printToc(nodes: TocNode[], depth = 0): void {
  for (const n of nodes) {
    const indent = '  '.repeat(depth);
    const href = n.href ? `  -> ${n.href}` : '';
    console.log(`${indent}- ${n.label || '(untitled)'}${href}`);
    printToc(n.children, depth + 1);
  }
}

async function main(argv: string[]): Promise<number> {
  const [cmd, file, arg] = argv;
  if (!cmd || cmd === '-h' || cmd === '--help') {
    console.log(USAGE);
    return cmd ? 0 : 1;
  }
  if (!file) {
    console.error('Error: missing <file.epub>\n');
    console.error(USAGE);
    return 1;
  }

  const book = await load(file);

  switch (cmd) {
    case 'info': {
      const m = book.metadata;
      console.log(`Title:      ${m.title}`);
      console.log(`Author:     ${m.authors.length ? m.authors.join(', ') : 'Unknown author'}`);
      console.log(`Version:    ${book.version}`);
      if (m.language) console.log(`Language:   ${m.language}`);
      if (m.identifier) console.log(`Identifier: ${m.identifier}`);
      if (m.publisher) console.log(`Publisher:  ${m.publisher}`);
      if (m.published) console.log(`Published:  ${m.published}`);
      console.log(`Spine:      ${book.spine.length} items`);
      console.log(`TOC:        ${countToc(book.toc)} entries`);
      console.log(`Cover:      ${book.coverHref ?? '(none)'}`);
      if (book.warnings.length) {
        console.log(`Warnings:   ${book.warnings.length}`);
        for (const w of book.warnings) console.log(`  - ${w}`);
      }
      return 0;
    }
    case 'toc': {
      printToc(book.toc);
      return 0;
    }
    case 'spine': {
      book.spine.forEach((s, i) => {
        const flag = s.linear ? '' : '  [non-linear]';
        console.log(`${String(i).padStart(3)} ${s.item.href}${flag}`);
      });
      return 0;
    }
    case 'text': {
      const n = Number(arg);
      if (!Number.isInteger(n)) {
        console.error('Error: chapter index must be an integer');
        return 1;
      }
      const { html } = book.getChapter(n);
      console.log(htmlToText(html));
      return 0;
    }
    default:
      console.error(`Unknown command: ${cmd}\n`);
      console.error(USAGE);
      return 1;
  }
}

function countToc(nodes: TocNode[]): number {
  return nodes.reduce((acc, n) => acc + 1 + countToc(n.children), 0);
}

main(process.argv.slice(2))
  .then((code) => process.exit(code))
  .catch((err) => {
    if (err instanceof EpubParseError) {
      console.error(`EPUB error [${err.code}]: ${err.message}`);
    } else {
      console.error(`Error: ${(err as Error).message}`);
    }
    process.exit(2);
  });
