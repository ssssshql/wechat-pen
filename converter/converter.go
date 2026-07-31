package converter

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Theme selects output mode.
type Theme string

const (
	ThemeDefault Theme = "default"
	ThemeWeChat  Theme = "wechat"
)

// StylePack is a visual style for WeChat inline output.
type StylePack string

const (
	StyleSimple   StylePack = "simple"
	StyleMagazine StylePack = "magazine"
	StyleTech     StylePack = "tech"
	StyleWarm     StylePack = "warm"
	StyleCustom   StylePack = "custom"
)

// Config controls conversion.
type Config struct {
	Unsafe   bool
	Complete bool
	Title    string
	CSS      string
	Theme    Theme

	// WeChat styling
	Style          StylePack // simple|magazine|tech|warm
	PrimaryColor   string    // e.g. #07c160
	TextIndent     bool      // 首行缩进 2em
	Justify        bool      // 两端对齐
	ParagraphGap   string    // e.g. "1em"
	FontSize       string    // e.g. "16px"
	LineHeight     string    // e.g. "1.75"
	Highlight      bool      // code block syntax highlight
	HighlightTheme string    // github | monokai | dracula | xcode
	TOC            bool      // prepend table of contents
	Footer         bool      // append footer tip
	ImageCaption   bool      // figure caption from italic under image
	Report         bool      // collect strip report (filled into Result)
	// PreviewWidth: phone | tablet | full | raw css width
	PreviewWidth string
	// PreviewShell: dark | light phone frame
	PreviewShell string
	// CustomCSS is a JSON string with tag->style overrides for StyleCustom
	CustomCSS string
}

// Result is conversion output plus optional diagnostics.
type Result struct {
	HTML    []byte
	Preview []byte // complete document when available
	Report  CleanReport
	Health  HealthReport
}

// CleanReport lists stripped content for debugging paste issues.
type CleanReport struct {
	DroppedTags  []string `json:"droppedTags"`
	DroppedAttrs []string `json:"droppedAttrs"`
}

// HealthReport flags content that may break when pasting into WeChat.
type HealthReport struct {
	Warnings []HealthWarning `json:"warnings"`
	OK       bool            `json:"ok"`
}

// HealthWarning is a single paste-risk item.
type HealthWarning struct {
	Level   string `json:"level"` // info | warn | error
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Convert turns markdown into HTML.
func Convert(source []byte, cfg Config) ([]byte, error) {
	res, err := ConvertEx(source, cfg)
	if err != nil {
		return nil, err
	}
	return res.HTML, nil
}

// ConvertEx returns HTML plus optional clean report / preview shell.
func ConvertEx(source []byte, cfg Config) (*Result, error) {
	cfg = normalizeConfig(cfg)

	// Preprocess custom markdown: components, image captions
	src := string(source)
	src = expandComponents(src)
	if cfg.ImageCaption {
		src = expandImageCaptions(src)
	}

	opts := []goldmark.Option{
		goldmark.WithExtensions(
			extension.GFM,
			extension.Footnote,
			extension.DefinitionList,
			extension.Typographer,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(gmhtml.WithHardWraps()),
	}
	if cfg.Unsafe || cfg.Theme == ThemeWeChat {
		opts = append(opts, goldmark.WithRendererOptions(gmhtml.WithUnsafe()))
	}

	md := goldmark.New(opts...)
	var body bytes.Buffer
	if err := md.Convert([]byte(src), &body); err != nil {
		return nil, err
	}

	res := &Result{}

	switch cfg.Theme {
	case ThemeWeChat:
		styled, report, err := applyWeChatStyles(body.String(), cfg)
		if err != nil {
			return nil, err
		}
		res.Report = report
		if cfg.TOC {
			styled = injectTOC(styled, cfg)
		}
		if cfg.Footer {
			styled = appendFooter(styled, cfg)
		}
		res.HTML = []byte(styled)
		if cfg.Complete {
			preview, err := wrapWeChatPreview(styled, cfg)
			if err != nil {
				return nil, err
			}
			res.Preview = preview
		}
	default:
		htmlBytes := body.Bytes()
		if cfg.Complete {
			doc, err := wrapDocument(string(htmlBytes), cfg)
			if err != nil {
				return nil, err
			}
			res.HTML = doc
			res.Preview = doc
		} else {
			res.HTML = htmlBytes
			res.Preview = htmlBytes
		}
	}

	if res.Preview == nil {
		res.Preview = res.HTML
	}
	res.Health = analyzeHealth(string(source), string(res.HTML), res.Report)
	return res, nil
}

func normalizeConfig(cfg Config) Config {
	if cfg.Theme == "" {
		cfg.Theme = ThemeDefault
	}
	if cfg.Style == "" {
		cfg.Style = StyleSimple
	}
	// External theme packs are identified by Style id (e.g. "my-brand").
	if cfg.PrimaryColor == "" {
		if pack, ok := GetExternalTheme(string(cfg.Style)); ok && pack.Primary != "" {
			cfg.PrimaryColor = pack.Primary
		} else {
			cfg.PrimaryColor = defaultPrimary(cfg.Style)
		}
	}
	if cfg.ParagraphGap == "" {
		cfg.ParagraphGap = "1em"
	}
	if cfg.FontSize == "" {
		cfg.FontSize = "16px"
	}
	if cfg.LineHeight == "" {
		cfg.LineHeight = "1.75"
	}
	if cfg.HighlightTheme == "" {
		cfg.HighlightTheme = "github"
	}
	return cfg
}

func defaultPrimary(s StylePack) string {
	switch s {
	case StyleMagazine:
		return "#c23a2b"
	case StyleTech:
		return "#3b82f6"
	case StyleWarm:
		return "#d97706"
	default:
		return "#07c160"
	}
}

// :::toc / :::footer / :::profile{...} / :::mp-link{...}
var (
	reComponent = regexp.MustCompile(`(?m)^:::([a-zA-Z0-9_-]+)(?:\{([^}]*)\})?\s*$`)
	reImgCap    = regexp.MustCompile(`(?m)^!\[([^\]]*)\]\(([^)]+)\)\s*\n\*(.+?)\*\s*$`)
)

func expandImageCaptions(src string) string {
	return reImgCap.ReplaceAllStringFunc(src, func(m string) string {
		sub := reImgCap.FindStringSubmatch(m)
		if len(sub) < 4 {
			return m
		}
		alt, url, cap := sub[1], sub[2], sub[3]
		return fmt.Sprintf(
			`<figure style="margin:1.2em 0;text-align:center;"><img src="%s" alt="%s" style="display:block;max-width:100%%;height:auto;margin:0 auto;border-radius:4px;"/><figcaption style="display:block;margin-top:0.5em;font-size:13px;color:#888;line-height:1.5;">%s</figcaption></figure>`,
			htmlEscape(url), htmlEscape(alt), htmlEscape(cap),
		)
	})
}

func expandComponents(src string) string {
	lines := strings.Split(src, "\n")
	var out []string
	for _, line := range lines {
		m := reComponent.FindStringSubmatch(line)
		if m == nil {
			out = append(out, line)
			continue
		}
		name := strings.ToLower(m[1])
		attrs := parseInlineAttrs(m[2])
		switch name {
		case "toc":
			// marker; real TOC injected later if enabled, or inline placeholder
			out = append(out, `<!--TOC-->`)
		case "footer":
			out = append(out, `<!--FOOTER-->`)
		case "profile":
			// Raw mp-common-profile: user provides real WeChat attributes.
			// Known keys get auto-prefixed with "data-".
			guide := `<p style="display:block;margin:1.2em 0 0.3em;font-size:13px;color:#888;text-align:center;text-indent:0;">👇 点击下方名片关注</p>`
			tagAttrs := ` data-pluginname="mpprofile"`
			dataKeys := map[string]bool{
				"pluginname": true, "id": true, "headimg": true, "nickname": true,
				"alias": true, "signature": true, "from": true, "is_biz_ban": true,
			}
			for k, v := range attrs {
				if v == "" {
					continue
				}
				key := htmlEscape(k)
				if dataKeys[k] && !strings.HasPrefix(key, "data-") && key != "data-pluginname" {
					key = "data-" + key
				}
				tagAttrs += fmt.Sprintf(` %s="%s"`, key, htmlEscape(v))
			}
			tag := `<mp-common-profile` + tagAttrs + ` contenteditable="false"></mp-common-profile>`
			out = append(out, guide+tag)

		case "mp-link", "miniprogram":
			out = append(out, `<p style="display:block;text-align:center;margin:1.6em 0;color:#ccc;letter-spacing:0.4em;">· · ·</p>`)
		case "follow", "cta":
			text := attrs["text"]
			if text == "" {
				text = "长按识别二维码，关注我们"
			}
			out = append(out, fmt.Sprintf(
				`<section style="margin:1.6em 0;padding:16px;border-radius:10px;background:#f7fafc;border:1px solid #e5eef8;text-align:center;">`+
					`<p style="margin:0;font-size:16px;font-weight:bold;color:#1a1a1a;text-indent:0;">点个「在看」再走</p>`+
					`<p style="margin:8px 0 0;font-size:13px;color:#666;line-height:1.6;text-indent:0;">%s</p>`+
					`</section>`, htmlEscape(text),
			))
		case "author":
			name := orDefault(attrs["name"], "作者")
			desc := orDefault(attrs["desc"], "欢迎关注，获取更多干货")
			out = append(out, fmt.Sprintf(
				`<section style="margin:1.4em 0;padding:14px;border-radius:10px;border:1px solid #eee;background:#fafafa;display:block;">`+
					`<p style="margin:0;font-size:15px;font-weight:bold;color:#1a1a1a;text-indent:0;">%s</p>`+
					`<p style="margin:6px 0 0;font-size:13px;color:#888;line-height:1.6;text-indent:0;">%s</p>`+
					`</section>`, htmlEscape(name), htmlEscape(desc),
			))
		case "steps":
			var items []string
			for i := 1; i <= 8; i++ {
				k := fmt.Sprintf("%d", i)
				if v, ok := attrs[k]; ok && v != "" {
					items = append(items, fmt.Sprintf(
						`<p style="margin:0.4em 0;font-size:15px;color:#333;line-height:1.7;text-indent:0;"><span style="display:inline-block;width:22px;height:22px;line-height:22px;text-align:center;border-radius:50%%;background:{{primary}};color:#fff;font-size:12px;margin-right:8px;">%d</span>%s</p>`,
						i, htmlEscape(v),
					))
				}
			}
			if len(items) == 0 {
				items = []string{
					`<p style="margin:0.4em 0;font-size:15px;color:#333;text-indent:0;">1. 第一步</p>`,
					`<p style="margin:0.4em 0;font-size:15px;color:#333;text-indent:0;">2. 第二步</p>`,
					`<p style="margin:0.4em 0;font-size:15px;color:#333;text-indent:0;">3. 第三步</p>`,
				}
			}
			out = append(out, `<section style="margin:1.2em 0;padding:12px 14px;border-left:4px solid {{primary}};background:#fafafa;border-radius:0 8px 8px 0;">`+strings.Join(items, "")+`</section>`)
		case "grid3", "threecol":
			a := orDefault(attrs["a"], "要点一")
			b := orDefault(attrs["b"], "要点二")
			c := orDefault(attrs["c"], "要点三")
			cell := func(t string) string {
				return fmt.Sprintf(
					`<section style="display:inline-block;width:31%%;vertical-align:top;margin:0 1%%;padding:12px 8px;background:#f7f7f7;border-radius:8px;text-align:center;box-sizing:border-box;">`+
						`<p style="margin:0;font-size:14px;color:#333;font-weight:bold;text-indent:0;">%s</p></section>`,
					htmlEscape(t),
				)
			}
			out = append(out, `<section style="margin:1.2em 0;text-align:center;font-size:0;">`+cell(a)+cell(b)+cell(c)+`</section>`)
		case "quote", "gold":
			text := orDefault(attrs["text"], "把复杂留给自己，把简单留给用户。")
			from := orDefault(attrs["from"], "")
			fromHTML := ""
			if from != "" {
				fromHTML = fmt.Sprintf(`<p style="margin:10px 0 0;font-size:13px;color:#999;text-align:right;text-indent:0;">—— %s</p>`, htmlEscape(from))
			}
			out = append(out, fmt.Sprintf(
				`<section style="margin:1.4em 0;padding:18px 16px;border-radius:12px;background:#fff7ed;border:1px solid #fde68a;">`+
					`<p style="margin:0;font-size:17px;line-height:1.7;color:#78350f;font-weight:600;text-align:center;text-indent:0;">“%s”</p>%s</section>`,
				htmlEscape(text), fromHTML,
			))
		case "faq":
			q := orDefault(attrs["q"], "常见问题？")
			a := orDefault(attrs["a"], "这里是回答内容。")
			out = append(out, fmt.Sprintf(
				`<section style="margin:1em 0;padding:12px 14px;border-radius:10px;border:1px solid #e5e5e5;background:#fafafa;">`+
					`<p style="margin:0;font-size:15px;font-weight:bold;color:#1a1a1a;text-indent:0;">Q：%s</p>`+
					`<p style="margin:8px 0 0;font-size:14px;color:#555;line-height:1.7;text-indent:0;">A：%s</p></section>`,
				htmlEscape(q), htmlEscape(a),
			))
		case "titlebar":
			text := orDefault(attrs["text"], "章节标题")
			out = append(out, fmt.Sprintf(
				`<section style="margin:1.5em 0 1em;padding:10px 14px;border-radius:8px;background:{{primary}};">`+
					`<p style="margin:0;font-size:16px;font-weight:bold;color:#fff;text-align:center;text-indent:0;">%s</p></section>`,
				htmlEscape(text),
			))
		case "kicker":
			text := orDefault(attrs["text"], "导读")
			out = append(out, fmt.Sprintf(
				`<p style="margin:0 0 1em;padding:10px 12px;border-left:4px solid {{primary}};background:#f8fafc;font-size:14px;color:#475569;line-height:1.7;text-indent:0;">%s</p>`,
				htmlEscape(text),
			))
		default:
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func parseInlineAttrs(s string) map[string]string {
	out := map[string]string{}
	s = strings.TrimSpace(s)
	if s == "" {
		return out
	}
	// key="value" or key=value pairs separated by space/comma
	re := regexp.MustCompile(`([a-zA-Z0-9_-]+)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s,]+))`)
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		val := m[2]
		if val == "" {
			val = m[3]
		}
		if val == "" {
			val = m[4]
		}
		out[m[1]] = val
	}
	return out
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

func applyWeChatStyles(fragment string, cfg Config) (string, CleanReport, error) {
	report := CleanReport{}
	pack := buildStylePack(cfg)

	doc, err := html.Parse(strings.NewReader(`<div id="__root__">` + fragment + `</div>`))
	if err != nil {
		return "", report, fmt.Errorf("parse html: %w", err)
	}
	root := findID(doc, "__root__")
	if root == nil {
		return "", report, fmt.Errorf("internal: root not found")
	}

	report.DroppedTags = pruneDangerous(root)

	// Highlight code blocks before generic styling
	if cfg.Highlight {
		highlightCodeBlocks(root, cfg.HighlightTheme)
	}

	// Read chroma theme colors for <pre>/<code> overrides
	hlBG, hlFG := chromaThemeColors(cfg.HighlightTheme)

	// Track dropped attrs
	attrDrops := map[string]struct{}{}

	// Auto-increment counter for {{num}} in tag templates
	tagNumCounter := 0

	walk(root, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		before := len(n.Attr)
		n.Attr, _ = filterAttrsReport(n.Attr, attrDrops)
		_ = before

		// Preserve already-highlighted spans
		if n.Data == "span" && hasAttr(n, "style") && styleContains(attrVal(n, "style"), "color:") {
			return
		}

		style, ok := pack.Tags[n.Data]
		if !ok {
			// still handle special cases
			applySpecial(n, pack, cfg)
			return
		}

		// If the tag value contains {{content}}, treat it as an HTML template.
		// {{content}} is replaced with the original tag's inner HTML.
		// {{num}} is replaced with an auto-incrementing 01, 02, 03...
		// {{primary}} is replaced with the configured primary color.
		if strings.Contains(style, "{{content}}") {
			tagNumCounter++
			inner := nodeInnerHTML(n)
			tpl := style
			tpl = strings.ReplaceAll(tpl, "{{content}}", inner)
			tpl = strings.ReplaceAll(tpl, "{{num}}", fmt.Sprintf("%02d", tagNumCounter))
			tpl = strings.ReplaceAll(tpl, "{{primary}}", cfg.PrimaryColor)

			parsed, err := html.Parse(strings.NewReader(tpl))
			if err == nil {
				if body := findNode(parsed, "body"); body != nil && body.FirstChild != nil {
					replacement := body.FirstChild
					body.RemoveChild(replacement)
					n.Parent.InsertBefore(replacement, n)
					n.Parent.RemoveChild(n)
				}
			}
			return
		}

		// Dynamic paragraph style
		if n.Data == "p" {
			style = pack.Paragraph(cfg)
		}
		style = strings.ReplaceAll(style, "{{primary}}", cfg.PrimaryColor)
		if n.Data == "code" && n.Parent != nil && n.Parent.DataAtom == atom.Pre {
			setStyleAbsolute(n, pack.PreCode)
			// Override default text color from chroma theme
			if hlFG != "" {
				st := attrVal(n, "style")
				st = replaceCSSProp(st, "color", hlFG)
				setStyleAbsolute(n, st)
			}
			applySpecial(n, pack, cfg)
			return
		}
		setStyle(n, style)
		// Override <pre> background/color from chroma theme
		if n.Data == "pre" {
			st := attrVal(n, "style")
			changed := false
			if hlBG != "" {
				st = replaceCSSProp(st, "background", hlBG)
				st = replaceCSSProp(st, "background-color", hlBG)
				changed = true
			}
			if hlFG != "" {
				st = replaceCSSProp(st, "color", hlFG)
				changed = true
			}
			if changed {
				setStyleAbsolute(n, st)
			}
		}
		applySpecial(n, pack, cfg)
	})

	for a := range attrDrops {
		report.DroppedAttrs = append(report.DroppedAttrs, a)
	}

	var buf bytes.Buffer
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		if err := html.Render(&buf, c); err != nil {
			return "", report, err
		}
	}

	base := pack.Base
	base = strings.ReplaceAll(base, "{{primary}}", cfg.PrimaryColor)
	base = strings.ReplaceAll(base, "{{fontSize}}", cfg.FontSize)
	base = strings.ReplaceAll(base, "{{lineHeight}}", cfg.LineHeight)
	if cfg.Justify {
		if !strings.Contains(base, "text-align") {
			base += "text-align:justify;"
		}
	} else {
		base = strings.ReplaceAll(base, "text-align:justify;", "text-align:left;")
	}

	bodyHTML := strings.ReplaceAll(buf.String(), "{{primary}}", cfg.PrimaryColor)
	outer := fmt.Sprintf(`<section style="%s">%s</section>`, escapeAttr(base), bodyHTML)
	return outer, report, nil
}

func applySpecial(n *html.Node, pack stylePack, cfg Config) {
	switch n.DataAtom {
	case atom.A:
		if !hasAttr(n, "target") {
			n.Attr = append(n.Attr, html.Attribute{Key: "target", Val: "_blank"})
		}
	case atom.Img:
		st := attrVal(n, "style")
		if !styleContains(st, "max-width") {
			setStyle(n, "max-width:100%;height:auto;display:block;margin:1em auto;border-radius:4px;")
		}
	case atom.Code:
		if n.Parent != nil && n.Parent.DataAtom == atom.Pre {
			// Always override pink chip style for fenced blocks.
			setStyleAbsolute(n, pack.PreCode)
		}
	case atom.Figcaption:
		setStyle(n, `display:block;margin-top:0.5em;font-size:13px;color:#888;line-height:1.5;text-align:center;`)
	case atom.Figure:
		setStyle(n, `display:block;margin:1.2em 0;text-align:center;`)
	}
}

func hasColoredDescendant(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "span" {
			if styleContains(attrVal(c, "style"), "color") {
				return true
			}
		}
		if hasColoredDescendant(c) {
			return true
		}
	}
	return false
}

func attrVal(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

// findNode returns the first descendant element with the given tag name.
func findNode(root *html.Node, tag string) *html.Node {
	if root.Type == html.ElementNode && root.Data == tag {
		return root
	}
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		if r := findNode(c, tag); r != nil {
			return r
		}
	}
	return nil
}

// nodeInnerHTML renders the inner HTML of a node (children only, no outer tag).
func nodeInnerHTML(n *html.Node) string {
	var buf bytes.Buffer
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		html.Render(&buf, c)
	}
	return buf.String()
}

func setStyleAbsolute(n *html.Node, style string) {
	idx := -1
	for i, a := range n.Attr {
		if strings.EqualFold(a.Key, "style") {
			idx = i
			break
		}
	}
	if idx >= 0 {
		n.Attr[idx].Val = style
	} else {
		n.Attr = append(n.Attr, html.Attribute{Key: "style", Val: style})
	}
}

func walk(n *html.Node, fn func(*html.Node)) {
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		fn(c)
		walk(c, fn)
		c = next
	}
}

// replaceCSSProp replaces the value of a CSS property in an inline style string.
func replaceCSSProp(style, prop, newVal string) string {
	// Match "prop:value;" or "prop:value" at end
	sep := prop + ":"
	idx := strings.Index(style, sep)
	if idx < 0 {
		return style + sep + newVal + ";"
	}
	start := idx + len(sep)
	end := strings.Index(style[start:], ";")
	if end < 0 {
		return style[:start] + newVal
	}
	return style[:start] + newVal + style[start+end:]
}

var dropTags = map[string]bool{
	"script": true, "style": true, "object": true,
	"embed": true, "form": true, "input": true, "textarea": true,
	"button": true, "select": true, "canvas": true, "svg": true,
	// keep iframe for tencent video? drop by default — user can use unsafe components
	"iframe": true,
}

func pruneDangerous(n *html.Node) []string {
	var dropped []string
	seen := map[string]struct{}{}
	var walkPrune func(*html.Node)
	walkPrune = func(n *html.Node) {
		for c := n.FirstChild; c != nil; {
			next := c.NextSibling
			if c.Type == html.ElementNode && dropTags[c.Data] {
				if _, ok := seen[c.Data]; !ok {
					seen[c.Data] = struct{}{}
					dropped = append(dropped, c.Data)
				}
				n.RemoveChild(c)
			} else {
				walkPrune(c)
			}
			c = next
		}
	}
	walkPrune(n)
	return dropped
}

func findID(n *html.Node, id string) *html.Node {
	if n.Type == html.ElementNode {
		for _, a := range n.Attr {
			if a.Key == "id" && a.Val == id {
				return n
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findID(c, id); found != nil {
			return found
		}
	}
	return nil
}

func filterAttrsReport(attrs []html.Attribute, drops map[string]struct{}) ([]html.Attribute, map[string]struct{}) {
	keep := make([]html.Attribute, 0, len(attrs))
	for _, a := range attrs {
		k := strings.ToLower(a.Key)
		switch {
		case strings.HasPrefix(k, "on"):
			drops[k] = struct{}{}
			continue
		case k == "id" || k == "class":
			// keep weapp_text_link class for mini program
			if k == "class" && strings.Contains(a.Val, "weapp_text_link") {
				keep = append(keep, a)
				continue
			}
			drops[k] = struct{}{}
			continue
		default:
			keep = append(keep, a)
		}
	}
	return keep, drops
}

func setStyle(n *html.Node, style string) {
	existing := ""
	idx := -1
	for i, a := range n.Attr {
		if strings.EqualFold(a.Key, "style") {
			existing = a.Val
			idx = i
			break
		}
	}
	merged := mergeStyles(existing, style)
	if idx >= 0 {
		n.Attr[idx].Val = merged
	} else {
		n.Attr = append(n.Attr, html.Attribute{Key: "style", Val: merged})
	}
}

func hasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return true
		}
	}
	return false
}

func styleContains(style, prop string) bool {
	return strings.Contains(strings.ToLower(style), strings.ToLower(prop))
}

func mergeStyles(existing, theme string) string {
	theme = strings.TrimSpace(theme)
	existing = strings.TrimSpace(existing)
	if existing == "" {
		return theme
	}
	if theme == "" {
		return existing
	}
	if !strings.HasSuffix(theme, ";") {
		theme += ";"
	}
	// theme first, existing overrides
	return theme + existing
}

func escapeAttr(s string) string {
	s = strings.ReplaceAll(s, `&`, "&amp;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, `<`, "&lt;")
	s = strings.ReplaceAll(s, `>`, "&gt;")
	return s
}
