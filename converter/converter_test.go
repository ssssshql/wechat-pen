package converter

import (
	"strings"
	"testing"
)

func TestConvertBasic(t *testing.T) {
	src := []byte("# Hello\n\nThis is **bold** and *italic*.\n")
	out, err := Convert(src, Config{})
	if err != nil {
		t.Fatal(err)
	}
	html := string(out)
	if !strings.Contains(html, "<h1") || !strings.Contains(html, "Hello") {
		t.Fatalf("missing heading, got:\n%s", html)
	}
	if !strings.Contains(html, "<strong>bold</strong>") {
		t.Fatalf("missing bold, got:\n%s", html)
	}
}

func TestConvertWeChatInlineStyles(t *testing.T) {
	src := []byte("# 标题\n\n段落 **加粗**。\n\n> 引用\n\n- 列表项\n")
	out, err := Convert(src, Config{Theme: ThemeWeChat, Style: StyleSimple, PrimaryColor: "#07c160"})
	if err != nil {
		t.Fatal(err)
	}
	html := string(out)
	if !strings.Contains(html, `style=`) {
		t.Fatalf("expected inline styles, got:\n%s", html)
	}
	if !strings.Contains(html, "<section") {
		t.Fatalf("expected outer section, got:\n%s", html)
	}
	if !strings.Contains(html, "font-size:48px") {
		t.Fatalf("expected h1 wechat style, got:\n%s", html)
	}
}

func TestConvertWeChatHighlight(t *testing.T) {
	src := []byte("```go\npackage main\nfunc main() {}\n```\n")
	out, err := Convert(src, Config{Theme: ThemeWeChat, Highlight: true})
	if err != nil {
		t.Fatal(err)
	}
	html := string(out)
	if !strings.Contains(html, "color:") {
		t.Fatalf("expected highlighted colors, got:\n%s", html)
	}
}

func TestConvertWeChatCodeSpacesPreserved(t *testing.T) {
	src := []byte("```go\npackage main\n\nfunc main() {\n    x := 1\n}\n```\n")
	out, err := Convert(src, Config{Theme: ThemeWeChat, Highlight: true, Style: StyleSimple})
	if err != nil {
		t.Fatal(err)
	}
	html := string(out)
	// Spaces must survive as NBSP (WeChat collapses normal spaces in spans).
	if !strings.Contains(html, " ") && !strings.Contains(html, "&#160;") && !strings.Contains(html, "&nbsp;") {
		// html.Render may emit raw NBSP rune
		if !strings.Contains(html, "package") || !strings.Contains(html, "main") {
			t.Fatalf("unexpected code html:\n%s", html)
		}
		// At least white-space:pre on pre
		if !strings.Contains(html, "white-space:pre") {
			t.Fatalf("expected white-space:pre and/or nbsp, got:\n%s", html)
		}
	}
	// Must not keep pink chip word-break on pre>code
	if strings.Contains(html, "<code style=\"font-family:Consolas,Monaco,Courier New,monospace;font-size:14px;color:#c7254e") {
		t.Fatalf("pre>code still has inline chip style:\n%s", html)
	}
}

func TestConvertWeChatTOC(t *testing.T) {
	src := []byte("## 一\n\ntext\n\n## 二\n\ntext\n")
	out, err := Convert(src, Config{Theme: ThemeWeChat, TOC: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "本文目录") {
		t.Fatalf("expected TOC, got:\n%s", out)
	}
}

func TestConvertWeChatTextIndent(t *testing.T) {
	src := []byte("hello paragraph\n")
	out, err := Convert(src, Config{Theme: ThemeWeChat, TextIndent: true, Justify: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "text-indent:2em") {
		t.Fatalf("expected text-indent, got:\n%s", out)
	}
}

func TestExpandComponents(t *testing.T) {
	src := []byte(":::divider\n\n:::profile{nickname=\"测试号\"}\n")
	out, err := Convert(src, Config{Theme: ThemeWeChat})
	if err != nil {
		t.Fatal(err)
	}
	html := string(out)
	if !strings.Contains(html, "· · ·") && !strings.Contains(html, "mp-common-profile") {
		t.Fatalf("expected divider or profile, got:\n%s", html)
	}
}

func TestStylePacks(t *testing.T) {
	src := []byte("## Hi\n\npara\n")
	for _, st := range []StylePack{StyleSimple, StyleMagazine, StyleTech, StyleWarm} {
		out, err := Convert(src, Config{Theme: ThemeWeChat, Style: st})
		if err != nil {
			t.Fatalf("%s: %v", st, err)
		}
		if !strings.Contains(string(out), "<section") {
			t.Fatalf("%s: missing section", st)
		}
	}
}
