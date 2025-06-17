import { describe, it, expect } from 'vitest';
import { makePRNG, WordProvider, COMMON_WORDS } from '../src/core/words.js';

describe('words — deterministic generation', () => {
  it('makePRNG yields a fixed sequence for a given seed', () => {
    const a = makePRNG(42);
    const b = makePRNG(42);
    const seqA = [a(), a(), a(), a()];
    const seqB = [b(), b(), b(), b()];
    expect(seqA).toEqual(seqB);
    for (const n of seqA) {
      expect(n).toBeGreaterThanOrEqual(0);
      expect(n).toBeLessThan(1);
    }
  });

  it('same seed => identical word chunk across runs', () => {
    const p1 = new WordProvider(COMMON_WORDS, makePRNG(7));
    const p2 = new WordProvider(COMMON_WORDS, makePRNG(7));
    expect(p1.nextWords(25)).toBe(p2.nextWords(25));
  });

  it('chunk has the requested word count, single spaces, no edge spaces', () => {
    const p = new WordProvider(COMMON_WORDS, makePRNG(1));
    const chunk = p.nextWords(25);
    const words = chunk.split(' ');
    expect(words.length).toBe(25);
    expect(chunk.startsWith(' ')).toBe(false);
    expect(chunk.endsWith(' ')).toBe(false);
    expect(chunk.includes('  ')).toBe(false);
  });

  it('all generated words come from the bundled list', () => {
    const set = new Set(COMMON_WORDS);
    const p = new WordProvider(COMMON_WORDS, makePRNG(123));
    for (const w of p.nextWords(100).split(' ')) {
      expect(set.has(w)).toBe(true);
    }
  });

  it('the word list is non-empty and lower-case alphabetic', () => {
    expect(COMMON_WORDS.length).toBeGreaterThan(50);
    for (const w of COMMON_WORDS) {
      expect(w).toMatch(/^[a-z]+$/);
    }
  });
});
