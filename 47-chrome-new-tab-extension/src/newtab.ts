/** New tab page entry point: hooks the clock and PR panel up to the DOM. */

import { startClock } from './clock.js';
import { fetchOpenPulls, GitHubHttpError } from './github.js';
import { renderError, renderLoading, renderPulls } from './render.js';

const REPO_OWNER = 'CodingChallengesFYI';
const REPO_NAME = 'SharedSolutions';

function requireEl(id: string): HTMLElement {
  const el = document.getElementById(id);
  if (!el) throw new Error(`Missing required element #${id}`);
  return el;
}

async function loadPulls(panel: HTMLElement): Promise<void> {
  renderLoading(panel);
  try {
    const prs = await fetchOpenPulls(REPO_OWNER, REPO_NAME);
    renderPulls(panel, prs);
  } catch (err) {
    if (err instanceof GitHubHttpError) {
      renderError(panel, err.message);
    } else {
      renderError(
        panel,
        "Couldn't load pull requests — you may be offline. The clock still works.",
      );
    }
  }
}

function main(): void {
  const timeEl = requireEl('time');
  const dateEl = requireEl('date');
  const panel = requireEl('panel');

  startClock(
    () => new Date(),
    (time, date) => {
      timeEl.textContent = time;
      dateEl.textContent = date;
    },
  );

  void loadPulls(panel);
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', main, { once: true });
} else {
  main();
}
