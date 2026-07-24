package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

const wechatMaterialBatchURL = "https://api.weixin.qq.com/cgi-bin/material/batchget_material"
const wechatMaterialDelURL = "https://api.weixin.qq.com/cgi-bin/material/del_material"

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

	reqBody, _ := json.Marshal(materialBatchReq{
		Type:   materialType,
		Offset: offset,
		Count:  count,
	})

	var resp materialBatchResp
	err := WithWeChatToken(func(token string) error {
		url := wechatMaterialBatchURL + "?access_token=" + token
		resp_, err := http.Post(url, "application/json", strings.NewReader(string(reqBody)))
		if err != nil {
			return fmt.Errorf("request batchget_material: %w", err)
		}
		defer resp_.Body.Close()
		body, _ := io.ReadAll(resp_.Body)
		if err := json.Unmarshal(body, &resp); err != nil {
			return fmt.Errorf("parse batchget_material: %w", err)
		}
		return nil
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if resp.ErrCode != 0 {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error":   resp.ErrMsg,
			"errcode": resp.ErrCode,
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
	err := WithWeChatToken(func(token string) error {
		url := wechatMaterialDelURL + "?access_token=" + token
		body, _ := json.Marshal(map[string]string{"media_id": req.MediaID})
		resp_, err := http.Post(url, "application/json", strings.NewReader(string(body)))
		if err != nil {
			return fmt.Errorf("request del_material: %w", err)
		}
		defer resp_.Body.Close()
		respBody, _ := io.ReadAll(resp_.Body)
		if err := json.Unmarshal(respBody, &delResp); err != nil {
			return fmt.Errorf("parse del_material: %w", err)
		}
		return nil
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if delResp.ErrCode != 0 {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error":   delResp.ErrMsg,
			"errcode": delResp.ErrCode,
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

	// Limit upload to 10MB
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	defer r.Body.Close()

	// Parse multipart form
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

	// Upload to WeChat as permanent material via uploadViaAddMaterial (from wechat_upload.go)
	wechatURL, err := uploadViaAddMaterial(imgData)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "upload to wechat: " + err.Error()})
		return
	}

	// We only get the URL from add_material, not the media_id directly.
	// We need to also get the media_id. Let's re-read the response...
	// Actually uploadViaAddMaterial only returns the URL. Let's add a version that returns media_id too.
	var mediaID string
	// Use a separate call that captures both
	err = WithWeChatToken(func(token string) error {
		url := wechatAddMaterialURL + "?access_token=" + token + "&type=image"

		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("media", "cover.jpg")
		if err != nil {
			return err
		}
		if _, err := part.Write(imgData); err != nil {
			return err
		}
		writer.Close()

		req, err := http.NewRequest("POST", url, &body)
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("request add_material: %w", err)
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)

		var result struct {
			MediaID string `json:"media_id"`
			URL     string `json:"url"`
			ErrCode int    `json:"errcode"`
			ErrMsg  string `json:"errmsg"`
		}
		if err := json.Unmarshal(respBody, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
		if result.ErrCode != 0 {
			return fmt.Errorf("wechat error %d: %s", result.ErrCode, result.ErrMsg)
		}
		mediaID = result.MediaID
		wechatURL = result.URL
		return nil
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"media_id": mediaID,
		"url":      wechatURL,
	})
}
