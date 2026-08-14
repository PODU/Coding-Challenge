'use strict';
// Pull Tracked, the sorts and buildArray out of index.html (they don't touch
// the DOM), run them in a VM sandbox and drive the generators to completion
// the same way the page does.
const fs = require('fs');
const path = require('path');
const vm = require('vm');

const html = fs.readFileSync(path.join(__dirname, '..', 'index.html'), 'utf8');
const script = html.slice(html.indexOf('<script>') + '<script>'.length, html.indexOf('</script>'));
const core = script.slice(0, script.indexOf('const css ='));
const buildArraySrc = script.match(/function buildArray\(n, order\) \{[\s\S]*?\n\}\n/)[0];

const sandbox = {};
vm.createContext(sandbox);
vm.runInContext(core + '\n' + buildArraySrc + '\nthis.Tracked = Tracked; this.ALGORITHMS = ALGORITHMS; this.buildArray = buildArray;', sandbox);
const { Tracked, ALGORITHMS, buildArray } = sandbox;

let failures = 0;
function check(name, cond, detail) {
  if (cond) {
    console.log('PASS  ' + name);
  } else {
    failures++;
    console.log('FAIL  ' + name + (detail ? '  (' + detail + ')' : ''));
  }
}

// Run an algorithm to completion; return the Tracked array and yield count.
function run(algo, values) {
  const t = new Tracked(values.slice());
  const gen = ALGORITHMS[algo](t);
  let steps = 0;
  while (!gen.next().done) {
    steps++;
    t.clearMarks();
    if (steps > 1e7) throw new Error(algo + ' did not terminate');
  }
  return { t, steps };
}

const isSorted = a => a.every((v, i) => i === 0 || a[i - 1] <= v);
const sameMultiset = (a, b) => JSON.stringify([...a].sort((x, y) => x - y)) === JSON.stringify([...b].sort((x, y) => x - y));

// Deterministic PRNG so failures reproduce.
let seed = 133;
const rand = () => ((seed = (seed * 1103515245 + 12345) % 2147483648) / 2147483648);

const inputs = {
  empty: [],
  single: [7],
  two: [2, 1],
  sorted: Array.from({ length: 50 }, (_, i) => i + 1),
  reversed: Array.from({ length: 50 }, (_, i) => 50 - i),
  duplicates: [5, 3, 5, 1, 3, 3, 9, 0, 9, 5],
  allEqual: Array(20).fill(4),
  random200: Array.from({ length: 200 }, () => Math.floor(rand() * 1000)),
};

check('8 algorithms registered', Object.keys(ALGORITHMS).length === 8, Object.keys(ALGORITHMS).join(', '));

for (const algo of Object.keys(ALGORITHMS)) {
  for (const [label, values] of Object.entries(inputs)) {
    const { t } = run(algo, values);
    check(`${algo}: ${label}`, isSorted(t.a) && sameMultiset(t.a, values), t.a.slice(0, 12).join(','));
  }
  const { t, steps } = run(algo, inputs.random200);
  check(`${algo}: marks all indices sorted on completion`, t.sorted.size === 200, 'sorted=' + t.sorted.size);
  check(`${algo}: records stats and yields`, steps > 0 && t.stats.accesses > 0 && t.stats.writes > 0,
    JSON.stringify(t.stats));
}

// Comparison sorts count comparisons; radix sort is non-comparative.
for (const algo of Object.keys(ALGORITHMS).filter(a => !a.startsWith('Radix'))) {
  const { t } = run(algo, inputs.random200);
  check(`${algo}: counts comparisons`, t.stats.comparisons > 0);
}

// Bubble sort on sorted input should exit early (one pass, no writes).
{
  const { t } = run('Bubble sort', inputs.sorted);
  check('Bubble sort: early exit on sorted input', t.stats.writes === 0 && t.stats.comparisons === 49,
    JSON.stringify(t.stats));
}

// Tracked bookkeeping.
{
  const t = new Tracked([3, 1, 2]);
  t.swap(0, 1);
  t.compare(0, 2);
  t.set(2, 9);
  check('Tracked: swap/compare/set update stats',
    t.stats.writes === 3 && t.stats.comparisons === 1 && t.stats.accesses === 5, JSON.stringify(t.stats));
  check('Tracked: marks recorded', t.marks.write.join() === '0,1,2' && t.marks.compare.join() === '0,2');
  t.clearMarks();
  check('Tracked: clearMarks resets highlights', t.marks.write.length === 0 && t.marks.compare.length === 0);
}

// buildArray orders: every order is a permutation of 1..n.
for (const order of ['random', 'reversed', 'sorted', 'nearly']) {
  const a = buildArray(100, order);
  const perm = sameMultiset(a, Array.from({ length: 100 }, (_, i) => i + 1));
  check(`buildArray(${order}) is a permutation of 1..n`, perm && a.length === 100);
}
check('buildArray(sorted) is ascending', isSorted(buildArray(30, 'sorted')));
check('buildArray(reversed) is descending', isSorted(buildArray(30, 'reversed').reverse()));

console.log(failures ? `\n${failures} FAILED` : '\nALL PASS');
process.exit(failures ? 1 : 0);
