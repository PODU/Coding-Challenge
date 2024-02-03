/**
 * Rendering for the PR panel. Remote data only ever goes into `textContent`,
 * never `innerHTML`.
 */

import type { PullRequest } from './github.js';

function clear(container: HTMLElement): void {
  container.replaceChildren();
}

function makeStatus(text: string, modifier: string): HTMLElement {
  const p = document.createElement('p');
  p.className = `panel__status panel__status--${modifier}`;
  p.textContent = text;
  return p;
}

export function renderLoading(container: HTMLElement): void {
  clear(container);
  container.appendChild(makeStatus('Loading open pull requests…', 'loading'));
}

export function renderError(container: HTMLElement, message: string): void {
  clear(container);
  container.appendChild(makeStatus(message, 'error'));
}

export function renderPulls(container: HTMLElement, prs: PullRequest[]): void {
  clear(container);

  if (prs.length === 0) {
    container.appendChild(makeStatus('No open pull requests right now.', 'empty'));
    return;
  }

  const list = document.createElement('ul');
  list.className = 'pr-list';

  for (const pr of prs) {
    const li = document.createElement('li');
    li.className = 'pr-list__item';

    const link = document.createElement('a');
    link.className = 'pr-list__link';
    link.href = pr.url;
    link.target = '_blank';
    link.rel = 'noopener noreferrer';
    link.title = pr.title;

    const title = document.createElement('span');
    title.className = 'pr-list__title';
    title.textContent = pr.title;

    const meta = document.createElement('span');
    meta.className = 'pr-list__meta';
    meta.textContent = `#${pr.number} · by ${pr.author}`;

    link.append(title, meta);
    li.appendChild(link);
    list.appendChild(li);
  }

  container.appendChild(list);
}
