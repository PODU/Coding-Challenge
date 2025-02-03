# Markdown to PDF — Sample Document

This sample exercises **every** supported element so the renderer and the test
suite have one fixture that touches *all* of them. It is intentionally short.

## Inline emphasis

Plain text, then **bold**, then *italic*, then ***bold italic***, then some
`inline code`, and a [link to the challenge](https://codingchallenges.fyi).
You can also write bold with __underscores__ and italic with _underscores_.

Special characters survive escaping: 5 < 10 & 10 > 5, and a <not-a-tag> stays
literal.

### Unordered lists (with nesting)

- First bullet
- Second bullet
  - Nested bullet
  - Another nested bullet
    - Deeper still
- Back to top level

### Ordered lists (with nesting)

1. First step
2. Second step
   1. Sub-step A
   2. Sub-step B
3. Third step

## Code block

```python
def greet(name: str) -> str:
    # whitespace and indentation are preserved verbatim
    return f"Hello, {name}!"
```

## Image

![A placeholder logo](logo.png)

---

That horizontal rule above is a thematic break. End of sample.
