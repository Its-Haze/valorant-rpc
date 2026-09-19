import { describe, expect, it } from "vitest";
import { createSerialQueue } from "./serialQueue";

describe("createSerialQueue", () => {
  it("runs tasks in call order even when an earlier one settles later", async () => {
    const queue = createSerialQueue();
    const order: string[] = [];

    let release!: () => void;
    const blocked = new Promise<void>((resolve) => (release = resolve));

    const first = queue.run(async () => {
      await blocked;
      order.push("first");
    });
    const second = queue.run(async () => {
      order.push("second");
    });

    release();
    await Promise.all([first, second]);

    expect(order).toEqual(["first", "second"]);
  });

  it("reads state inside the task, so each one sees the previous one's writes", async () => {
    const queue = createSerialQueue();
    const seen: number[] = [];
    let value = 0;

    const first = queue.run(async () => {
      await Promise.resolve();
      value += 1;
    });
    const second = queue.run(async () => {
      seen.push(value);
    });

    await Promise.all([first, second]);
    expect(seen).toEqual([1]);
  });

  it("keeps running after a task rejects", async () => {
    const queue = createSerialQueue();

    await expect(queue.run(async () => {
      throw new Error("boom");
    })).rejects.toThrow("boom");

    await expect(queue.run(async () => "still here")).resolves.toBe("still here");
  });
});
