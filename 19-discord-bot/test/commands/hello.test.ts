import { describe, it, expect } from "vitest";
import { hello } from "../../src/commands/hello";
import { makeContext } from "../helpers";

describe("hello command", () => {
  it("greets the user by name", async () => {
    const ctx = makeContext([], { user: { username: "Ada" } });
    await hello.run(ctx);
    expect(ctx.replies).toEqual(["Hello, Ada!"]);
  });
});
