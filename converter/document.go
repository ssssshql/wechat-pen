package converter

import (
	"bytes"
	"fmt"
	"html/template"
)

func wrapDocument(body string, cfg Config) ([]byte, error) {
	title := cfg.Title
	if title == "" {
		title = "Document"
	}

	const tpl = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<style>
{{.CSS}}
</style>
</head>
<body>
<article class="markdown-body">
{{.Body}}
</article>
</body>
</html>
`

	css := cfg.CSS
	if css == "" {
		css = defaultCSS
	}

	t, err := template.New("doc").Parse(tpl)
	if err != nil {
		return nil, fmt.Errorf("template: %w", err)
	}

	var buf bytes.Buffer
	err = t.Execute(&buf, struct {
		Title string
		CSS   template.CSS
		Body  template.HTML
	}{
		Title: title,
		CSS:   template.CSS(css),
		Body:  template.HTML(body),
	})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

const defaultCSS = `/* wechat-pen default theme */
:root {
  color-scheme: light dark;
  --bg: #ffffff;
  --fg: #1f2328;
  --muted: #656d76;
  --border: #d0d7de;
  --code-bg: #f6f8fa;
  --link: #0969da;
  --blockquote: #656d76;
}
@media (prefers-color-scheme: dark) {
  :root {
    --bg: #0d1117;
    --fg: #e6edf3;
    --muted: #8b949e;
    --border: #30363d;
    --code-bg: #161b22;
    --link: #2f81f7;
    --blockquote: #8b949e;
  }
}
* { box-sizing: border-box; }
html { font-size: 16px; }
body {
  margin: 0;
  background: var(--bg);
  color: var(--fg);
  font-family: system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
  line-height: 1.6;
}
.markdown-body {
  max-width: 48rem;
  margin: 0 auto;
  padding: 2rem 1.25rem 4rem;
}
h1, h2, h3, h4, h5, h6 {
  line-height: 1.25;
  margin-top: 1.6em;
  margin-bottom: 0.6em;
  font-weight: 650;
}
h1 { font-size: 2rem; border-bottom: 1px solid var(--border); padding-bottom: 0.3em; }
h2 { font-size: 1.5rem; border-bottom: 1px solid var(--border); padding-bottom: 0.3em; }
h3 { font-size: 1.25rem; }
p, ul, ol, pre, table, blockquote { margin: 0 0 1em; }
a { color: var(--link); text-decoration: none; }
a:hover { text-decoration: underline; }
code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 0.9em;
  background: var(--code-bg);
  padding: 0.15em 0.4em;
  border-radius: 4px;
}
pre {
  background: var(--code-bg);
  padding: 1rem;
  overflow: auto;
  border-radius: 8px;
  border: 1px solid var(--border);
}
pre code { background: transparent; padding: 0; font-size: 0.875rem; }
blockquote {
  margin-left: 0;
  padding: 0 1em;
  color: var(--blockquote);
  border-left: 0.25em solid var(--border);
}
table { border-collapse: collapse; width: 100%; display: block; overflow-x: auto; }
th, td { border: 1px solid var(--border); padding: 0.5rem 0.75rem; }
th { background: var(--code-bg); font-weight: 600; }
hr { border: 0; border-top: 1px solid var(--border); margin: 2rem 0; }
img { max-width: 100%; height: auto; }
`
