// @vitest-environment happy-dom
import { beforeEach, describe, expect, it } from 'vitest';
import { fetchOpenPulls, type FetchLike, type FetchResponse } from '../src/github.js';
import { renderError, renderLoading, renderPulls } from '../src/render.js';

const FIXTURE = [
  {
    number: 12,
    title: 'Add Go solution for challenge 7',
    html_url: 'https://github.com/CodingChallengesFYI/SharedSolutions/pull/12',
    user: { login: 'alice' },
  },
  {
    number: 13,
    title: 'Fix typo',
    html_url: 'https://github.com/CodingChallengesFYI/SharedSolutions/pull/13',
    user: { login: 'bob' },
  },
];

function okResponse(body: unknown): FetchResponse {
  return { ok: true, status: 200, json: () => Promise.resolve(body) };
}

let panel: HTMLElement;
beforeEach(() => {
  document.body.innerHTML = '<div id="panel"></div>';
  panel = document.getElementById('panel') as HTMLElement;
});

describe('PR-list rendering from a mocked fetch', () => {
  it('renders one link per PR with title and author, no real network', async () => {
    const fakeFetch: FetchLike = () => Promise.resolve(okResponse(FIXTURE));

    renderLoading(panel);
    expect(panel.textContent).toContain('Loading');

    const prs = await fetchOpenPulls('CodingChallengesFYI', 'SharedSolutions', fakeFetch);
    renderPulls(panel, prs);

    const links = panel.querySelectorAll('a.pr-list__link');
    expect(links).toHaveLength(2);

    const first = links[0] as HTMLAnchorElement;
    expect(first.getAttribute('href')).toBe(
      'https://github.com/CodingChallengesFYI/SharedSolutions/pull/12',
    );
    expect(first.textContent).toContain('Add Go solution for challenge 7');
    expect(first.textContent).toContain('alice');
    expect(first.textContent).toContain('#12');
    // External links open safely.
    expect(first.getAttribute('rel')).toContain('noopener');
  });

  it('shows an explicit empty-state message for zero PRs', () => {
    renderPulls(panel, []);
    expect(panel.querySelectorAll('a').length).toBe(0);
    expect(panel.textContent).toMatch(/no open pull requests/i);
  });

  it('does not inject markup from PR titles (XSS-safe)', () => {
    renderPulls(panel, [
      {
        number: 1,
        title: '<img src=x onerror=alert(1)>',
        author: 'mallory',
        url: 'https://example.com/1',
      },
    ]);
    // The angle brackets survive as text, not as an <img> element.
    expect(panel.querySelector('img')).toBeNull();
    expect(panel.textContent).toContain('<img src=x onerror=alert(1)>');
  });
});

describe('offline / failure fallback rendering', () => {
  it('renders a friendly error when the (mocked) fetch rejects', async () => {
    const offlineFetch: FetchLike = () => Promise.reject(new Error('Failed to fetch'));

    renderLoading(panel);
    try {
      const prs = await fetchOpenPulls('o', 'r', offlineFetch);
      renderPulls(panel, prs);
    } catch {
      renderError(panel, "Couldn't load pull requests — you may be offline.");
    }

    expect(panel.querySelector('.panel__status--error')).not.toBeNull();
    expect(panel.textContent).toMatch(/offline/i);
  });
});
