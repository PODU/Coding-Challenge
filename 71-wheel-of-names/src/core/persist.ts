import { WheelDecodeError } from "./errors.js";

export interface WheelState {
  names: string[];
  weights?: number[];
  colors?: string[];
}

const VERSION = 1;

/**
 * Encode a wheel as versioned JSON. Missing optional fields are left out so
 * that decodeWheel(encodeWheel(s)) deep-equals s.
 */
export function encodeWheel(s: WheelState): string {
  const payload: Record<string, unknown> = { v: VERSION, names: s.names };
  if (s.weights !== undefined) payload.weights = s.weights;
  if (s.colors !== undefined) payload.colors = s.colors;
  return JSON.stringify(payload);
}

/** Decode encodeWheel output. Throws WheelDecodeError if it is malformed. */
export function decodeWheel(str: string): WheelState {
  let raw: unknown;
  try {
    raw = JSON.parse(str);
  } catch {
    throw new WheelDecodeError("Invalid JSON.");
  }
  if (typeof raw !== "object" || raw === null) {
    throw new WheelDecodeError("Expected an object.");
  }
  const obj = raw as Record<string, unknown>;
  if (!isStringArray(obj.names)) {
    throw new WheelDecodeError("Missing or invalid 'names'.");
  }

  const state: WheelState = { names: obj.names };

  if (obj.weights !== undefined) {
    if (!isNumberArray(obj.weights)) throw new WheelDecodeError("Invalid 'weights'.");
    state.weights = obj.weights;
  }
  if (obj.colors !== undefined) {
    if (!isStringArray(obj.colors)) throw new WheelDecodeError("Invalid 'colors'.");
    state.colors = obj.colors;
  }
  return state;
}

function isStringArray(v: unknown): v is string[] {
  return Array.isArray(v) && v.every((x) => typeof x === "string");
}

function isNumberArray(v: unknown): v is number[] {
  return Array.isArray(v) && v.every((x) => typeof x === "number");
}
