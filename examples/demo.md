# wechat-pen demo

A small Markdown → HTML tool built with [goldmark](https://github.com/yuin/goldmark).

## Features

- **GFM**: tables, task lists, strikethrough
- Auto heading IDs
- Full document mode with a readable default theme
- Safe by default (raw HTML off)

## Task list

- [x] Parse Markdown
- [x] Render HTML
- [ ] Publish to npm (just kidding)

## Table

| Flag        | Meaning              |
| ----------- | -------------------- |
| `-o`        | Output file          |
| `-complete` | Full HTML document   |
| `-unsafe`   | Allow raw HTML       |

## Code

```go
html, err := converter.Convert(src, converter.Config{
    Complete: true,
    Title:    "Hello",
})
```

## Quote

> Goldmark is standards-compliant, fast, and easy to extend.

That's it — ~~simple~~ straightforward.
