import './styles.css';
import { App } from './ui/app.js';
import { RealClock, DEFAULT_CONFIG, DURATION_OPTIONS, type TestConfig } from './core/index.js';

const root = document.getElementById('app');
if (!root) throw new Error('#app root not found');

const clock = new RealClock();
let config: TestConfig = { ...DEFAULT_CONFIG };
const app = new App(root, config, clock);
app.start();

const selector = document.getElementById('durations');
if (selector) {
  for (const sec of DURATION_OPTIONS) {
    const btn = document.createElement('button');
    btn.className = 'duration-btn';
    btn.textContent = `${sec}s`;
    btn.dataset.sec = String(sec);
    if (sec === config.durationSec) btn.classList.add('active');
    btn.addEventListener('click', () => {
      config = { ...config, durationSec: sec };
      app.setConfig(config);
      app.restart();
      for (const b of selector.querySelectorAll('.duration-btn')) {
        b.classList.toggle('active', b === btn);
      }
      (document.activeElement as HTMLElement | null)?.blur();
    });
    selector.append(btn);
  }
}

const restartBtn = document.getElementById('restart');
restartBtn?.addEventListener('click', () => {
  app.restart();
  (document.activeElement as HTMLElement | null)?.blur();
});
