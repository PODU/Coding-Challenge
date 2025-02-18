/** `mdpresent init`: creates a new presentation project. */
import { mkdirSync, writeFileSync, existsSync } from 'node:fs';
import { join } from 'node:path';
import { execFileSync } from 'node:child_process';

export interface ScaffoldOptions {
  dir: string;
  /** Run `git init`. Defaults to true. */
  git?: boolean;
}

export interface ScaffoldResult {
  dir: string;
  created: string[];
  gitInitialized: boolean;
}

const STARTER_SLIDES = `---
title: My Presentation
theme: default
---

# My Presentation

A deck built with **mdpresent**.

Use the arrow keys (or the on-screen buttons) to navigate.

---

## Markdown, your way

- Write slides in plain Markdown
- Separate them with a line containing \`---\`
- Live reload updates the browser as you type

---

## Code blocks

\`\`\`ts
function greet(name: string): string {
  return \`Hello, \${name}!\`;
}
\`\`\`

Fenced code is safe — a \`---\` inside a fence does **not** split the slide.

---

## Images

Drop assets in \`images/\` and reference them relatively:

![Diagram](images/example.png)

---

# Thank you

Export to PDF with \`mdpresent export\`.
`;

const README = `# My Presentation

Built with [mdpresent](https://github.com/) — a Markdown presentation tool.

## Usage

\`\`\`sh
mdpresent serve     # dev server with live reload (http://localhost:3000)
mdpresent build     # render a static deck to ./dist
mdpresent export    # render to ./out.pdf (requires puppeteer)
\`\`\`

## Layout

- \`slides/\`     — your Markdown slides
- \`templates/\`  — optional theme/layout overrides (falls back to built-in defaults)
- \`images/\`     — assets referenced by slides
- \`mdpresent.config.json\` — project config
`;

const CONFIG = {
  theme: 'default',
  port: 3000,
  source: 'slides',
  out: 'dist',
};

const GITIGNORE = `node_modules/
dist/
out.pdf
.mdpresent-build/
`;

function writeIfAbsent(path: string, content: string, created: string[]): void {
  if (!existsSync(path)) {
    writeFileSync(path, content, 'utf8');
    created.push(path);
  }
}

export function scaffold(options: ScaffoldOptions): ScaffoldResult {
  const { dir } = options;
  const created: string[] = [];

  mkdirSync(dir, { recursive: true });
  for (const sub of ['slides', 'templates', 'images']) {
    const p = join(dir, sub);
    if (!existsSync(p)) {
      mkdirSync(p, { recursive: true });
      created.push(p);
    }
  }

  writeIfAbsent(join(dir, 'slides', 'slides.md'), STARTER_SLIDES, created);
  writeIfAbsent(join(dir, 'README.md'), README, created);
  writeIfAbsent(join(dir, 'mdpresent.config.json'), JSON.stringify(CONFIG, null, 2) + '\n', created);
  writeIfAbsent(join(dir, '.gitignore'), GITIGNORE, created);
  // Keep otherwise-empty dirs in git.
  writeIfAbsent(join(dir, 'templates', '.gitkeep'), '', created);
  writeIfAbsent(join(dir, 'images', '.gitkeep'), '', created);

  let gitInitialized = false;
  if (options.git !== false && !existsSync(join(dir, '.git'))) {
    try {
      execFileSync('git', ['init'], { cwd: dir, stdio: 'ignore' });
      gitInitialized = true;
    } catch {
      // git isn't installed; carry on without it.
    }
  }

  return { dir, created, gitInitialized };
}
