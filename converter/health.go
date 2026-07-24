package converter

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	reMDImage   = regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)
	reHTMLImage = regexp.MustCompile(`(?i)<img[^>]+src=["']([^"']+)["']`)
	reLocalPath = regexp.MustCompile(`(?i)^(file:|/|[a-z]:\\|\\\\|\./|\.\./)`)
	reFenced    = regexp.MustCompile("(?s)```[a-zA-Z0-9_-]*\\n(.*?)```")
)

func analyzeHealth(md, htmlOut string, report CleanReport) HealthReport {
	var warns []HealthWarning

	// Dropped tags
	for _, tag := range report.DroppedTags {
		warns = append(warns, HealthWarning{
			Level:   "warn",
			Code:    "dropped_tag",
			Message: fmt.Sprintf("已剥离不支持标签 <%s>", tag),
		})
	}

	// Local / relative images
	for _, m := range reMDImage.FindAllStringSubmatch(md, -1) {
		src := strings.TrimSpace(m[1])
		if i := strings.IndexAny(src, " \t"); i > 0 {
			src = src[:i]
		}
		if isRiskyImage(src) {
			warns = append(warns, HealthWarning{
				Level:   "error",
				Code:    "local_image",
				Message: fmt.Sprintf("本地/相对路径图片无法在公众号显示：%s（请先上传素材库）", short(src, 80)),
			})
		}
	}
	for _, m := range reHTMLImage.FindAllStringSubmatch(htmlOut, -1) {
		src := strings.TrimSpace(m[1])
		if isRiskyImage(src) {
			warns = append(warns, HealthWarning{
				Level:   "error",
				Code:    "local_image",
				Message: fmt.Sprintf("HTML 中含本地图片：%s", short(src, 80)),
			})
		}
	}

	// Very long fenced code blocks
	for _, m := range reFenced.FindAllStringSubmatch(md, -1) {
		body := m[1]
		lines := strings.Count(body, "\n") + 1
		if lines > 40 {
			warns = append(warns, HealthWarning{
				Level:   "warn",
				Code:    "long_code",
				Message: fmt.Sprintf("代码块较长（约 %d 行），手机端阅读体验可能较差", lines),
			})
		}
		if utf8.RuneCountInString(body) > 3000 {
			warns = append(warns, HealthWarning{
				Level:   "info",
				Code:    "huge_code",
				Message: "代码块体积较大，建议拆分或放到外链",
			})
		}
	}

	// External non-https images
	for _, m := range reMDImage.FindAllStringSubmatch(md, -1) {
		src := strings.TrimSpace(m[1])
		if strings.HasPrefix(strings.ToLower(src), "http://") {
			warns = append(warns, HealthWarning{
				Level:   "warn",
				Code:    "http_image",
				Message: fmt.Sprintf("图片为 http 非 https：%s（部分环境可能拦截）", short(src, 80)),
			})
		}
	}

	// Mini program placeholders
	if strings.Contains(htmlOut, "weapp_text_link") {
		warns = append(warns, HealthWarning{
			Level:   "info",
			Code:    "miniprogram",
			Message: "含小程序链接占位，请在公众号编辑器核对 appid/path",
		})
	}
	if strings.Contains(htmlOut, "mp-common-profile") {
		warns = append(warns, HealthWarning{
			Level:   "info",
			Code:    "profile",
			Message: "含公众号名片占位，建议在编辑器替换为官方组件",
		})
	}

	// Empty content
	plain := strings.TrimSpace(stripTags(htmlOut))
	if plain == "" {
		warns = append(warns, HealthWarning{
			Level:   "warn",
			Code:    "empty",
			Message: "输出内容为空",
		})
	}

	ok := true
	for _, w := range warns {
		if w.Level == "error" {
			ok = false
			break
		}
	}
	return HealthReport{Warnings: warns, OK: ok}
}

func isRiskyImage(src string) bool {
	s := strings.TrimSpace(src)
	if s == "" {
		return false
	}
	low := strings.ToLower(s)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") || strings.HasPrefix(low, "data:") {
		return false
	}
	if reLocalPath.MatchString(s) {
		return true
	}
	// bare relative like images/a.png
	if !strings.Contains(s, "://") {
		base := path.Base(s)
		if strings.Contains(base, ".") {
			return true
		}
	}
	return false
}

func short(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
