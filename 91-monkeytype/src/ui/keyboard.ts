/**
 * Turns keydown events into typed characters, backspaces and restarts. Shortcuts,
 * IME composition and non-printing keys are ignored.
 */
export interface KeyboardHandlers {
  onChar(ch: string): void;
  onBackspace(): void;
  onRestart(): void;
}

export class KeyboardController {
  private readonly target: EventTarget;
  private readonly handlers: KeyboardHandlers;
  private readonly listener: (e: Event) => void;

  constructor(target: EventTarget, handlers: KeyboardHandlers) {
    this.target = target;
    this.handlers = handlers;
    this.listener = (e: Event) => this.onKeyDown(e as KeyboardEvent);
  }

  attach(): void {
    this.target.addEventListener('keydown', this.listener);
  }

  detach(): void {
    this.target.removeEventListener('keydown', this.listener);
  }

  private onKeyDown(e: KeyboardEvent): void {
    // Tab or Escape restarts.
    if (e.key === 'Tab' || e.key === 'Escape') {
      e.preventDefault();
      this.handlers.onRestart();
      return;
    }

    // Keys pressed while an IME is composing would be counted twice.
    if (e.isComposing) return;
    // Ctrl/Alt/Meta combinations are shortcuts, not typing.
    if (e.ctrlKey || e.altKey || e.metaKey) return;

    if (e.key === 'Backspace') {
      e.preventDefault();
      this.handlers.onBackspace();
      return;
    }

    // Printable keys (including Space) have a one-character key value; arrows,
    // Enter, Shift and the like have longer names.
    if (e.key.length === 1) {
      e.preventDefault();
      this.handlers.onChar(e.key);
    }
  }
}
