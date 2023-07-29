import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { promises as fs } from "fs";
import * as os from "os";
import * as path from "path";
import { createCatalog, normalizeUrl } from "../../src/services/catalog";

let tmpDir: string;
let catalogPath: string;
let seedPath: string;

beforeEach(async () => {
  tmpDir = await fs.mkdtemp(path.join(os.tmpdir(), "catalog-test-"));
  catalogPath = path.join(tmpDir, "challenges.json");
  seedPath = path.join(tmpDir, "seed.json");
  await fs.writeFile(
    seedPath,
    JSON.stringify([
      {
        title: "Seeded",
        url: "https://codingchallenges.fyi/seeded",
        addedBy: "seed",
        addedAt: "1970-01-01T00:00:00.000Z",
      },
    ]),
    "utf8",
  );
});

afterEach(async () => {
  await fs.rm(tmpDir, { recursive: true, force: true });
});

describe("normalizeUrl", () => {
  it("strips trailing slash and hash, lowercases host", () => {
    expect(normalizeUrl("https://CodingChallenges.fyi/x/#frag")).toBe(
      "https://codingchallenges.fyi/x",
    );
  });
});

describe("catalog service", () => {
  it("seeds on first run and persists the file", async () => {
    const cat = createCatalog({ catalogPath, seedPath });
    const all = await cat.list();
    expect(all).toHaveLength(1);
    expect(all[0]!.title).toBe("Seeded");
    // file written
    const onDisk = JSON.parse(await fs.readFile(catalogPath, "utf8"));
    expect(onDisk).toHaveLength(1);
  });

  it("adds and round-trips through a fresh instance", async () => {
    const cat = createCatalog({
      catalogPath,
      seedPath,
      now: () => new Date("2020-01-01T00:00:00.000Z"),
    });
    const res = await cat.add({
      title: "New One",
      url: "https://codingchallenges.fyi/new",
      addedBy: "ada",
    });
    expect(res.status).toBe("added");

    // Re-open: a new instance should read the persisted entry.
    const cat2 = createCatalog({ catalogPath, seedPath });
    const all = await cat2.list();
    expect(all.map((c) => c.title)).toContain("New One");
    expect(all).toHaveLength(2);
  });

  it("dedupes by normalized url", async () => {
    const cat = createCatalog({ catalogPath, seedPath });
    await cat.add({
      title: "Dup",
      url: "https://codingchallenges.fyi/dup",
      addedBy: "a",
    });
    const res = await cat.add({
      title: "Dup Again",
      url: "https://codingchallenges.fyi/dup/",
      addedBy: "b",
    });
    expect(res.status).toBe("duplicate");
    const all = await cat.list();
    expect(all.filter((c) => c.url.includes("dup"))).toHaveLength(1);
  });

  it("recovers from a corrupt file by re-seeding", async () => {
    await fs.writeFile(catalogPath, "{ this is not json", "utf8");
    const cat = createCatalog({ catalogPath, seedPath });
    const all = await cat.list();
    expect(all).toHaveLength(1);
    expect(all[0]!.title).toBe("Seeded");
  });

  it("starts empty with no seed and no file", async () => {
    const cat = createCatalog({ catalogPath });
    expect(await cat.list()).toHaveLength(0);
    expect(await cat.random()).toBeUndefined();
  });

  it("serializes concurrent adds without losing writes", async () => {
    const cat = createCatalog({ catalogPath });
    await Promise.all(
      Array.from({ length: 10 }, (_, i) =>
        cat.add({
          title: `C${i}`,
          url: `https://codingchallenges.fyi/c${i}`,
          addedBy: "x",
        }),
      ),
    );
    const cat2 = createCatalog({ catalogPath });
    expect(await cat2.list()).toHaveLength(10);
  });
});
