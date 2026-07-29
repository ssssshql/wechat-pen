package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"golang.org/x/net/html"
)

// --- Writing Style ---

type WritingStyle struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	FakeID      string `json:"fakeid"`
	Nickname    string `json:"nickname"`
	StylePrompt string `json:"stylePrompt"`
	SampleCount int    `json:"sampleCount"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

// --- LLM helper ---

func getAILLM() (llms.Model, error) {
	currentCredsMu.RLock()
	baseURL := currentCreds.AIBaseURL
	apiKey := currentCreds.AIAPIKey
	model := currentCreds.AIModel
	currentCredsMu.RUnlock()

	if apiKey == "" {
		return nil, fmt.Errorf("AI API Key 未配置，请先在设置中配置")
	}
	if model == "" {
		model = "gpt-4o-mini"
	}

	opts := []openai.Option{
		openai.WithToken(apiKey),
		openai.WithModel(model),
	}
	if baseURL != "" {
		opts = append(opts, openai.WithBaseURL(baseURL))
	}

	return openai.New(opts...)
}

// --- Style CRUD ---

func listStyles() ([]WritingStyle, error) {
	if notesDB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	notesDB.mu.Lock()
	defer notesDB.mu.Unlock()
	rows, err := notesDB.db.Query(
		`SELECT id, name, fakeid, nickname, style_prompt, sample_count, created_at, updated_at FROM writing_styles ORDER BY updated_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WritingStyle
	for rows.Next() {
		var s WritingStyle
		if err := rows.Scan(&s.ID, &s.Name, &s.FakeID, &s.Nickname, &s.StylePrompt, &s.SampleCount, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if out == nil {
		out = []WritingStyle{}
	}
	return out, nil
}

func getStyle(id string) (WritingStyle, error) {
	if notesDB == nil {
		return WritingStyle{}, fmt.Errorf("数据库未初始化")
	}
	notesDB.mu.Lock()
	defer notesDB.mu.Unlock()
	var s WritingStyle
	err := notesDB.db.QueryRow(
		`SELECT id, name, fakeid, nickname, style_prompt, sample_count, created_at, updated_at FROM writing_styles WHERE id = ?`, id,
	).Scan(&s.ID, &s.Name, &s.FakeID, &s.Nickname, &s.StylePrompt, &s.SampleCount, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return WritingStyle{}, err
	}
	return s, nil
}

func saveStyle(s *WritingStyle) error {
	if notesDB == nil {
		return fmt.Errorf("数据库未初始化")
	}
	notesDB.mu.Lock()
	defer notesDB.mu.Unlock()
	if s.ID == "" {
		s.ID = fmt.Sprintf("ws_%d_%s", time.Now().UnixMilli(), randSuffix())
	}
	now := time.Now().UnixMilli()
	if s.CreatedAt == 0 {
		s.CreatedAt = now
	}
	s.UpdatedAt = now
	_, err := notesDB.db.Exec(
		`INSERT INTO writing_styles (id, name, fakeid, nickname, style_prompt, sample_count, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET name=excluded.name, fakeid=excluded.fakeid, nickname=excluded.nickname, style_prompt=excluded.style_prompt, sample_count=excluded.sample_count, updated_at=excluded.updated_at`,
		s.ID, s.Name, s.FakeID, s.Nickname, s.StylePrompt, s.SampleCount, s.CreatedAt, s.UpdatedAt,
	)
	return err
}

func deleteStyleByID(id string) error {
	if notesDB == nil {
		return fmt.Errorf("数据库未初始化")
	}
	notesDB.mu.Lock()
	defer notesDB.mu.Unlock()
	_, err := notesDB.db.Exec(`DELETE FROM writing_styles WHERE id = ?`, id)
	return err
}

// --- HTML text extraction ---

func extractTextFromHTML(htmlStr string) string {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return ""
	}
	var sb strings.Builder
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		// Skip script and style content
		if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style") {
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return strings.TrimSpace(sb.String())
}

func truncateRunes(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes])
}

// --- HTTP Handlers ---

// GET/POST /api/ai/config
func handleAIConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		currentCredsMu.RLock()
		defer currentCredsMu.RUnlock()
		writeJSON(w, http.StatusOK, map[string]any{
			"ai_base_url":       currentCreds.AIBaseURL,
			"ai_model":          currentCreds.AIModel,
			"has_ai_key":        currentCreds.AIAPIKey != "",
			"ai_image_base_url": currentCreds.AIImageBaseURL,
			"ai_image_model":    currentCreds.AIImageModel,
			"has_ai_image_key":  currentCreds.AIImageAPIKey != "",
		})
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
		defer r.Body.Close()
		var req struct {
			AIBaseURL      *string `json:"ai_base_url"`
			AIAPIKey       *string `json:"ai_api_key"`
			AIModel        *string `json:"ai_model"`
			AIImageBaseURL *string `json:"ai_image_base_url"`
			AIImageAPIKey  *string `json:"ai_image_api_key"`
			AIImageModel   *string `json:"ai_image_model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		currentCredsMu.Lock()
		if req.AIBaseURL != nil {
			currentCreds.AIBaseURL = strings.TrimRight(strings.TrimSpace(*req.AIBaseURL), "/")
		}
		if req.AIAPIKey != nil {
			currentCreds.AIAPIKey = strings.TrimSpace(*req.AIAPIKey)
		}
		if req.AIModel != nil {
			currentCreds.AIModel = strings.TrimSpace(*req.AIModel)
		}
		if req.AIImageBaseURL != nil {
			currentCreds.AIImageBaseURL = strings.TrimRight(strings.TrimSpace(*req.AIImageBaseURL), "/")
		}
		if req.AIImageAPIKey != nil {
			currentCreds.AIImageAPIKey = strings.TrimSpace(*req.AIImageAPIKey)
		}
		if req.AIImageModel != nil {
			currentCreds.AIImageModel = strings.TrimSpace(*req.AIImageModel)
		}
		currentCredsMu.Unlock()
		cfg := snapshotConfig()
		if err := saveConfigFile(cfg); err != nil {
			fmt.Printf("warn: save config: %v\n", err)
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// POST /api/ai/analyze-style
func handleAnalyzeStyle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	defer r.Body.Close()
	var req struct {
		FakeID   string `json:"fakeid"`
		Nickname string `json:"nickname"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.FakeID == "" || req.Nickname == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "fakeid and nickname are required"})
		return
	}

	// Fetch 3 recent articles
	body, err := mpGet("appmsgpublish", map[string]string{
		"sub":               "list",
		"search_field":      "null",
		"begin":             "0",
		"count":             "3",
		"query":             "",
		"fakeid":            req.FakeID,
		"type":              "101_1",
		"free_publish_type": "1",
		"sub_action":        "list_ex",
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "获取文章列表失败: " + err.Error()})
		return
	}

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

	var pageData struct {
		TotalCount  int                `json:"total_count"`
		PublishList []publishListItem  `json:"publish_list"`
	}
	if err := json.Unmarshal(raw.PublishPage, &pageData); err != nil {
		var pageStr string
		if err2 := json.Unmarshal(raw.PublishPage, &pageStr); err2 == nil {
			json.Unmarshal([]byte(pageStr), &pageData)
		}
	}

	type articleData struct {
		Title string
		Link  string
		Text  string
	}
	var articles []articleData
	for _, item := range pageData.PublishList {
		var info parsedPublishInfo
		var infoStr string
		if err := json.Unmarshal(item.PublishInfo, &infoStr); err != nil {
			json.Unmarshal(item.PublishInfo, &info)
		} else {
			json.Unmarshal([]byte(infoStr), &info)
		}
		for _, ex := range info.AppmsgEx {
			articles = append(articles, articleData{Title: ex.Title, Link: ex.Link})
		}
	}

	if len(articles) == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "该公众号暂无已发布文章"})
		return
	}

	// Fetch each article's HTML and extract text
	cookie := GetLoginCookie()
	for i := range articles {
		if articles[i].Link == "" {
			continue
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		req2, err := http.NewRequestWithContext(ctx, "GET", articles[i].Link, nil)
		if err != nil {
			cancel()
			fmt.Printf("analyze-style: 创建请求失败 [%s]: %v\n", articles[i].Link, err)
			continue
		}
		req2.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 14; Pixel 8 Pro) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Mobile Safari/537.36")
		req2.Header.Set("Accept", "text/html,application/xhtml+xml")
		req2.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
		if cookie != "" {
			req2.Header.Set("Cookie", cookie)
			req2.Header.Set("Referer", "https://mp.weixin.qq.com/cgi-bin/home?t=home/index&lang=zh_CN")
		}
		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req2)
		if err != nil {
			cancel()
			fmt.Printf("analyze-style: 请求文章失败 [%s]: %v\n", articles[i].Link, err)
			continue
		}
		body2, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		resp.Body.Close()
		cancel()
		if err != nil {
			fmt.Printf("analyze-style: 读取文章失败 [%s]: %v\n", articles[i].Link, err)
			continue
		}
		text := extractTextFromHTML(string(body2))
		if text == "" {
			fmt.Printf("analyze-style: 文章 %s 提取文本为空, HTML长度=%d\n", articles[i].Link, len(body2))
		} else {
			fmt.Printf("analyze-style: 文章 %s 提取文本=%d字符\n", articles[i].Link, len([]rune(text)))
		}
		articles[i].Text = text
	}

	// Build analysis prompt
	var sb strings.Builder
	sb.WriteString("以下是公众号「")
	sb.WriteString(req.Nickname)
	sb.WriteString("」的几篇文章内容，请分析其写作风格：\n\n")
	validCount := 0
	for i, a := range articles {
		if a.Text == "" {
			continue
		}
		validCount++
		sb.WriteString(fmt.Sprintf("【文章 %d】%s\n", i+1, a.Title))
		sb.WriteString(truncateRunes(a.Text, 2000))
		sb.WriteString("\n\n---\n\n")
	}
	if validCount == 0 {
		fmt.Printf("analyze-style: 无法提取文章文本内容, articles=%d\n", len(articles))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "无法提取文章文本内容"})
		return
	}

	sb.WriteString(`请从以下维度分析该公众号的写作风格，输出一段简洁的系统提示词（system prompt），使其能在 AI 写作时指导模型模仿该风格：

1. 语气语调（正式/轻松/煽情/客观/犀利等）
2. 常用句式特点（长短句偏好、开头结尾模式、过渡方式）
3. 段落结构（是否有固定模板、配图习惯、标题风格）
4. 用词特点（专业术语密度、口语化程度、成语/网络用语使用）
5. 受众定位（面向什么人群，语言难度）
6. 标志性表达（如果有的话，比如特定的开场白、结尾语、口头禅）

请直接输出一段中文 system prompt，以"你是一位微信公众号「` + req.Nickname + `」的写手"开头，控制在 500 字以内。不要包含任何其他说明。`)

	// Call LLM
	llm, err := getAILLM()
	if err != nil {
		fmt.Printf("analyze-style: AI 初始化失败: %v\n", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "AI 初始化失败: " + err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	stylePrompt, err := llms.GenerateFromSinglePrompt(ctx, llm, sb.String())
	if err != nil {
		fmt.Printf("analyze-style: AI 分析失败: %v\n", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "AI 分析失败: " + err.Error()})
		return
	}

	// Save style
	style := &WritingStyle{
		Name:        req.Nickname + " 写作风格",
		FakeID:      req.FakeID,
		Nickname:    req.Nickname,
		StylePrompt: strings.TrimSpace(stylePrompt),
		SampleCount: validCount,
	}
	if err := saveStyle(style); err != nil {
		fmt.Printf("analyze-style: 保存风格失败: %v\n", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "保存风格失败: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"style": style})
}

// GET / DELETE /api/ai/styles
func handleAIStyles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		styles, err := listStyles()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"styles": styles})
	case http.MethodPost:
		// DELETE via POST with id in body
		r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
		defer r.Body.Close()
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if req.ID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
			return
		}
		if err := deleteStyleByID(req.ID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// POST /api/ai/write (SSE streaming)
func handleAIWrite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	defer r.Body.Close()
	var req struct {
		StyleID string `json:"styleId"`
		Topic   string `json:"topic"`
		Outline string `json:"outline"`
		Length  string `json:"length"` // short | medium | long
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.Topic == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "topic is required"})
		return
	}

	// Load style if specified
	var systemPrompt string
	if req.StyleID != "" {
		style, err := getStyle(req.StyleID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "style not found: " + err.Error()})
			return
		}
		systemPrompt = style.StylePrompt + "\n\n请生成一篇微信公众号文章，使用 Markdown 格式。输出中文。"
	} else {
		systemPrompt = "你是一位微信公众号文章作者。请生成一篇微信公众号文章，使用 Markdown 格式。输出中文。"
	}

	// Build user prompt
	var userPrompt strings.Builder
	userPrompt.WriteString("主题：")
	userPrompt.WriteString(req.Topic)
	userPrompt.WriteString("\n")

	lengthGuide := "中篇（约1500字）"
	switch req.Length {
	case "short":
		lengthGuide = "短篇（约500字）"
	case "medium":
		lengthGuide = "中篇（约1500字）"
	case "long":
		lengthGuide = "长篇（约3000字）"
	}
	userPrompt.WriteString("字数要求：" + lengthGuide + "\n")

	if req.Outline != "" {
		userPrompt.WriteString("大纲：\n")
		userPrompt.WriteString(req.Outline)
		userPrompt.WriteString("\n")
	}

	// Setup LLM
	llm, err := getAILLM()
	if err != nil {
		fmt.Printf("ai-write: AI 初始化失败: %v\n", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "AI 初始化失败: " + err.Error()})
		return
	}

	// Setup SSE
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming not supported"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	sendSSE := func(data string) {
		// Escape newlines in JSON
		jsonData, _ := json.Marshal(map[string]string{"token": data})
		fmt.Fprintf(w, "data: %s\n\n", string(jsonData))
		flusher.Flush()
	}

	messages := []llms.MessageContent{
		{Role: llms.ChatMessageTypeSystem, Parts: []llms.ContentPart{llms.TextContent{Text: systemPrompt}}},
		{Role: llms.ChatMessageTypeHuman, Parts: []llms.ContentPart{llms.TextContent{Text: userPrompt.String()}}},
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	_, err = llm.GenerateContent(ctx, messages,
		llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			sendSSE(string(chunk))
			return nil
		}),
	)
	if err != nil {
		sendSSE("\n\n[生成出错: " + err.Error() + "]")
	}

	fmt.Fprintf(w, "data: {\"done\":true}\n\n")
	flusher.Flush()
}

// --- Image Generation (OpenAI-compatible) ---

func handleAIImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	defer r.Body.Close()

	var req struct {
		Prompt string `json:"prompt"`
		N      int    `json:"n,omitempty"`
		Size   string `json:"size,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.Prompt == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "prompt is required"})
		return
	}
	if req.N == 0 {
		req.N = 1
	}
	if req.Size == "" {
		req.Size = "1024x1024"
	}

	currentCredsMu.RLock()
	baseURL := currentCreds.AIImageBaseURL
	apiKey := currentCreds.AIImageAPIKey
	model := currentCreds.AIImageModel
	if baseURL == "" {
		baseURL = currentCreds.AIBaseURL
	}
	if apiKey == "" {
		apiKey = currentCreds.AIAPIKey
	}
	currentCredsMu.RUnlock()

	if apiKey == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "AI API Key 未配置"})
		return
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if model == "" {
		model = "dall-e-3"
	}

	apiReq := map[string]any{"model": model, "prompt": req.Prompt, "n": req.N, "size": req.Size}
	body, _ := json.Marshal(apiReq)

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/images/generations", bytes.NewReader(body))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "图像生成请求失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "读取响应失败"})
		return
	}

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]any
		if json.Unmarshal(respBody, &errResp) == nil {
			if e, ok := errResp["error"].(map[string]any); ok {
				if msg, ok := e["message"].(string); ok {
					writeJSON(w, resp.StatusCode, map[string]string{"error": msg})
					return
				}
			}
		}
		writeJSON(w, resp.StatusCode, map[string]string{"error": string(respBody)})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(respBody)
}
