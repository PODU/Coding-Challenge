/** Thrown when an operation requires at least one segment but the wheel is empty. */
export class EmptyWheelError extends Error {
  constructor(message = "The wheel has no names.") {
    super(message);
    this.name = "EmptyWheelError";
  }
}

/** Thrown when a persisted wheel string cannot be decoded. */
export class WheelDecodeError extends Error {
  constructor(message = "Could not decode wheel state.") {
    super(message);
    this.name = "WheelDecodeError";
  }
}

/** Thrown when an index argument falls outside the valid range. */
export class IndexRangeError extends RangeError {
  constructor(message = "Index out of range.") {
    super(message);
    this.name = "IndexRangeError";
  }
}
