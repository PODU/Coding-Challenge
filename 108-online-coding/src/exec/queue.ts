/** Raised when the queue is saturated; the API maps this to HTTP 429. */
export class QueueFullError extends Error {
  constructor(message = 'Server busy: execution queue is full') {
    super(message);
    this.name = 'QueueFullError';
  }
}

interface Job<T> {
  task: () => Promise<T>;
  resolve: (value: T) => void;
  reject: (err: unknown) => void;
}

/**
 * Bounded worker pool: caps simultaneous untrusted processes and the queue
 * depth so a flood of submissions can't exhaust the host. Excess load is shed
 * with QueueFullError (-> 429). Stateless across requests beyond live counts.
 */
export class WorkerPool {
  private active = 0;
  private readonly waiting: Array<Job<unknown>> = [];

  constructor(
    private readonly maxConcurrency: number,
    private readonly maxQueueLength: number,
  ) {}

  get activeCount(): number {
    return this.active;
  }

  get queuedCount(): number {
    return this.waiting.length;
  }

  submit<T>(task: () => Promise<T>): Promise<T> {
    if (this.active >= this.maxConcurrency && this.waiting.length >= this.maxQueueLength) {
      return Promise.reject(new QueueFullError());
    }
    return new Promise<T>((resolve, reject) => {
      this.waiting.push({ task, resolve, reject } as Job<unknown>);
      this.pump();
    });
  }

  private pump(): void {
    while (this.active < this.maxConcurrency && this.waiting.length > 0) {
      const job = this.waiting.shift()!;
      this.active += 1;
      void job
        .task()
        .then((v) => job.resolve(v))
        .catch((e) => job.reject(e))
        .finally(() => {
          this.active -= 1;
          this.pump();
        });
    }
  }
}
