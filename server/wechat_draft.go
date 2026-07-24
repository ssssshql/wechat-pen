package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"wechat-pen/converter"
)

const wechatDraftAddURL = "https://api.weixin.qq.com/cgi-bin/draft/add"

type draftArticle struct {
	Title              string `json:"title"`
	Author             string `json:"author"`
	Digest             string `json:"digest"`
	Content            string `json:"content"`
	ContentSourceURL   string `json:"content_source_url"`
	ThumbMediaID       string `json:"thumb_media_id"`
	NeedOpenComment    int    `json:"need_open_comment"`
	OnlyFansCanComment int    `json:"only_fans_can_comment"`
}

type draftAddReq struct {
	Articles []draftArticle `json:"articles"`
}

type draftAddResp struct {
	MediaID string `json:"media_id"`
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

func handleDraftAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	defer r.Body.Close()

	var req struct {
		Markdown      string `json:"markdown"`
		Title         string `json:"title"`
		Author        string `json:"author"`
		Digest        string `json:"digest"`
		ThumbMediaID  string `json:"thumb_media_id"`
		Style         string `json:"style"`
		PrimaryColor  string `json:"primaryColor"`
		UploadImages  bool   `json:"upload_images"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Markdown) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "markdown is required"})
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return
	}
	if strings.TrimSpace(req.ThumbMediaID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "thumb_media_id is required"})
		return
	}

	// Convert markdown to WeChat-compatible HTML
	style := converter.StylePack(strings.ToLower(strings.TrimSpace(req.Style)))
	if style == "" {
		style = converter.StyleSimple
	}
	cfg := converter.Config{
		Theme:        converter.ThemeWeChat,
		Style:        style,
		PrimaryColor: req.PrimaryColor,
		Justify:      true,
		Highlight:    true,
		ImageCaption: true,
	}
	result, err := converter.ConvertEx([]byte(req.Markdown), cfg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "convert: " + err.Error()})
		return
	}

	content := string(result.HTML)

	// Upload content images to WeChat:
	//   upload_images=true  → uploadimg (≤1MB auto-compress, no quota)
	//   upload_images=false → add_material (permanent material, no size limit)
	processed, err := processContentImages(content, req.UploadImages)
	if err != nil {
		fmt.Printf("draft/add: upload images failed: %v\n", err)
	} else {
		content = processed
	}

	// Build draft API request
	draftReq := draftAddReq{
		Articles: []draftArticle{{
			Title:        req.Title,
			Author:       req.Author,
			Digest:       req.Digest,
			Content:      content,
			ThumbMediaID: req.ThumbMediaID,
		}},
	}

	body, _ := json.Marshal(draftReq)

	var draftResp draftAddResp
	err = WithWeChatToken(func(token string) error {
		url := wechatDraftAddURL + "?access_token=" + token
		resp_, err := http.Post(url, "application/json", strings.NewReader(string(body)))
		if err != nil {
			return fmt.Errorf("request draft/add: %w", err)
		}
		defer resp_.Body.Close()
		respBody, _ := io.ReadAll(resp_.Body)
		if err := json.Unmarshal(respBody, &draftResp); err != nil {
			return fmt.Errorf("parse draft/add: %w", err)
		}
		return nil
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if draftResp.ErrCode != 0 {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error":   draftResp.ErrMsg,
			"errcode": draftResp.ErrCode,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"media_id": draftResp.MediaID,
	})
}
