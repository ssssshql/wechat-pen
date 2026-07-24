package converter

import (
	"bytes"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	nethtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// highlightCodeBlocks finds <pre><code class="language-xx"> and replaces
// with chroma-highlighted HTML using inline styles (no classes).
func highlightCodeBlocks(root *nethtml.Node, themeName string) {
	var pres []*nethtml.Node
	walk(root, func(n *nethtml.Node) {
		if n.Type == nethtml.ElementNode && n.DataAtom == atom.Pre {
			pres = append(pres, n)
		}
	})
	for _, pre := range pres {
		code := firstChildElement(pre, atom.Code)
		if code == nil {
			continue
		}
		lang := detectLang(code)
		src := textContent(code)
		src = strings.TrimRight(src, "\n")
		highlighted, err := highlightToInlineHTML(src, lang, themeName)
		if err != nil || highlighted == "" {
			continue
		}
		frag, err := nethtml.ParseFragment(strings.NewReader(highlighted), code)
		if err != nil {
			continue
		}
		for c := code.FirstChild; c != nil; {
			next := c.NextSibling
			code.RemoveChild(c)
			c = next
		}
		for _, n := range frag {
			preserveSpaces(n)
			code.AppendChild(n)
		}
		code.Attr = filterLangClass(code.Attr)
	}
}

// preserveSpaces walks text nodes and replaces regular spaces with NBSP
// so WeChat editor does not collapse whitespace in highlighted code.
func preserveSpaces(n *nethtml.Node) {
	if n.Type == nethtml.TextNode {
		// Keep newlines; only convert spaces/tabs so indentation survives paste.
		s := n.Data
		s = strings.ReplaceAll(s, "\t", "    ")
		s = strings.ReplaceAll(s, " ", " ")
		n.Data = s
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		preserveSpaces(c)
	}
}

func firstChildElement(n *nethtml.Node, a atom.Atom) *nethtml.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == nethtml.ElementNode && c.DataAtom == a {
			return c
		}
	}
	return nil
}

func detectLang(code *nethtml.Node) string {
	for _, a := range code.Attr {
		if strings.EqualFold(a.Key, "class") {
			for _, part := range strings.Fields(a.Val) {
				if strings.HasPrefix(part, "language-") {
					return strings.TrimPrefix(part, "language-")
				}
			}
		}
	}
	return ""
}

func filterLangClass(attrs []nethtml.Attribute) []nethtml.Attribute {
	out := make([]nethtml.Attribute, 0, len(attrs))
	for _, a := range attrs {
		if strings.EqualFold(a.Key, "class") {
			continue
		}
		out = append(out, a)
	}
	return out
}

func textContent(n *nethtml.Node) string {
	var b strings.Builder
	var walkText func(*nethtml.Node)
	walkText = func(n *nethtml.Node) {
		if n.Type == nethtml.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkText(c)
		}
	}
	walkText(n)
	return b.String()
}

func highlightToInlineHTML(source, lang, themeName string) (string, error) {
	var lexer chroma.Lexer
	if lang != "" {
		lexer = lexers.Get(lang)
	}
	if lexer == nil {
		lexer = lexers.Analyse(source)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	styleName := mapHighlightTheme(themeName)
	style := styles.Get(styleName)
	if style == nil {
		style = styles.Get("github")
	}
	if style == nil {
		style = styles.Fallback
	}

	formatter := html.New(
		html.WithClasses(false),
		html.PreventSurroundingPre(true),
	)

	iterator, err := lexer.Tokenise(nil, source)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := formatter.Format(&buf, style, iterator); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// chromaThemeColors reads the background and default text color from a chroma style.
func chromaThemeColors(themeName string) (bg, fg string) {
	styleName := mapHighlightTheme(themeName)
	style := styles.Get(styleName)
	if style == nil {
		style = styles.Get("github")
	}
	if style == nil {
		style = styles.Fallback
	}

	bgEntry := style.Get(chroma.Background)
	if bgEntry.Background.IsSet() {
		bg = bgEntry.Background.String()
	} else if bgEntry.Colour.IsSet() {
		bg = bgEntry.Colour.String()
	}

	defaultEntry := style.Get(chroma.Text)
	if defaultEntry.Colour.IsSet() {
		fg = defaultEntry.Colour.String()
	}
	return bg, fg
}

func mapHighlightTheme(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "monokai", "dark":
		return "monokai"
	case "dracula":
		return "dracula"
	case "xcode", "light":
		return "xcode"
	case "github", "":
		return "github"
	default:
		// allow direct chroma style names
		return name
	}
}
