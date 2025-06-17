export interface TestConfig {
  durationSec: number;
  /** Words added to the prompt at a time. */
  wordCount: number;
  /** Seed for word selection. If omitted the app picks a random one. */
  seed?: number;
  /** Backspace can't go back into a previous word. */
  strictBackspace: boolean;
}

export const DEFAULT_CONFIG: TestConfig = {
  durationSec: 30,
  wordCount: 25,
  strictBackspace: false,
};

/** Durations offered in the UI, in seconds. */
export const DURATION_OPTIONS = [15, 30, 60, 120] as const;
