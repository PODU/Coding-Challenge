import { describe, it, expect } from "vitest";
import {
  fromPlainText,
  fromGithubUrl,
  fromGoogleDocUrl,
  resolveImportUrl,
  fromMarkdown,
} from "../src/core/loaders.js";

describe("fromPlainText", () => {
  it("normalizes CRLF and lone CR to LF, preserving line breaks", () => {
    expect(fromPlainText("a\r\nb\rc\nd")).toBe("a\nb\nc\nd");
  });
  it("strips a leading BOM", () => {
    expect(fromPlainText("﻿hello")).toBe("hello");
  });
});

describe("fromGithubUrl", () => {
  it("rewrites a blob URL to raw.githubusercontent.com", () => {
    expect(
      fromGithubUrl("https://github.com/owner/repo/blob/main/docs/script.md"),
    ).toBe("https://raw.githubusercontent.com/owner/repo/main/docs/script.md");
  });
  it("handles nested paths and refs", () => {
    expect(
      fromGithubUrl("https://github.com/o/r/blob/v1.2/a/b/c.md"),
    ).toBe("https://raw.githubusercontent.com/o/r/v1.2/a/b/c.md");
  });
  it("leaves already-raw URLs unchanged", () => {
    const raw = "https://raw.githubusercontent.com/o/r/main/f.md";
    expect(fromGithubUrl(raw)).toBe(raw);
  });
  it("passes through non-GitHub and malformed URLs", () => {
    expect(fromGithubUrl("https://example.com/x")).toBe("https://example.com/x");
    expect(fromGithubUrl("not a url")).toBe("not a url");
  });
});

describe("fromGoogleDocUrl", () => {
  it("rewrites an edit URL to a txt export URL", () => {
    expect(
      fromGoogleDocUrl("https://docs.google.com/document/d/ABC123/edit?usp=sharing"),
    ).toBe("https://docs.google.com/document/d/ABC123/export?format=txt");
  });
  it("works without trailing path segments", () => {
    expect(
      fromGoogleDocUrl("https://docs.google.com/document/d/XYZ"),
    ).toBe("https://docs.google.com/document/d/XYZ/export?format=txt");
  });
  it("passes through non-doc URLs", () => {
    expect(fromGoogleDocUrl("https://docs.google.com/spreadsheets/d/1")).toBe(
      "https://docs.google.com/spreadsheets/d/1",
    );
  });
});

describe("resolveImportUrl", () => {
  it("routes GitHub blob URLs to raw", () => {
    expect(resolveImportUrl("https://github.com/o/r/blob/main/f.md")).toBe(
      "https://raw.githubusercontent.com/o/r/main/f.md",
    );
  });
  it("routes Google Doc URLs to export", () => {
    expect(resolveImportUrl("https://docs.google.com/document/d/ID/edit")).toBe(
      "https://docs.google.com/document/d/ID/export?format=txt",
    );
  });
  it("leaves arbitrary URLs unchanged", () => {
    expect(resolveImportUrl("https://example.com/script.txt")).toBe(
      "https://example.com/script.txt",
    );
  });
});

describe("fromMarkdown", () => {
  it("renders ATX headings", () => {
    const { html } = fromMarkdown("# Title\n## Sub");
    expect(html).toContain("<h1>Title</h1>");
    expect(html).toContain("<h2>Sub</h2>");
  });
  it("renders bold, italic, and code emphasis", () => {
    const { html } = fromMarkdown("This is **bold**, _italic_, and `code`.");
    expect(html).toContain("<strong>bold</strong>");
    expect(html).toContain("<em>italic</em>");
    expect(html).toContain("<code>code</code>");
  });
  it("groups consecutive lines into a paragraph", () => {
    const { html } = fromMarkdown("line one\nline two\n\nnext para");
    expect(html).toContain("<p>line one line two</p>");
    expect(html).toContain("<p>next para</p>");
  });
  it("escapes HTML in source to prevent injection", () => {
    const { html } = fromMarkdown("a <script>alert(1)</script> b");
    expect(html).not.toContain("<script>");
    expect(html).toContain("&lt;script&gt;");
  });
  it("degrades malformed input to text rather than throwing", () => {
    expect(() => fromMarkdown("**unterminated and # weird")).not.toThrow();
    const { html } = fromMarkdown("**unterminated");
    expect(html).toContain("**unterminated");
  });
});
