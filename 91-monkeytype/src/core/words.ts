/** mulberry32, a small seedable PRNG returning numbers in [0, 1). */
export function makePRNG(seed: number): () => number {
  let a = seed >>> 0;
  return function next(): number {
    a |= 0;
    a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/** About 200 of the most common English words. */
export const COMMON_WORDS: readonly string[] = [
  'the', 'be', 'to', 'of', 'and', 'a', 'in', 'that', 'have', 'it',
  'for', 'not', 'on', 'with', 'he', 'as', 'you', 'do', 'at', 'this',
  'but', 'his', 'by', 'from', 'they', 'we', 'say', 'her', 'she', 'or',
  'an', 'will', 'my', 'one', 'all', 'would', 'there', 'their', 'what', 'so',
  'up', 'out', 'if', 'about', 'who', 'get', 'which', 'go', 'me', 'when',
  'make', 'can', 'like', 'time', 'no', 'just', 'him', 'know', 'take', 'people',
  'into', 'year', 'your', 'good', 'some', 'could', 'them', 'see', 'other', 'than',
  'then', 'now', 'look', 'only', 'come', 'its', 'over', 'think', 'also', 'back',
  'after', 'use', 'two', 'how', 'our', 'work', 'first', 'well', 'way', 'even',
  'new', 'want', 'because', 'any', 'these', 'give', 'day', 'most', 'us', 'man',
  'find', 'here', 'thing', 'great', 'world', 'life', 'still', 'hand', 'part', 'child',
  'eye', 'woman', 'place', 'week', 'case', 'point', 'company', 'number', 'group', 'problem',
  'fact', 'be', 'high', 'every', 'such', 'tell', 'why', 'ask', 'need', 'feel',
  'three', 'state', 'never', 'become', 'between', 'home', 'water', 'room', 'mother', 'area',
  'money', 'story', 'young', 'month', 'right', 'study', 'book', 'word', 'side', 'kind',
  'head', 'house', 'service', 'friend', 'father', 'power', 'hour', 'game', 'line', 'end',
  'member', 'law', 'car', 'city', 'name', 'team', 'minute', 'idea', 'body', 'face',
  'level', 'office', 'door', 'health', 'person', 'art', 'war', 'history', 'party', 'result',
  'change', 'morning', 'reason', 'girl', 'guy', 'moment', 'air', 'teacher', 'force', 'education',
  'foot', 'boy', 'age', 'policy', 'music', 'market', 'sense', 'nation', 'plan', 'college',
];

/** Picks random words from a list. */
export class WordProvider {
  private readonly words: readonly string[];
  private readonly rng: () => number;

  constructor(words: readonly string[], rng: () => number) {
    if (words.length === 0) {
      throw new Error('WordProvider requires a non-empty word list');
    }
    this.words = words;
    this.rng = rng;
  }

  /** `count` random words joined by spaces. */
  nextWords(count: number): string {
    const n = Math.max(0, Math.floor(count));
    const out: string[] = [];
    for (let i = 0; i < n; i += 1) {
      const idx = Math.floor(this.rng() * this.words.length) % this.words.length;
      out.push(this.words[idx]);
    }
    return out.join(' ');
  }
}
