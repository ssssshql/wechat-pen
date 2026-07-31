package converter

import (
	"fmt"
	"strings"
)

type stylePack struct {
	Base    string
	Tags    map[string]string
	PreCode string
}

func (p stylePack) Paragraph(cfg Config) string {
	if tpl, ok := p.Tags["p"]; ok && strings.TrimSpace(tpl) != "" {
		return applyParagraphTemplate(tpl, cfg)
	}
	return applyParagraphTemplate(
		`display:block;margin:0 0 1em;font-size:16px;color:#333;line-height:1.75;letter-spacing:0.034em;text-align:{{align}};{{indent}}`,
		cfg,
	)
}

func applyParagraphTemplate(tpl string, cfg Config) string {
	align := "justify"
	if !cfg.Justify {
		align = "left"
	}
	indent := ""
	if cfg.TextIndent {
		indent = "text-indent:2em;"
	}
	gap := cfg.ParagraphGap
	if gap == "" {
		gap = "1em"
	}
	fs := cfg.FontSize
	if fs == "" {
		fs = "16px"
	}
	lh := cfg.LineHeight
	if lh == "" {
		lh = "1.75"
	}
	out := tpl
	out = strings.ReplaceAll(out, "{{gap}}", gap)
	out = strings.ReplaceAll(out, "{{fontSize}}", fs)
	out = strings.ReplaceAll(out, "{{lineHeight}}", lh)
	out = strings.ReplaceAll(out, "{{align}}", align)
	out = strings.ReplaceAll(out, "{{indent}}", indent)
	return out
}

func buildStylePack(cfg Config) stylePack {
	if pack, ok := GetExternalTheme(string(cfg.Style)); ok {
		return packFromExternal(cfg, pack)
	}
	return builtinPack(cfg)
}

// h1Template builds a chapter-style h1 using the {{content}} template system.
func h1Template(accent, label, numColor, titleColor string) string {
	return fmt.Sprintf(
		`<section style="margin:32px 0 24px;padding-left:16px;border-left:4px solid %s;">`+
			`<p style="margin:0 0 4px;font-size:11px;font-weight:600;color:%s;letter-spacing:2.4px;text-transform:uppercase;line-height:1;">%s {{num}}</p>`+
			`<strong style="display:block;font-size:48px;line-height:1;color:%s;letter-spacing:-2px;opacity:0.18;font-weight:800;">{{num}}</strong>`+
			`<strong style="display:block;font-size:26px;font-weight:800;color:%s;line-height:1.3;letter-spacing:-0.5px;margin-top:-40px;">{{content}}</strong>`+
			`</section>`,
		accent, accent, label, numColor, titleColor,
	)
}

// h1Centered builds a centered chapter-style h1 with a horizontal rule divider.
func h1Centered(ruleColor, label, numColor, titleColor string) string {
	return fmt.Sprintf(
		`<section style="margin:36px 0 24px;text-align:center;">`+
			`<p style="margin:0 0 10px;font-size:10px;font-weight:700;color:%s;letter-spacing:3px;text-transform:uppercase;">%s {{num}}</p>`+
			`<section style="display:flex;align-items:center;margin:0 0 16px;"><section style="flex:1;border-top:1px solid %s;"></section><span style="margin:0 12px;font-size:20px;color:%s;opacity:0.3;">{{num}}</span><section style="flex:1;border-top:1px solid %s;"></section></section>`+
			`<strong style="display:block;font-size:28px;font-weight:800;color:%s;line-height:1.3;letter-spacing:0.06em;">{{content}}</strong>`+
			`</section>`,
		ruleColor, label, ruleColor, numColor, ruleColor, titleColor,
	)
}

// h1BigNum builds a style with a large background watermark number.
func h1BigNum(numColor, label, titleColor string) string {
	return fmt.Sprintf(
		`<section style="margin:36px 0 24px;">`+
			`<section style="display:flex;align-items:center;padding-bottom:12px;">`+
			`<span style="font-size:10px;font-weight:800;color:%s;letter-spacing:2.6px;text-transform:uppercase;">%s {{num}}</span>`+
			`<section style="flex:1 1 0%%;border-top:1px solid #e5e7eb;margin:0 0 0 12px;height:0;"></section>`+
			`</section>`+
			`<section style="position:relative;">`+
			`<strong style="display:block;font-size:60px;line-height:1;color:%s;letter-spacing:-3px;white-space:nowrap;opacity:0.25;">{{num}}</strong>`+
			`<strong style="display:block;font-size:30px;font-weight:900;color:%s;line-height:1.26;letter-spacing:-0.8px;margin-top:-60px;margin-left:50px;word-break:break-all;">{{content}}</strong>`+
			`</section></section>`,
		numColor, label, numColor, titleColor,
	)
}

// h2AccentBar builds a left-accent-bar h2 with a small color block.
func h2AccentBar(accent, bgColor, titleColor string) string {
	return fmt.Sprintf(
		`<section style="display:flex;align-items:stretch;margin:1.5em 0 0.7em;gap:12px;">`+
			`<section style="flex-shrink:0;width:4px;background:%s;border-radius:2px;"></section>`+
			`<section style="flex:1;padding:10px 0 10px 4px;">`+
			`<strong style="display:block;font-size:18px;font-weight:700;color:#%s;line-height:1.4;letter-spacing:0.04em;">{{content}}</strong>`+
			`</section></section>`,
		accent, titleColor,
	)
}

// h2Centered builds a centered h2 with decorative lines.
func h2Centered(lineColor, titleColor string) string {
	return fmt.Sprintf(
		`<section style="display:flex;align-items:center;margin:2em 0 0.9em;gap:16px;">`+
			`<section style="flex:1;height:0;border-top:1px solid %s;"></section>`+
			`<strong style="display:block;font-size:18px;font-weight:700;color:#%s;line-height:1.35;text-align:center;letter-spacing:0.12em;white-space:nowrap;">{{content}}</strong>`+
			`<section style="flex:1;height:0;border-top:1px solid %s;"></section>`+
			`</section>`,
		lineColor, titleColor, lineColor,
	)
}

// h2Pill builds a rounded pill/badge h2.
func h2Pill(bgColor, textColor string) string {
	return fmt.Sprintf(
		`<section style="margin:1.5em 0 0.8em;">`+
			`<strong style="display:inline-block;font-size:16px;font-weight:700;color:%s;line-height:1.4;padding:6px 16px;background:%s;border-radius:999px;letter-spacing:0.02em;">{{content}}</strong>`+
			`</section>`,
		textColor, bgColor,
	)
}

// h2Card builds a card-style h2 with border and background.
func h2Card(borderColor, bgColor, accent, titleColor string) string {
	return fmt.Sprintf(
		`<section style="margin:1.5em 0 0.8em;padding:12px 16px;background:%s;border:1px solid %s;border-left:4px solid %s;border-radius:8px;">`+
			`<strong style="display:block;font-size:17px;font-weight:700;color:#%s;line-height:1.35;">{{content}}</strong>`+
			`</section>`,
		bgColor, borderColor, accent, titleColor,
	)
}

// h2Glow builds a glowing accent-bar h2 (for dark theme).
func h2Glow(accent, titleColor string) string {
	return fmt.Sprintf(
		`<section style="display:flex;align-items:center;margin:1.4em 0 0.7em;gap:12px;">`+
			`<section style="flex-shrink:0;width:3px;height:22px;background:%s;border-radius:2px;box-shadow:0 0 8px %s60;"></section>`+
			`<strong style="display:block;font-size:18px;font-weight:800;color:%s;line-height:1.35;">{{content}}</strong>`+
			`</section>`,
		accent, accent, titleColor,
	)
}

// ---------- 简约：干净留白 + 左侧色条章节号 ----------
func simplePack(primary string) stylePack {
	return stylePack{
		Base:    `margin:0 auto;padding:0 4px;max-width:677px;font-size:16px;color:#2c2c2c;line-height:1.75;letter-spacing:0.025em;word-wrap:break-word;font-family:-apple-system,BlinkMacSystemFont,"Helvetica Neue","PingFang SC","Hiragino Sans GB","Microsoft YaHei UI","Microsoft YaHei",Arial,sans-serif;text-align:justify;`,
		PreCode: `display:block;font-family:"JetBrains Mono","Fira Code",Consolas,Monaco,"Courier New",monospace;font-size:13.5px;color:#2c2c2c;background:transparent;padding:0;margin:0;border-radius:0;white-space:pre;word-break:normal;word-wrap:normal;`,
		Tags: map[string]string{
			"h1":         h1Template("{{primary}}", "SECTION", "#94a3b8", "#1a1a1a"),
			"h2":         `display:block;font-size:18px;font-weight:700;color:#1a1a1a;line-height:1.4;margin:1.5em 0 0.65em;padding-bottom:0.35em;border-bottom:2px solid {{primary}};letter-spacing:0.04em;`,
			"h3":         `display:block;font-size:17px;font-weight:600;color:#1a1a1a;line-height:1.4;margin:1.3em 0 0.55em;`,
			"h4":         `display:block;font-size:16px;font-weight:600;color:#333;line-height:1.4;margin:1.2em 0 0.5em;`,
			"h5":         `display:block;font-size:15px;font-weight:600;color:#555;line-height:1.4;margin:1.1em 0 0.5em;`,
			"h6":         `display:block;font-size:15px;font-weight:600;color:#888;line-height:1.4;margin:1em 0 0.5em;`,
			"p":          `display:block;margin:0 0 1em;font-size:16px;color:#2c2c2c;line-height:1.75;letter-spacing:0.025em;text-align:{{align}};{{indent}}`,
			"strong":     `font-weight:700;color:#1a1a1a;`,
			"b":          `font-weight:700;color:#1a1a1a;`,
			"em":         `font-style:italic;`,
			"i":          `font-style:italic;`,
			"u":          `text-decoration:underline;text-underline-offset:2px;`,
			"s":          `text-decoration:line-through;color:#aaa;`,
			"del":        `text-decoration:line-through;color:#aaa;`,
			"a":          `color:#576b95;text-decoration:none;border-bottom:1px solid rgba(87,107,149,0.25);transition:border-color 0.2s;`,
			"ul":         `display:block;margin:0.3em 0 1em;padding-left:1.5em;list-style-type:disc;color:#2c2c2c;`,
			"ol":         `display:block;margin:0.3em 0 1em;padding-left:1.5em;list-style-type:decimal;color:#2c2c2c;`,
			"li":         `display:list-item;margin:0.35em 0;line-height:1.75;`,
			"blockquote": `display:block;margin:1.1em 0;padding:1em 1.2em;color:#555;background:linear-gradient(135deg,#f9fafb 0%,#f5f6f8 100%);border-radius:10px;border-left:4px solid {{primary}};font-size:15px;line-height:1.8;`,
			"code":       `font-family:"JetBrains Mono","Fira Code",Consolas,Monaco,monospace;font-size:13.5px;color:#c7254e;background:#fdf2f5;padding:2px 6px;border-radius:5px;word-break:break-word;`,
			"pre":        `display:block;margin:1em 0;padding:16px 18px;background:#f8f9fb;border-radius:10px;border:1px solid #e8eaef;overflow-x:auto;line-height:1.6;font-size:13.5px;white-space:pre;word-wrap:normal;word-break:normal;box-shadow:inset 0 1px 2px rgba(0,0,0,.03);`,
			"img":        `display:block;max-width:100%;height:auto;margin:1.2em auto;border-radius:8px;`,
			"hr":         `display:block;border:none;height:0;margin:2em 0;border-top:1px solid #e8eaef;`,
			"table":      `display:table;border-collapse:collapse;width:100%;margin:1em 0;font-size:14px;border-radius:8px;overflow:hidden;border:1px solid #e8eaef;`,
			"th":         `border:1px solid #e0e2e7;padding:10px 14px;background:#f6f8fa;font-weight:600;text-align:left;color:#1a1a1a;font-size:13.5px;`,
			"td":         `border:1px solid #e8eaef;padding:10px 14px;color:#2c2c2c;`,
			"section":    `display:block;margin:0;padding:0;`,
			"div":        `display:block;margin:0;padding:0;`,
			"figure":     `display:block;margin:1.4em 0;text-align:center;`,
			"figcaption": `display:block;margin-top:0.6em;font-size:13px;color:#999;line-height:1.6;text-align:center;`,
		},
	}
}

// ---------- 杂志：全居中衬线 + 对称分割线章节标题 ----------
func magazinePack(primary string) stylePack {
	p := simplePack(primary)
	p.Base = `margin:0 auto;padding:8px 12px;max-width:640px;font-size:16px;color:#2a2a2a;line-height:1.8;letter-spacing:0.06em;word-wrap:break-word;font-family:Georgia,"Songti SC",SimSun,"Times New Roman",serif;text-align:justify;`
	p.Tags["h1"] = h1Centered("{{primary}}", "CHAPTER", "#94a3b8", "#111")
	p.Tags["h2"] = fmt.Sprintf(
		`<section style="display:flex;align-items:center;margin:2em 0 0.9em;gap:16px;">`+
			`<section style="flex:1;height:0;border-top:1px solid %s;"></section>`+
			`<strong style="display:block;font-size:18px;font-weight:700;color:#111;line-height:1.35;text-align:center;letter-spacing:0.12em;white-space:nowrap;">{{content}}</strong>`+
			`<section style="flex:1;height:0;border-top:1px solid %s;"></section>`+
			`</section>`,
		"{{primary}}", "{{primary}}",
	)
	p.Tags["h3"] = `display:block;font-size:15px;font-weight:700;color:{{primary}};line-height:1.4;margin:1.6em 0 0.7em;text-align:center;letter-spacing:0.2em;text-transform:uppercase;`
	p.Tags["p"] = `display:block;margin:0 0 1.2em;font-size:16px;color:#2a2a2a;line-height:1.8;letter-spacing:0.05em;text-align:{{align}};{{indent}}`
	p.Tags["blockquote"] = `display:block;margin:1.6em 1.5em;padding:1em 0;color:#666;background:transparent;border:none;border-top:1px solid #ddd;border-bottom:1px solid #ddd;font-size:15px;line-height:1.9;font-style:italic;text-align:center;`
	p.Tags["hr"] = `display:block;border:none;margin:2.2em auto;height:0;width:48%;border-top:1px solid {{primary}};opacity:0.55;`
	p.Tags["ul"] = `display:block;margin:0.6em auto 1.2em;padding:0;list-style-type:none;color:#2a2a2a;max-width:92%;`
	p.Tags["ol"] = `display:block;margin:0.6em auto 1.2em;padding:0 0 0 1.2em;list-style-type:decimal;color:#2a2a2a;max-width:92%;`
	p.Tags["li"] = `display:list-item;margin:0.5em 0;line-height:1.8;text-align:left;`
	p.Tags["pre"] = `display:block;margin:1.4em 0;padding:16px 18px;background:#f4f1ea;border-radius:0;border:none;border-top:2px solid #e5dfd3;border-bottom:2px solid #e5dfd3;overflow-x:auto;line-height:1.6;font-size:13px;white-space:pre;`
	p.Tags["code"] = `font-family:Consolas,Monaco,Courier New,monospace;font-size:13.5px;color:#8b4513;background:#f4f1ea;padding:1px 5px;border-radius:0;`
	p.Tags["img"] = `display:block;max-width:100%;height:auto;margin:1.4em auto;border-radius:0;box-shadow:0 8px 24px rgba(0,0,0,.08);`
	p.Tags["table"] = `display:table;border-collapse:collapse;width:100%;margin:1.4em 0;font-size:13.5px;`
	p.Tags["th"] = `border:none;border-bottom:2px solid #111;padding:10px 12px;background:transparent;font-weight:700;text-align:left;color:#111;letter-spacing:0.06em;`
	p.Tags["td"] = `border:none;border-bottom:1px solid #e5e5e5;padding:10px 12px;color:#333;`
	p.Tags["figcaption"] = `display:block;margin-top:0.6em;font-size:12px;color:#999;line-height:1.5;text-align:center;font-style:italic;letter-spacing:0.08em;`
	return p
}

// ---------- 科技：代码标签风格章节标题 ----------
func techPack(primary string) stylePack {
	p := simplePack(primary)
	p.Base = `margin:0 auto;padding:0 2px;max-width:700px;font-size:15px;color:#0f172a;line-height:1.7;letter-spacing:0.01em;word-wrap:break-word;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",PingFang SC,Microsoft YaHei,sans-serif;text-align:left;`
	p.Tags["h1"] = fmt.Sprintf(
		`<section style="margin:32px 0 20px;padding:16px 20px;background:linear-gradient(135deg,#f0f9ff,#f8fafc);border:1px solid #e0f2fe;border-radius:12px;">`+
			`<p style="margin:0 0 6px;font-size:11px;font-weight:700;color:%s;letter-spacing:1.5px;font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;">SECTION {{num}}</p>`+
			`<strong style="display:block;font-size:26px;font-weight:800;color:#0f172a;line-height:1.25;letter-spacing:-0.02em;">{{content}}</strong>`+
			`<section style="margin-top:10px;height:3px;width:48px;background:%s;border-radius:2px;"></section>`+
			`</section>`,
		"{{primary}}", "{{primary}}",
	)
	p.Tags["h2"] = fmt.Sprintf(
		`<section style="display:flex;align-items:center;margin:1.5em 0 0.7em;gap:10px;padding:10px 14px;background:linear-gradient(90deg,rgba(59,130,246,0.12),transparent);border-left:4px solid %s;border-radius:0 10px 10px 0;">`+
			`<strong style="display:block;font-size:18px;font-weight:800;color:#0f172a;line-height:1.3;">{{content}}</strong>`+
			`</section>`,
		"{{primary}}",
	)
	p.Tags["h3"] = `display:block;font-size:16px;font-weight:700;color:#1e293b;line-height:1.35;margin:1.2em 0 0.5em;padding-left:0;border-bottom:1px dashed #cbd5e1;padding-bottom:0.25em;`
	p.Tags["p"] = `display:block;margin:0 0 0.8em;font-size:15px;color:#1e293b;line-height:1.7;letter-spacing:0.01em;text-align:{{align}};{{indent}}`
	p.Tags["ul"] = `display:block;margin:0 0 1em;padding:12px 12px 12px 1.6em;list-style-type:square;color:#0f172a;background:#f8fafc;border-radius:10px;border:1px solid #e2e8f0;`
	p.Tags["ol"] = `display:block;margin:0 0 1em;padding:12px 12px 12px 1.6em;list-style-type:decimal;color:#0f172a;background:#f8fafc;border-radius:10px;border:1px solid #e2e8f0;`
	p.Tags["li"] = `display:list-item;margin:0.4em 0;line-height:1.7;`
	p.Tags["blockquote"] = `display:block;margin:1em 0;padding:12px 14px;color:#334155;background:#eff6ff;border:none;border-left:5px solid {{primary}};font-size:14.5px;line-height:1.7;border-radius:0 12px 12px 0;`
	p.Tags["code"] = `font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:13px;color:#db2777;background:#f1f5f9;padding:2px 7px;border-radius:999px;border:1px solid #e2e8f0;`
	p.Tags["pre"] = `display:block;margin:1.1em 0;padding:16px 18px;background:#0b1220;border-radius:12px;border:1px solid #1e293b;overflow-x:auto;line-height:1.55;font-size:13px;color:#e2e8f0;white-space:pre;box-shadow:inset 0 1px 0 rgba(255,255,255,.04);`
	p.Tags["hr"] = `display:block;border:none;border-top:1px dashed #94a3b8;margin:1.8em 0;height:1px;`
	p.Tags["table"] = `display:table;border-collapse:separate;border-spacing:0;width:100%;margin:1.1em 0;font-size:13.5px;border:1px solid #cbd5e1;border-radius:10px;overflow:hidden;`
	p.Tags["th"] = `border:none;border-bottom:1px solid #cbd5e1;padding:10px 12px;background:#1e293b;font-weight:700;text-align:left;color:#f8fafc;`
	p.Tags["td"] = `border:none;border-bottom:1px solid #e2e8f0;padding:10px 12px;color:#334155;background:#fff;`
	p.Tags["img"] = `display:block;max-width:100%;height:auto;margin:1.1em auto;border-radius:12px;border:1px solid #e2e8f0;`
	p.PreCode = `display:block;font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:13px;color:#e2e8f0;background:transparent;padding:0;margin:0;border-radius:0;white-space:pre;word-break:normal;word-wrap:normal;`
	return p
}

// ---------- 暖色：圆角卡片章节标题 + 温暖色调 ----------
func warmPack(primary string) stylePack {
	p := simplePack(primary)
	p.Base = `margin:0 auto;padding:6px 2px;max-width:677px;font-size:16px;color:#44403c;line-height:1.75;letter-spacing:0.04em;word-wrap:break-word;font-family:PingFang SC,Hiragino Sans GB,Microsoft YaHei,serif;text-align:justify;`
	p.Tags["h1"] = fmt.Sprintf(
		`<section style="margin:32px 0 24px;padding:20px 24px;background:linear-gradient(135deg,#fff7ed,#fef3c7);border-radius:20px;border:1px solid #fde68a;">`+
			`<p style="margin:0 0 4px;font-size:10px;font-weight:700;color:%s;letter-spacing:2.5px;text-transform:uppercase;">%s {{num}}</p>`+
			`<strong style="display:block;font-size:26px;font-weight:800;color:#78350f;line-height:1.35;letter-spacing:-0.01em;">{{content}}</strong>`+
			`</section>`,
		"{{primary}}", "CHAPTER",
	)
	p.Tags["h2"] = fmt.Sprintf(
		`<section style="display:flex;align-items:center;margin:1.5em 0 0.75em;gap:10px;">`+
			`<section style="flex-shrink:0;width:28px;height:28px;background:%s;border-radius:8px;line-height:28px;text-align:center;"><span style="font-size:12px;color:#fff;">{{num}}</span></section>`+
			`<strong style="display:block;font-size:18px;font-weight:800;color:#92400e;line-height:1.4;">{{content}}</strong>`+
			`</section>`,
		"{{primary}}",
	)
	p.Tags["h3"] = `display:inline-block;font-size:16px;font-weight:700;color:#b45309;line-height:1.4;margin:1.2em 0 0.55em;padding:4px 12px;background:#ffedd5;border-radius:999px;`
	p.Tags["p"] = `display:block;margin:0 0 1em;font-size:16px;color:#44403c;line-height:1.75;letter-spacing:0.04em;text-align:{{align}};{{indent}}`
	p.Tags["blockquote"] = `display:block;margin:1.1em 0;padding:14px 16px;color:#78716c;background:#fff7ed;border:1px solid #fed7aa;border-left:6px solid {{primary}};font-size:15px;line-height:1.8;border-radius:14px;`
	p.Tags["ul"] = `display:block;margin:0 0 1em;padding:0.2em 0 0.2em 1.3em;list-style-type:circle;color:#44403c;`
	p.Tags["ol"] = `display:block;margin:0 0 1em;padding:0.2em 0 0.2em 1.3em;list-style-type:decimal;color:#44403c;`
	p.Tags["li"] = `display:list-item;margin:0.45em 0;line-height:1.8;padding-left:0.15em;`
	p.Tags["code"] = `font-family:Consolas,Monaco,Courier New,monospace;font-size:13.5px;color:#b45309;background:#fef3c7;padding:2px 7px;border-radius:8px;`
	p.Tags["pre"] = `display:block;margin:1.1em 0;padding:14px 16px;background:#fffbeb;border-radius:14px;border:1px dashed #fcd34d;overflow-x:auto;line-height:1.55;font-size:13px;white-space:pre;`
	p.Tags["hr"] = `display:block;border:none;border-top:2px dotted {{primary}};margin:2em auto;width:70%;height:0;opacity:0.7;`
	p.Tags["img"] = `display:block;max-width:100%;height:auto;margin:1.2em auto;border-radius:16px;box-shadow:0 10px 28px rgba(180,83,9,.15);`
	p.Tags["table"] = `display:table;border-collapse:separate;border-spacing:0;width:100%;margin:1.1em 0;font-size:14px;border:1px solid #fed7aa;border-radius:12px;overflow:hidden;`
	p.Tags["th"] = `border:none;padding:10px 12px;background:#fdba74;font-weight:700;text-align:left;color:#7c2d12;`
	p.Tags["td"] = `border:none;border-top:1px solid #fed7aa;padding:10px 12px;color:#44403c;background:#fffbeb;`
	return p
}

func htmlEscape(s string) string {
	r := strings.NewReplacer(`&`, "&amp;", `<`, "&lt;", `>`, "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

func injectTOC(styled string, cfg Config) string {
	headings := extractHeadings(styled)
	if len(headings) == 0 {
		styled = strings.ReplaceAll(styled, "<!--TOC-->", "")
		return styled
	}
	var b strings.Builder
	b.WriteString(`<section style="margin:0 0 1.4em;padding:14px 16px;background:#fafafa;border-radius:8px;border:1px solid #eee;">`)
	b.WriteString(`<p style="margin:0 0 0.6em;font-size:15px;font-weight:bold;color:#333;text-indent:0;">本文目录</p>`)
	for i, h := range headings {
		pad := ""
		if h.Level >= 3 {
			pad = "padding-left:1em;"
		}
		b.WriteString(fmt.Sprintf(
			`<p style="margin:0.25em 0;font-size:14px;color:#576b95;line-height:1.6;text-indent:0;%s">%d. %s</p>`,
			pad, i+1, htmlEscape(h.Text),
		))
	}
	b.WriteString(`</section>`)
	toc := b.String()
	if strings.Contains(styled, "<!--TOC-->") {
		return strings.Replace(styled, "<!--TOC-->", toc, 1)
	}
	if idx := strings.Index(styled, ">"); idx > 0 {
		return styled[:idx+1] + toc + styled[idx+1:]
	}
	return toc + styled
}

type heading struct {
	Level int
	Text  string
}

func extractHeadings(htmlStr string) []heading {
	return extractHeadingsOrdered(htmlStr)
}

func extractHeadingsOrdered(htmlStr string) []heading {
	var out []heading
	rest := htmlStr
	for {
		i2 := strings.Index(rest, "<h2")
		i3 := strings.Index(rest, "<h3")
		i := -1
		level := 2
		if i2 < 0 && i3 < 0 {
			break
		}
		if i2 >= 0 && (i3 < 0 || i2 < i3) {
			i, level = i2, 2
		} else {
			i, level = i3, 3
		}
		rest = rest[i:]
		tag := fmt.Sprintf("h%d", level)
		gt := strings.Index(rest, ">")
		if gt < 0 {
			break
		}
		closeTag := "</" + tag + ">"
		end := strings.Index(rest, closeTag)
		if end < 0 {
			break
		}
		text := strings.TrimSpace(stripTags(rest[gt+1 : end]))
		if text != "" {
			out = append(out, heading{Level: level, Text: text})
		}
		rest = rest[end+len(closeTag):]
	}
	return out
}

func stripTags(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func appendFooter(styled string, cfg Config) string {
	footer := fmt.Sprintf(
		`<section style="margin:2em 0 0;padding-top:1.2em;border-top:1px dashed #e5e5e5;">`+
			`<p style="margin:0;font-size:13px;color:#999;text-align:center;line-height:1.7;text-indent:0;">— END —</p>`+
			`<p style="margin:0.6em 0 0;font-size:13px;color:#999;text-align:center;line-height:1.7;text-indent:0;">觉得有用？点个<strong style="color:%s;">在看</strong>或转发吧</p>`+
			`</section>`,
		cfg.PrimaryColor,
	)
	if strings.Contains(styled, "<!--FOOTER-->") {
		return strings.Replace(styled, "<!--FOOTER-->", footer, 1)
	}
	if strings.HasSuffix(styled, "</section>") {
		return strings.TrimSuffix(styled, "</section>") + footer + "</section>"
	}
	return styled + footer
}
