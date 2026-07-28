package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"wechat-pen/converter"
)

const cgiDraftAdd = "/cgi-bin/draft/add"

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
		Markdown     string `json:"markdown"`
		Title        string `json:"title"`
		Author       string `json:"author"`
		Digest       string `json:"digest"`
		ThumbMediaID string `json:"thumb_media_id"`
		Style        string `json:"style"`
		PrimaryColor string `json:"primaryColor"`
		UploadImages bool   `json:"upload_images"`
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

	processed, err := processContentImages(content, req.UploadImages)
	if err != nil {
		fmt.Printf("draft/add: upload images failed: %v\n", err)
	} else {
		content = processed
	}

	draftReq := draftAddReq{
		Articles: []draftArticle{{
			Title:        req.Title,
			Author:       req.Author,
			Digest:       req.Digest,
			Content:      content,
			ThumbMediaID: req.ThumbMediaID,
		}},
	}

	var draftResp draftAddResp
	err = wechatPostJSON(cgiDraftAdd, draftReq, &draftResp)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if draftResp.ErrCode != 0 {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error": draftResp.ErrMsg, "errcode": draftResp.ErrCode,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"media_id": draftResp.MediaID,
	})
}
