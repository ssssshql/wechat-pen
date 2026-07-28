package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	cgiMaterialBatch = "/cgi-bin/material/batchget_material"
	cgiMaterialDel   = "/cgi-bin/material/del_material"
	cgiAddMaterial   = "/cgi-bin/material/add_material"
)

type materialBatchReq struct {
	Type   string `json:"type"`
	Offset int    `json:"offset"`
	Count  int    `json:"count"`
}

type materialBatchResp struct {
	TotalCount int            `json:"total_count"`
	ItemCount  int            `json:"item_count"`
	Item       []materialItem `json:"item"`
	ErrCode    int            `json:"errcode"`
	ErrMsg     string         `json:"errmsg"`
}

type materialItem struct {
	MediaID    string `json:"media_id"`
	Name       string `json:"name"`
	UpdateTime int64  `json:"update_time"`
	URL        string `json:"url"`
}

type materialListResp struct {
	TotalCount int            `json:"total_count"`
	ItemCount  int            `json:"item_count"`
	Items      []materialItem `json:"items"`
}

func handleMaterialBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	materialType := r.URL.Query().Get("type")
	if materialType == "" {
		materialType = "image"
	}
	offset := 0
	fmt.Sscanf(r.URL.Query().Get("offset"), "%d", &offset)
	count := 20
	if c := r.URL.Query().Get("count"); c != "" {
		fmt.Sscanf(c, "%d", &count)
	}
	if count < 1 || count > 20 {
		count = 20
	}

	var resp materialBatchResp
	err := wechatPostJSON(cgiMaterialBatch, materialBatchReq{
		Type: materialType, Offset: offset, Count: count,
	}, &resp)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if resp.ErrCode != 0 {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error": resp.ErrMsg, "errcode": resp.ErrCode,
		})
		return
	}

	writeJSON(w, http.StatusOK, materialListResp{
		TotalCount: resp.TotalCount,
		ItemCount:  resp.ItemCount,
		Items:      resp.Item,
	})
}

func handleMaterialDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	defer r.Body.Close()

	var req struct {
		MediaID string `json:"media_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.MediaID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "media_id is required"})
		return
	}

	var delResp struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	err := wechatPostJSON(cgiMaterialDel, map[string]string{"media_id": req.MediaID}, &delResp)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if delResp.ErrCode != 0 {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error": delResp.ErrMsg, "errcode": delResp.ErrCode,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func handleMaterialUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	defer r.Body.Close()

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "parse form: " + err.Error()})
		return
	}

	file, _, err := r.FormFile("media")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing media file"})
		return
	}
	defer file.Close()

	imgData, err := io.ReadAll(file)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "read file: " + err.Error()})
		return
	}

	var result struct {
		MediaID string `json:"media_id"`
		URL     string `json:"url"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	err = wechatPostMultipart(cgiAddMaterial, "cover.jpg", imgData, "type=image", &result)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if result.ErrCode != 0 {
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("wechat error %d: %s", result.ErrCode, result.ErrMsg),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"media_id": result.MediaID,
		"url":      result.URL,
	})
}
