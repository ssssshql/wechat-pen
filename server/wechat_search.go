package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// --- Search Biz ---

type searchBizItem struct {
	FakeID       string `json:"fakeid"`
	Nickname     string `json:"nickname"`
	Alias        string `json:"alias"`
	RoundHeadImg string `json:"round_head_img"`
	ServiceType  int    `json:"service_type"`
	Signature    string `json:"signature"`
	VerifyStatus int    `json:"verify_status"`
}

type searchBizResp struct {
	BaseResp struct {
		Ret    int    `json:"ret"`
		ErrMsg string `json:"err_msg"`
	} `json:"base_resp"`
	List  []searchBizItem `json:"list"`
	Total int             `json:"total"`
}

func handleSearchBiz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query is required"})
		return
	}

	begin := r.URL.Query().Get("begin")
	if begin == "" {
		begin = "0"
	}
	count := r.URL.Query().Get("count")
	if count == "" {
		count = "10"
	}

	body, err := mpGet("searchbiz", map[string]string{
		"action": "search_biz",
		"begin":  begin,
		"count":  count,
		"query":  query,
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	var result searchBizResp
	if err := json.Unmarshal(body, &result); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "parse failed", "body": string(body)})
		return
	}
	if result.BaseResp.Ret != 0 {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": result.BaseResp.ErrMsg, "errcode": result.BaseResp.Ret})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"list": result.List, "total": result.Total})
}

// --- Published Articles ---

type appmsgExItem struct {
	AID        string `json:"aid"`
	Title      string `json:"title"`
	Cover      string `json:"cover"`
	Link       string `json:"link"`
	Digest     string `json:"digest"`
	UpdateTime int64  `json:"update_time"`
	CreateTime int64  `json:"create_time"`
	AppmsgID   int64  `json:"appmsgid"`
}

type publishListItem struct {
	PublishType  int             `json:"publish_type"`
	PublishInfo  json.RawMessage `json:"publish_info"`
}

type parsedPublishInfo struct {
	AppmsgEx []appmsgExItem `json:"appmsgex"`
}

func handleBizArticles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fakeid := strings.TrimSpace(r.URL.Query().Get("fakeid"))
	if fakeid == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "fakeid is required"})
		return
	}

	begin := r.URL.Query().Get("begin")
	if begin == "" {
		begin = "0"
	}
	count := r.URL.Query().Get("count")
	if count == "" {
		count = "10"
	}

	body, err := mpGet("appmsgpublish", map[string]string{
		"sub":               "list",
		"search_field":      "null",
		"begin":             begin,
		"count":             count,
		"query":             "",
		"fakeid":            fakeid,
		"type":              "101_1",
		"free_publish_type": "1",
		"sub_action":        "list_ex",
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	// The response has publish_page as a JSON string
	var raw struct {
		BaseResp struct {
			Ret    int    `json:"ret"`
			ErrMsg string `json:"err_msg"`
		} `json:"base_resp"`
		PublishPage json.RawMessage `json:"publish_page"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "parse failed", "body": truncateStr(string(body), 500)})
		return
	}
	if raw.BaseResp.Ret != 0 {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": raw.BaseResp.ErrMsg, "errcode": raw.BaseResp.Ret})
		return
	}

	// publish_page is a JSON string, parse it
	var pageData struct {
		TotalCount  int                `json:"total_count"`
		PublishList []publishListItem  `json:"publish_list"`
	}
	if err := json.Unmarshal(raw.PublishPage, &pageData); err != nil {
		// It might be a double-encoded string
		var pageStr string
		if err2 := json.Unmarshal(raw.PublishPage, &pageStr); err2 == nil {
			json.Unmarshal([]byte(pageStr), &pageData)
		}
	}

	// Extract articles from publish_list
	type Article struct {
		Title      string `json:"title"`
		Link       string `json:"link"`
		Cover      string `json:"cover"`
		Digest     string `json:"digest"`
		CreateTime int64  `json:"create_time"`
		UpdateTime int64  `json:"update_time"`
		AppmsgID   int64  `json:"appmsg_id"`
	}

	var articles []Article
	for _, item := range pageData.PublishList {
		var info parsedPublishInfo
		// publish_info is also a JSON string
		var infoStr string
		if err := json.Unmarshal(item.PublishInfo, &infoStr); err != nil {
			// try direct
			json.Unmarshal(item.PublishInfo, &info)
		} else {
			json.Unmarshal([]byte(infoStr), &info)
		}
		for _, ex := range info.AppmsgEx {
			articles = append(articles, Article{
				Title:      ex.Title,
				Link:       ex.Link,
				Cover:      ex.Cover,
				Digest:     ex.Digest,
				CreateTime: ex.CreateTime,
				UpdateTime: ex.UpdateTime,
				AppmsgID:   ex.AppmsgID,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total":    pageData.TotalCount,
		"articles": articles,
	})
}

// --- Shared helper ---

func mpGet(endpoint string, params map[string]string) ([]byte, error) {
	cookie := GetLoginCookie()
	if cookie == "" {
		return nil, fmt.Errorf("未登录，请先扫码登录")
	}

	currentCredsMu.RLock()
	token := currentCreds.Token
	fingerprint := currentCreds.Fingerprint
	currentCredsMu.RUnlock()

	if token == "" || fingerprint == "" {
		return nil, fmt.Errorf("token/fingerprint 未获取，请重新登录")
	}

	params["fingerprint"] = fingerprint
	params["token"] = token
	params["lang"] = "zh_CN"
	params["f"] = "json"
	params["ajax"] = "1"

	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}

	apiURL := fmt.Sprintf("https://mp.weixin.qq.com/cgi-bin/%s?%s", endpoint, q.Encode())

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", cookie)
	req.Header.Set("Referer", "https://mp.weixin.qq.com/cgi-bin/home?t=home/index&lang=zh_CN&token="+token)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept", "*/*")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求微信接口失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("微信接口返回 %d: %s", resp.StatusCode, truncateStr(string(body), 200))
	}

	return body, nil
}

// --- Image Proxy (hotlink bypass) ---

var (
	imgTagRe  = regexp.MustCompile(`(?i)<img\b[^>]*>`)
	dataSrcRe = regexp.MustCompile(`data-src="([^"]*)"`)
	srcRe     = regexp.MustCompile(`\ssrc="([^"]*)"`)
)

// rewriteDataSrc replaces data-src with src when src is empty/missing (WeChat lazy-loading JS won't run in iframe).
// Must run BEFORE proxyRewrite so the rewrite regex sees src=, not data-src=.
func rewriteDataSrc(html string) string {
	return imgTagRe.ReplaceAllStringFunc(html, func(tag string) string {
		srcMatch := srcRe.FindStringSubmatch(tag)
		dataSrcMatch := dataSrcRe.FindStringSubmatch(tag)
		hasSrc := srcMatch != nil && srcMatch[1] != "" && !strings.Contains(srcMatch[1], "data:image")
		if !hasSrc && dataSrcMatch != nil && dataSrcMatch[1] != "" {
			if srcMatch == nil {
				return strings.Replace(tag, "data-src=", "src=", 1)
			}
			return srcRe.ReplaceAllString(tag, ` src="`+dataSrcMatch[1]+`"`)
		}
		return tag
	})
}

// proxyRewrite rewrites ALL mmbiz.qpic.cn URLs to go through /api/biz/image/proxy.
// Handles src="...", data-src="...", url(...), and bare URLs.
var fullMmbizURLRe = regexp.MustCompile(`(?:https?:)?//mmbiz\.qpic\.cn/[^\s"'<>\)]+`)

func proxyRewrite(html string) string {
	return fullMmbizURLRe.ReplaceAllStringFunc(html, func(raw string) string {
		if strings.HasPrefix(raw, "//") {
			raw = "https:" + raw
		}
		if strings.HasPrefix(raw, "http://") {
			raw = "https" + raw[4:]
		}
		raw = strings.ReplaceAll(raw, "&amp;", "&")
		return `/api/biz/image/proxy?url=` + url.QueryEscape(raw)
	})
}

func handleImageProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rawURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if rawURL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}

	// Normalize: ensure https://
	if strings.HasPrefix(rawURL, "http://") {
		rawURL = "https" + rawURL[4:]
	}
	if strings.HasPrefix(rawURL, "//") {
		rawURL = "https:" + rawURL
	}

	// Only allow known WeChat / QQ image domains
	allowedPrefixes := []string{
		"https://mmbiz.qpic.cn/",
		"https://thirdwx.qlogo.cn/",
		"https://wx.qlogo.cn/",
	}
	allowed := false
	for _, p := range allowedPrefixes {
		if strings.HasPrefix(rawURL, p) {
			allowed = true
			break
		}
	}
	if !allowed {
		http.Error(w, "domain not allowed", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Referer", "https://mp.weixin.qq.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "fetch failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "image/jpeg"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(body)
}

// --- Article Proxy (for iframe) ---

func handleArticleProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rawURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if rawURL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}

	// Only allow mp.weixin.qq.com
	if !strings.HasPrefix(rawURL, "https://mp.weixin.qq.com/") {
		http.Error(w, "only mp.weixin.qq.com URLs are allowed", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 14; Pixel 8 Pro Build/UD1A.231105.004) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Mobile Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "fetch failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	html := string(body)

	// Rewrite mmbiz.qpic.cn image URLs to go through our image proxy
	// Order matters: data-src→src FIRST, then rewrite mmbiz URLs to proxy
	html = rewriteDataSrc(html)
	html = proxyRewrite(html)

	// Remove X-Frame-Options meta tag if present
	html = strings.ReplaceAll(html, `<meta http-equiv="X-Frame-Options"`, `<meta http-equiv="disabled-X-Frame-Options"`)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=300")
	// Strip X-Frame-Options so the browser allows iframe embedding
	w.Header().Del("X-Frame-Options")
	w.Header().Del("Content-Security-Policy")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write([]byte(html))
}
