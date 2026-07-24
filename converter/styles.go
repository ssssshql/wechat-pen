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
	// Prefer pack-specific paragraph template when present.
	if tpl, ok := p.Tags["p"]; ok && strings.TrimSpace(tpl) != "" {
		return applyParagraphTemplate(tpl, cfg)
	}
	return applyParagraphTemplate(
		`display:block;margin:0 0 {{gap}};font-size:{{fontSize}};color:#333;line-height:{{lineHeight}};letter-spacing:0.034em;text-align:{{align}};{{indent}}`,
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
	// External theme pack from themes/*.json takes precedence when style id matches.
	if pack, ok := GetExternalTheme(string(cfg.Style)); ok {
		return packFromExternal(cfg, pack)
	}
	return builtinPack(cfg)
}

// ---------- 简约：干净留白 + 优雅排版 ----------
func simplePack(primary string) stylePack {
	return stylePack{
		Base:    `margin:0 auto;padding:0 4px;max-width:677px;font-size:{{fontSize}};color:#2c2c2c;line-height:{{lineHeight}};letter-spacing:0.025em;word-wrap:break-word;font-family:-apple-system,BlinkMacSystemFont,"Helvetica Neue","PingFang SC","Hiragino Sans GB","Microsoft YaHei UI","Microsoft YaHei",Arial,sans-serif;text-align:justify;`,
		PreCode: `display:block;font-family:"JetBrains Mono","Fira Code",Consolas,Monaco,"Courier New",monospace;font-size:13.5px;color:#2c2c2c;background:transparent;padding:0;margin:0;border-radius:0;white-space:pre;word-break:normal;word-wrap:normal;`,
		Tags: map[string]string{
			"h1":         `display:block;font-size:22px;font-weight:700;color:#1a1a1a;line-height:1.4;margin:1.4em 0 0.7em;text-align:center;letter-spacing:0.06em;`,
			"h2":         `display:block;font-size:18px;font-weight:700;color:#1a1a1a;line-height:1.4;margin:1.5em 0 0.65em;padding-bottom:0.35em;border-bottom:2px solid {{primary}};letter-spacing:0.04em;`,
			"h3":         `display:block;font-size:17px;font-weight:600;color:#1a1a1a;line-height:1.4;margin:1.3em 0 0.55em;`,
			"h4":         `display:block;font-size:16px;font-weight:600;color:#333;line-height:1.4;margin:1.2em 0 0.5em;`,
			"h5":         `display:block;font-size:15px;font-weight:600;color:#555;line-height:1.4;margin:1.1em 0 0.5em;`,
			"h6":         `display:block;font-size:15px;font-weight:600;color:#888;line-height:1.4;margin:1em 0 0.5em;`,
			"p":          `display:block;margin:0 0 {{gap}};font-size:{{fontSize}};color:#2c2c2c;line-height:{{lineHeight}};letter-spacing:0.025em;text-align:{{align}};{{indent}}`,
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

// ---------- 杂志：全居中衬线 + 装饰分割线 + 无边框引用 ----------
func magazinePack(primary string) stylePack {
	p := simplePack(primary)
	p.Base = `margin:0 auto;padding:8px 12px;max-width:640px;font-size:{{fontSize}};color:#2a2a2a;line-height:{{lineHeight}};letter-spacing:0.06em;word-wrap:break-word;font-family:Georgia,"Songti SC",SimSun,"Times New Roman",serif;text-align:justify;`
	// Full structural overrides
	p.Tags["h1"] = `display:block;font-size:26px;font-weight:700;color:#111;line-height:1.3;margin:1.8em 0 0.4em;text-align:center;letter-spacing:0.18em;`
	p.Tags["h2"] = `display:block;font-size:18px;font-weight:700;color:#111;line-height:1.35;margin:2em 0 0.9em;text-align:center;letter-spacing:0.12em;`
	p.Tags["h3"] = `display:block;font-size:15px;font-weight:700;color:{{primary}};line-height:1.4;margin:1.6em 0 0.7em;text-align:center;letter-spacing:0.2em;text-transform:uppercase;`
	p.Tags["p"] = `display:block;margin:0 0 {{gap}};font-size:{{fontSize}};color:#2a2a2a;line-height:{{lineHeight}};letter-spacing:0.05em;text-align:{{align}};{{indent}}`
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

// ---------- 科技：左对齐骨架 + 暗底代码 + 胶囊标签感 ----------
func techPack(primary string) stylePack {
	p := simplePack(primary)
	p.Base = `margin:0 auto;padding:0 2px;max-width:700px;font-size:{{fontSize}};color:#0f172a;line-height:{{lineHeight}};letter-spacing:0.01em;word-wrap:break-word;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",PingFang SC,Microsoft YaHei,sans-serif;text-align:left;`
	p.Tags["h1"] = `display:block;font-size:24px;font-weight:800;color:#0f172a;line-height:1.25;margin:1.2em 0 0.7em;text-align:left;padding:0 0 0.45em;border-bottom:3px solid {{primary}};letter-spacing:-0.01em;`
	p.Tags["h2"] = `display:flex;align-items:center;font-size:18px;font-weight:800;color:#0f172a;line-height:1.3;margin:1.5em 0 0.7em;padding:8px 12px;background:linear-gradient(90deg,rgba(59,130,246,0.12),transparent);border-left:4px solid {{primary}};border-radius:0 8px 8px 0;`
	p.Tags["h3"] = `display:block;font-size:16px;font-weight:700;color:#1e293b;line-height:1.35;margin:1.2em 0 0.5em;padding-left:0;border-bottom:1px dashed #cbd5e1;padding-bottom:0.25em;`
	p.Tags["p"] = `display:block;margin:0 0 {{gap}};font-size:{{fontSize}};color:#1e293b;line-height:{{lineHeight}};letter-spacing:0.01em;text-align:left;{{indent}}`
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

// ---------- 暖色：卡片式段落感 + 虚线装饰 + 软阴影图片 ----------
func warmPack(primary string) stylePack {
	p := simplePack(primary)
	p.Base = `margin:0 auto;padding:6px 2px;max-width:677px;font-size:{{fontSize}};color:#44403c;line-height:{{lineHeight}};letter-spacing:0.04em;word-wrap:break-word;font-family:PingFang SC,Hiragino Sans GB,Microsoft YaHei,serif;text-align:justify;`
	p.Tags["h1"] = `display:block;font-size:23px;font-weight:800;color:#78350f;line-height:1.35;margin:1.4em 0 0.8em;text-align:center;padding:0.55em 0.4em;background:linear-gradient(180deg,#fff7ed,transparent);border-radius:16px;`
	p.Tags["h2"] = `display:block;font-size:18px;font-weight:800;color:#92400e;line-height:1.4;margin:1.5em 0 0.75em;text-align:center;padding-bottom:0.45em;border-bottom:2px dashed {{primary}};`
	p.Tags["h3"] = `display:inline-block;font-size:16px;font-weight:700;color:#b45309;line-height:1.4;margin:1.2em 0 0.55em;padding:4px 12px;background:#ffedd5;border-radius:999px;`
	p.Tags["p"] = `display:block;margin:0 0 {{gap}};font-size:{{fontSize}};color:#44403c;line-height:{{lineHeight}};letter-spacing:0.04em;text-align:{{align}};{{indent}}`
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

// ---------- 暗黑：整页深色阅读卡（注意公众号白底外的兼容） ----------
func darkPack(primary string) stylePack {
	p := simplePack(primary)
	// Use padded card so dark bg is visible as an article card
	p.Base = `margin:0 auto;padding:18px 16px 28px;max-width:677px;font-size:{{fontSize}};color:#e5e7eb;line-height:{{lineHeight}};letter-spacing:0.02em;word-wrap:break-word;font-family:-apple-system,BlinkMacSystemFont,Helvetica Neue,PingFang SC,Microsoft YaHei,sans-serif;text-align:left;background:#0b1220;border-radius:16px;`
	p.Tags["h1"] = `display:block;font-size:24px;font-weight:800;color:#f8fafc;line-height:1.3;margin:0.8em 0 0.8em;text-align:left;padding-bottom:0.45em;border-bottom:1px solid #1f2937;`
	p.Tags["h2"] = `display:block;font-size:18px;font-weight:800;color:#f8fafc;line-height:1.35;margin:1.4em 0 0.7em;padding:8px 0 8px 12px;border-left:3px solid {{primary}};background:linear-gradient(90deg,rgba(88,166,255,.12),transparent);`
	p.Tags["h3"] = `display:block;font-size:16px;font-weight:700;color:#cbd5e1;line-height:1.4;margin:1.2em 0 0.5em;`
	p.Tags["p"] = `display:block;margin:0 0 {{gap}};font-size:{{fontSize}};color:#d1d5db;line-height:{{lineHeight}};letter-spacing:0.02em;text-align:left;{{indent}}`
	p.Tags["strong"] = `font-weight:700;color:#f8fafc;background:linear-gradient(180deg,transparent 60%,rgba(88,166,255,.35) 60%);`
	p.Tags["b"] = `font-weight:700;color:#f8fafc;`
	p.Tags["a"] = `color:#58a6ff;text-decoration:none;border-bottom:1px solid rgba(88,166,255,.35);`
	p.Tags["ul"] = `display:block;margin:0 0 1em;padding:0 0 0 1.25em;list-style-type:disc;color:#d1d5db;`
	p.Tags["ol"] = `display:block;margin:0 0 1em;padding:0 0 0 1.25em;list-style-type:decimal;color:#d1d5db;`
	p.Tags["li"] = `display:list-item;margin:0.4em 0;line-height:1.75;`
	p.Tags["blockquote"] = `display:block;margin:1em 0;padding:12px 14px;color:#94a3b8;background:#111827;border:1px solid #1f2937;border-left:4px solid {{primary}};font-size:14.5px;line-height:1.7;border-radius:0 10px 10px 0;`
	p.Tags["code"] = `font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:13px;color:#f97583;background:#1f2937;padding:2px 6px;border-radius:6px;`
	p.Tags["pre"] = `display:block;margin:1em 0;padding:14px 16px;background:#020617;border-radius:12px;border:1px solid #1f2937;overflow-x:auto;line-height:1.55;font-size:13px;color:#e5e7eb;white-space:pre;`
	p.Tags["hr"] = `display:block;border:none;border-top:1px solid #1f2937;margin:1.8em 0;height:1px;`
	p.Tags["table"] = `display:table;border-collapse:collapse;width:100%;margin:1em 0;font-size:13.5px;`
	p.Tags["th"] = `border:1px solid #1f2937;padding:9px 11px;background:#111827;font-weight:700;text-align:left;color:#f8fafc;`
	p.Tags["td"] = `border:1px solid #1f2937;padding:9px 11px;color:#d1d5db;`
	p.Tags["img"] = `display:block;max-width:100%;height:auto;margin:1.1em auto;border-radius:10px;border:1px solid #1f2937;opacity:0.95;`
	p.Tags["figcaption"] = `display:block;margin-top:0.5em;font-size:12px;color:#64748b;line-height:1.5;text-align:center;`
	p.PreCode = `display:block;font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:13px;color:#e5e7eb;background:transparent;padding:0;margin:0;border-radius:0;white-space:pre;word-break:normal;word-wrap:normal;`
	return p
}

// ---------- 清新：数字角标式 h2 + 浅底清单 ----------
func freshPack(primary string) stylePack {
	p := simplePack(primary)
	p.Base = `margin:0 auto;padding:0 2px;max-width:677px;font-size:{{fontSize}};color:#1f2937;line-height:{{lineHeight}};letter-spacing:0.02em;word-wrap:break-word;font-family:-apple-system,BlinkMacSystemFont,Helvetica Neue,PingFang SC,sans-serif;text-align:justify;`
	p.Tags["h1"] = `display:block;font-size:24px;font-weight:800;color:#065f46;line-height:1.3;margin:1.3em 0 0.7em;text-align:center;`
	p.Tags["h2"] = `display:block;font-size:17px;font-weight:800;color:#fff;line-height:1.35;margin:1.5em 0 0.8em;padding:10px 14px;background:{{primary}};border-radius:12px;text-align:left;box-shadow:0 8px 18px rgba(5,150,105,.18);`
	p.Tags["h3"] = `display:block;font-size:16px;font-weight:700;color:#047857;line-height:1.4;margin:1.2em 0 0.5em;padding-left:10px;border-left:3px solid #6ee7b7;`
	p.Tags["p"] = `display:block;margin:0 0 {{gap}};font-size:{{fontSize}};color:#334155;line-height:{{lineHeight}};letter-spacing:0.02em;text-align:{{align}};{{indent}}`
	p.Tags["blockquote"] = `display:block;margin:1em 0;padding:0;color:#065f46;background:transparent;border:none;font-size:15px;line-height:1.8;text-align:center;font-style:italic;`
	p.Tags["ul"] = `display:block;margin:0 0 1em;padding:12px 14px 12px 1.5em;list-style-type:disc;color:#1f2937;background:#ecfdf5;border-radius:14px;`
	p.Tags["ol"] = `display:block;margin:0 0 1em;padding:12px 14px 12px 1.5em;list-style-type:decimal;color:#1f2937;background:#ecfdf5;border-radius:14px;`
	p.Tags["li"] = `display:list-item;margin:0.4em 0;line-height:1.75;`
	p.Tags["code"] = `font-family:Consolas,Monaco,Courier New,monospace;font-size:13.5px;color:#047857;background:#d1fae5;padding:2px 6px;border-radius:6px;`
	p.Tags["pre"] = `display:block;margin:1em 0;padding:14px 16px;background:#f0fdf4;border-radius:14px;border:1px solid #a7f3d0;overflow-x:auto;line-height:1.55;font-size:13px;white-space:pre;`
	p.Tags["hr"] = `display:block;border:none;margin:2em auto;height:8px;width:80px;background:linear-gradient(90deg,#6ee7b7,{{primary}},#6ee7b7);border-radius:999px;`
	p.Tags["img"] = `display:block;max-width:100%;height:auto;margin:1.1em auto;border-radius:18px;`
	p.Tags["table"] = `display:table;border-collapse:separate;border-spacing:0;width:100%;margin:1em 0;font-size:14px;overflow:hidden;border-radius:12px;border:1px solid #a7f3d0;`
	p.Tags["th"] = `border:none;padding:10px 12px;background:#059669;font-weight:700;text-align:left;color:#fff;`
	p.Tags["td"] = `border:none;border-top:1px solid #d1fae5;padding:10px 12px;color:#334155;background:#f0fdf4;`
	return p
}

// ---------- 粉色：对话气泡式引用 + 圆角软卡片 ----------
func pinkPack(primary string) stylePack {
	p := simplePack(primary)
	p.Base = `margin:0 auto;padding:4px 2px;max-width:677px;font-size:{{fontSize}};color:#4a3347;line-height:{{lineHeight}};letter-spacing:0.03em;word-wrap:break-word;font-family:PingFang SC,Hiragino Sans GB,Microsoft YaHei,sans-serif;text-align:justify;`
	p.Tags["h1"] = `display:block;font-size:24px;font-weight:800;color:#9d174d;line-height:1.3;margin:1.3em 0 0.6em;text-align:center;`
	p.Tags["h2"] = `display:block;font-size:18px;font-weight:800;color:#9d174d;line-height:1.35;margin:1.5em 0 0.8em;text-align:center;position:relative;`
	p.Tags["h3"] = `display:block;font-size:16px;font-weight:700;color:#be185d;line-height:1.4;margin:1.2em 0 0.5em;text-align:left;padding-bottom:0.2em;border-bottom:2px dotted #f9a8d4;`
	p.Tags["p"] = `display:block;margin:0 0 {{gap}};font-size:{{fontSize}};color:#4a3347;line-height:{{lineHeight}};letter-spacing:0.03em;text-align:{{align}};{{indent}}`
	p.Tags["strong"] = `font-weight:800;color:#9d174d;`
	p.Tags["b"] = `font-weight:800;color:#9d174d;`
	p.Tags["blockquote"] = `display:block;margin:1.1em 0 1.1em 0.4em;padding:14px 16px;color:#6b4c5e;background:#fdf2f8;border:none;font-size:15px;line-height:1.8;border-radius:18px 18px 18px 4px;box-shadow:0 6px 16px rgba(236,72,153,.10);`
	p.Tags["ul"] = `display:block;margin:0 0 1em;padding:0;list-style-type:none;color:#4a3347;`
	p.Tags["ol"] = `display:block;margin:0 0 1em;padding:0 0 0 1.2em;list-style-type:decimal;color:#4a3347;`
	p.Tags["li"] = `display:list-item;margin:0.45em 0;line-height:1.8;padding:8px 12px;background:#fdf2f8;border-radius:12px;`
	p.Tags["code"] = `font-family:Consolas,Monaco,Courier New,monospace;font-size:13.5px;color:#be185d;background:#fce7f3;padding:2px 7px;border-radius:999px;`
	p.Tags["pre"] = `display:block;margin:1em 0;padding:14px 16px;background:#fdf2f8;border-radius:16px;border:1px solid #fbcfe8;overflow-x:auto;line-height:1.55;font-size:13px;white-space:pre;`
	p.Tags["hr"] = `display:block;border:none;margin:2em auto;height:0;width:40%;border-top:1px solid #f9a8d4;`
	p.Tags["img"] = `display:block;max-width:100%;height:auto;margin:1.1em auto;border-radius:20px;box-shadow:0 10px 24px rgba(236,72,153,.12);`
	p.Tags["table"] = `display:table;border-collapse:separate;border-spacing:0;width:100%;margin:1em 0;font-size:14px;border-radius:14px;overflow:hidden;border:1px solid #fbcfe8;`
	p.Tags["th"] = `border:none;padding:10px 12px;background:#f472b6;font-weight:700;text-align:left;color:#fff;`
	p.Tags["td"] = `border:none;border-top:1px solid #fce7f3;padding:10px 12px;color:#4a3347;background:#fff;`
	return p
}

// ---------- 极简灰：等宽正文 + 无装饰线稿感 ----------
func monoPack(primary string) stylePack {
	_ = primary
	p := simplePack(primary)
	p.Base = `margin:0 auto;padding:0;max-width:640px;font-size:{{fontSize}};color:#111827;line-height:{{lineHeight}};letter-spacing:0;word-wrap:break-word;font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,"Cascadia Mono",monospace;text-align:left;`
	p.Tags["h1"] = `display:block;font-size:20px;font-weight:700;color:#111827;line-height:1.35;margin:1.8em 0 1em;text-align:left;text-transform:uppercase;letter-spacing:0.14em;border-bottom:1px solid #111827;padding-bottom:0.4em;`
	p.Tags["h2"] = `display:block;font-size:15px;font-weight:700;color:#111827;line-height:1.4;margin:1.8em 0 0.7em;text-align:left;letter-spacing:0.08em;`
	p.Tags["h3"] = `display:block;font-size:14px;font-weight:700;color:#374151;line-height:1.4;margin:1.3em 0 0.5em;text-align:left;`
	p.Tags["p"] = `display:block;margin:0 0 {{gap}};font-size:{{fontSize}};color:#1f2937;line-height:{{lineHeight}};letter-spacing:0;text-align:left;{{indent}}`
	p.Tags["blockquote"] = `display:block;margin:1.1em 0;padding:0 0 0 1em;color:#6b7280;background:transparent;border-left:2px solid #9ca3af;font-size:13.5px;line-height:1.75;font-style:normal;`
	p.Tags["ul"] = `display:block;margin:0 0 1em;padding:0 0 0 1.1em;list-style-type:"-  ";color:#111827;`
	p.Tags["ol"] = `display:block;margin:0 0 1em;padding:0 0 0 1.3em;list-style-type:decimal;color:#111827;`
	p.Tags["li"] = `display:list-item;margin:0.3em 0;line-height:1.7;`
	p.Tags["code"] = `font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:12.5px;color:#111827;background:#f3f4f6;padding:0 4px;border-radius:0;border-bottom:1px solid #d1d5db;`
	p.Tags["pre"] = `display:block;margin:1em 0;padding:12px 14px;background:#f9fafb;border-radius:0;border:1px solid #e5e7eb;overflow-x:auto;line-height:1.55;font-size:12.5px;white-space:pre;`
	p.Tags["hr"] = `display:block;border:none;border-top:1px solid #111827;margin:2em 0;height:1px;width:100%;`
	p.Tags["a"] = `color:#111827;text-decoration:underline;text-underline-offset:3px;`
	p.Tags["img"] = `display:block;max-width:100%;height:auto;margin:1.2em auto;border-radius:0;border:1px solid #e5e7eb;filter:grayscale(100%);`
	p.Tags["table"] = `display:table;border-collapse:collapse;width:100%;margin:1em 0;font-size:12.5px;`
	p.Tags["th"] = `border:1px solid #111827;padding:8px 10px;background:#fff;font-weight:700;text-align:left;color:#111827;`
	p.Tags["td"] = `border:1px solid #d1d5db;padding:8px 10px;color:#1f2937;`
	p.PreCode = `display:block;font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:12.5px;color:#111827;background:transparent;padding:0;margin:0;border-radius:0;white-space:pre;word-break:normal;word-wrap:normal;`
	return p
}

// ---------- 学术：论文体 + 居中标题编号感 + 表注风格 ----------
func academicPack(primary string) stylePack {
	p := simplePack(primary)
	p.Base = `margin:0 auto;padding:0 8px;max-width:680px;font-size:{{fontSize}};color:#1f2937;line-height:{{lineHeight}};letter-spacing:0.01em;word-wrap:break-word;font-family:"Times New Roman",Georgia,SimSun,"Songti SC",serif;text-align:justify;`
	p.Tags["h1"] = `display:block;font-size:20px;font-weight:700;color:#111827;line-height:1.4;margin:1.6em 0 1.1em;text-align:center;letter-spacing:0.04em;`
	p.Tags["h2"] = `display:block;font-size:16px;font-weight:700;color:#111827;line-height:1.4;margin:1.8em 0 0.8em;text-align:center;`
	p.Tags["h3"] = `display:block;font-size:15px;font-weight:700;color:#1f2937;line-height:1.4;margin:1.4em 0 0.55em;text-align:left;`
	p.Tags["p"] = `display:block;margin:0 0 {{gap}};font-size:{{fontSize}};color:#1f2937;line-height:{{lineHeight}};letter-spacing:0.01em;text-align:{{align}};{{indent}}`
	p.Tags["blockquote"] = `display:block;margin:1.2em 2.2em;padding:0;color:#4b5563;background:transparent;border:none;font-size:14px;line-height:1.85;font-style:italic;text-align:left;`
	p.Tags["ul"] = `display:block;margin:0.4em 0 1em 0.5em;padding-left:1.4em;list-style-type:disc;color:#1f2937;`
	p.Tags["ol"] = `display:block;margin:0.4em 0 1em 0.5em;padding-left:1.5em;list-style-type:decimal;color:#1f2937;`
	p.Tags["li"] = `display:list-item;margin:0.25em 0;line-height:1.8;`
	p.Tags["code"] = `font-family:Consolas,Monaco,Courier New,monospace;font-size:13px;color:#b91c1c;background:transparent;padding:0 2px;border-radius:0;border-bottom:1px dotted #fca5a5;`
	p.Tags["pre"] = `display:block;margin:1em 0;padding:12px 14px;background:#f9fafb;border-radius:0;border:1px solid #d1d5db;overflow-x:auto;line-height:1.5;font-size:12.5px;white-space:pre;`
	p.Tags["hr"] = `display:block;border:none;border-top:1px solid #9ca3af;margin:2em auto;width:30%;height:1px;`
	p.Tags["a"] = `color:#1d4ed8;text-decoration:none;border-bottom:1px solid #93c5fd;`
	p.Tags["table"] = `display:table;border-collapse:collapse;width:100%;margin:1.2em 0 0.4em;font-size:13px;`
	p.Tags["th"] = `border:1px solid #6b7280;padding:7px 10px;background:#f3f4f6;font-weight:700;text-align:center;color:#111827;`
	p.Tags["td"] = `border:1px solid #9ca3af;padding:7px 10px;color:#1f2937;text-align:center;`
	p.Tags["img"] = `display:block;max-width:92%;height:auto;margin:1.2em auto 0.3em;border-radius:0;`
	p.Tags["figcaption"] = `display:block;margin:0.3em 0 1em;font-size:12px;color:#6b7280;line-height:1.5;text-align:center;font-family:-apple-system,BlinkMacSystemFont,sans-serif;`
	p.PreCode = `display:block;font-family:Consolas,Monaco,Courier New,monospace;font-size:12.5px;color:#111827;background:transparent;padding:0;margin:0;border-radius:0;white-space:pre;word-break:normal;word-wrap:normal;`
	return p
}

// ---------- 中国风：竖排装饰感标题 + 双线分割 + 宣纸底 ----------
func chinPack(primary string) stylePack {
	p := simplePack(primary)
	p.Base = `margin:0 auto;padding:14px 16px 24px;max-width:660px;font-size:{{fontSize}};color:#3f3f3f;line-height:{{lineHeight}};letter-spacing:0.08em;word-wrap:break-word;font-family:"Songti SC",SimSun,STSong,"Noto Serif SC",serif;text-align:justify;background:#fffdf8;border:1px solid #f3e8d5;border-radius:4px;`
	p.Tags["h1"] = `display:block;font-size:24px;font-weight:700;color:#9a3412;line-height:1.4;margin:1.2em 0 0.9em;text-align:center;letter-spacing:0.28em;`
	p.Tags["h2"] = `display:block;font-size:18px;font-weight:700;color:#c2410c;line-height:1.4;margin:1.7em 0 0.9em;text-align:center;letter-spacing:0.18em;padding:0.3em 0;border-top:1px solid #e7c9a0;border-bottom:1px solid #e7c9a0;`
	p.Tags["h3"] = `display:block;font-size:16px;font-weight:700;color:#c2410c;line-height:1.4;margin:1.3em 0 0.55em;text-align:left;letter-spacing:0.12em;`
	p.Tags["p"] = `display:block;margin:0 0 {{gap}};font-size:{{fontSize}};color:#3f3f3f;line-height:{{lineHeight}};letter-spacing:0.08em;text-align:{{align}};{{indent}}`
	p.Tags["strong"] = `font-weight:700;color:#9a3412;`
	p.Tags["b"] = `font-weight:700;color:#9a3412;`
	p.Tags["blockquote"] = `display:block;margin:1.3em 1.2em;padding:0.9em 0.4em;color:#78350f;background:transparent;border:none;border-top:1px solid #e7c9a0;border-bottom:1px solid #e7c9a0;font-size:15px;line-height:1.9;text-align:center;`
	p.Tags["ul"] = `display:block;margin:0.5em 0 1em;padding-left:1.5em;list-style-type:square;color:#3f3f3f;`
	p.Tags["ol"] = `display:block;margin:0.5em 0 1em;padding-left:1.5em;list-style-type:cjk-ideographic;color:#3f3f3f;`
	p.Tags["li"] = `display:list-item;margin:0.4em 0;line-height:1.85;`
	p.Tags["a"] = `color:#c2410c;text-decoration:none;border-bottom:1px solid rgba(194,65,12,0.35);`
	p.Tags["code"] = `font-family:Consolas,Monaco,Courier New,monospace;font-size:13px;color:#c2410c;background:#fef3c7;padding:1px 5px;border-radius:2px;`
	p.Tags["pre"] = `display:block;margin:1.1em 0;padding:14px 16px;background:#fff7ed;border-radius:2px;border:1px solid #e7c9a0;overflow-x:auto;line-height:1.55;font-size:13px;white-space:pre;`
	p.Tags["hr"] = `display:block;border:none;margin:2em auto;height:0;width:36%;border-top:1px solid #c2410c;border-bottom:1px solid #c2410c;padding:2px 0;`
	p.Tags["img"] = `display:block;max-width:100%;height:auto;margin:1.2em auto;border-radius:2px;border:1px solid #e7c9a0;padding:4px;background:#fff;`
	p.Tags["table"] = `display:table;border-collapse:collapse;width:100%;margin:1.1em 0;font-size:13.5px;`
	p.Tags["th"] = `border:1px solid #d6b48a;padding:8px 11px;background:#fff7ed;font-weight:700;text-align:center;color:#9a3412;`
	p.Tags["td"] = `border:1px solid #e7c9a0;padding:8px 11px;color:#3f3f3f;text-align:center;`
	p.Tags["figcaption"] = `display:block;margin-top:0.45em;font-size:12px;color:#a16207;line-height:1.5;text-align:center;letter-spacing:0.12em;`
	p.PreCode = `display:block;font-family:Consolas,Monaco,Courier New,monospace;font-size:13px;color:#3f3f3f;background:transparent;padding:0;margin:0;border-radius:0;white-space:pre;word-break:normal;word-wrap:normal;`
	return p
}


func htmlEscape(s string) string {
	r := strings.NewReplacer(`&`, "&amp;", `<`, "&lt;", `>`, "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

func injectTOC(styled string, cfg Config) string {
	// Extract h2/h3 text from already-styled HTML naively
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
	// insert after opening section
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
	// before closing section
	if strings.HasSuffix(styled, "</section>") {
		return strings.TrimSuffix(styled, "</section>") + footer + "</section>"
	}
	return styled + footer
}
