package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tmc/langchaingo/llms"
)

// POST /api/ai/chat (SSE streaming)
// Request: { messages: [{role:"user", content:"..."}], currentContent: "...", styleId: "..." }
// Response: SSE stream with tokens, each token is content to insert into the editor
func handleAIChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<17)
	defer r.Body.Close()

	var req struct {
		Messages       []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		CurrentContent string `json:"currentContent"`
		StyleID        string `json:"styleId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if len(req.Messages) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "messages is required"})
		return
	}

	// Build system prompt
	var systemPrompt string
	if req.StyleID != "" {
		style, err := getStyle(req.StyleID)
		if err != nil {
			fmt.Printf("ai-chat: 风格未找到: %v\n", err)
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "风格未找到: " + err.Error()})
			return
		}
		systemPrompt = style.StylePrompt
	} else {
		systemPrompt = "你是一位微信公众号文章写作助手。"
	}
	systemPrompt += "\n\n你是一个写作助手，帮助用户创作和修改微信公众号文章。请遵守以下规则：\n" +
		"1. 直接输出 Markdown 格式的文章内容，不要加任何解释、说明或前言\n" +
		"2. 只输出需要写入编辑器的内容，不要输出元对话（如\"好的，我来帮你写\"之类的）\n" +
		"3. 输出中文\n" +
		"4. 如果用户要求修改现有内容，输出完整的修改后版本\n" +
		"5. 如果用户要求续写，只输出续写部分\n" +
		"6. 如果用户要求写新文章，输出完整文章"

	// Build langchain messages
	msgContents := []llms.MessageContent{
		{Role: llms.ChatMessageTypeSystem, Parts: []llms.ContentPart{llms.TextContent{Text: systemPrompt}}},
	}

	// Include current editor content as context if present
	if req.CurrentContent != "" {
		contextMsg := "当前编辑器中的文章内容如下（供你参考和修改）：\n\n" + req.CurrentContent
		msgContents = append(msgContents, llms.MessageContent{
			Role:  llms.ChatMessageTypeSystem,
			Parts: []llms.ContentPart{llms.TextContent{Text: contextMsg}},
		})
	}

	// Add user messages
	for _, m := range req.Messages {
		role := llms.ChatMessageTypeHuman
		if m.Role == "assistant" {
			role = llms.ChatMessageTypeAI
		}
		msgContents = append(msgContents, llms.MessageContent{
			Role:  role,
			Parts: []llms.ContentPart{llms.TextContent{Text: m.Content}},
		})
	}

	// Setup LLM
	llm, err := getAILLM()
	if err != nil {
		fmt.Printf("ai-chat: AI 初始化失败: %v\n", err)
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
		jsonData, _ := json.Marshal(map[string]string{"token": data})
		fmt.Fprintf(w, "data: %s\n\n", string(jsonData))
		flusher.Flush()
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	_, err = llm.GenerateContent(ctx, msgContents,
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
