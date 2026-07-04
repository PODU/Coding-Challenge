import { describe, it, expect } from 'vitest';
import { splitLines, lineKey, isBlank, displayText } from '../src/core/normalize';

describe('splitLines', () => {
  it('empty text → no lines', () => {
    expect(splitLines('')).toEqual({ lines: [], hadTrailingNewline: false });
  });

  it('no trailing newline', () => {
    expect(splitLines('a\nb')).toEqual({ lines: ['a', 'b'], hadTrailingNewline: false });
  });

  it('trailing newline recorded, no phantom empty line', () => {
    expect(splitLines('a\nb\n')).toEqual({ lines: ['a', 'b'], hadTrailingNewline: true });
  });

  it('keeps CR on CRLF lines (comparison decides whether it matters)', () => {
    expect(splitLines('a\r\nb\r\n').lines).toEqual(['a\r', 'b\r']);
  });

  it('preserves interior blank lines', () => {
    expect(splitLines('a\n\nb\n').lines).toEqual(['a', '', 'b']);
  });
});

describe('lineKey', () => {
  it('identity by default (except CR stripping)', () => {
    expect(lineKey('  Foo Bar ')).toBe('  Foo Bar ');
  });

  it('ignoreLineEndings strips trailing CR by default', () => {
    expect(lineKey('abc\r')).toBe('abc');
    expect(lineKey('abc\r', { ignoreLineEndings: false })).toBe('abc\r');
  });

  it('ignoreWhitespace trims and collapses runs (covers indentation-only changes)', () => {
    expect(lineKey('  if  (x)   {', { ignoreWhitespace: true })).toBe('if (x) {');
    expect(lineKey('\tif (x) {', { ignoreWhitespace: true })).toBe(
      lineKey('    if (x) {', { ignoreWhitespace: true }),
    );
  });

  it('ignoreCase lowercases', () => {
    expect(lineKey('Hello World', { ignoreCase: true })).toBe('hello world');
  });

  it('options compose', () => {
    expect(lineKey('  HELLO  World \r', { ignoreWhitespace: true, ignoreCase: true })).toBe(
      'hello world',
    );
  });
});

describe('isBlank / displayText', () => {
  it('blank detection includes whitespace-only and CR-only lines', () => {
    expect(isBlank('')).toBe(true);
    expect(isBlank('   \t')).toBe(true);
    expect(isBlank('\r')).toBe(true);
    expect(isBlank(' x ')).toBe(false);
  });

  it('displayText strips trailing CR only', () => {
    expect(displayText('abc\r')).toBe('abc');
    expect(displayText('a\rb')).toBe('a\rb');
  });
});
