// A queue that runs async tasks one at a time, in the order they were
// enqueued, so overlapping writes settle in call order rather than by
// whichever response happens to land first.

export interface SerialQueue {
  run<T>(task: () => Promise<T>): Promise<T>;
}

export function createSerialQueue(): SerialQueue {
  // The tail of the chain. Every task waits on it and then becomes it, and
  // a rejection is swallowed here so one failure cannot wedge the queue.
  let tail: Promise<unknown> = Promise.resolve();

  function run<T>(task: () => Promise<T>): Promise<T> {
    const result = tail.then(task, task);
    tail = result.catch(() => {});
    return result;
  }

  return { run };
}
