package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const wechatAPIHost = "https://api.weixin.qq.com"

// CallMode is how open-platform requests leave this process.
type CallMode string

const (
	ModeLocal CallMode = "local"
	ModeProxy CallMode = "proxy"
)

// wechatPostJSON POSTs JSON to a WeChat CGI path (e.g. "/cgi-bin/material/batchget_material").
// Local mode injects access_token; proxy mode forwards via remote proxy.
func wechatPostJSON(cgiPath string, payload any, result any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	raw, err := wechatDo("POST", cgiPath, "application/json", body, "")
	if err != nil {
		return err
	}
	if result == nil {
		return nil
	}
	return json.Unmarshal(raw, result)
}

// wechatPostMultipart uploads a file field "media" to a WeChat CGI path.
// extraQuery is appended as-is (e.g. "type=image"), without leading '?'.
func wechatPostMultipart(cgiPath, filename string, data []byte, extraQuery string, result any) error {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("media", filename)
	if err != nil {
		return err
	}
	if _, err := part.Write(data); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	raw, err := wechatDo("POST", cgiPath, w.FormDataContentType(), body.Bytes(), extraQuery)
	if err != nil {
		return err
	}
	if result == nil {
		return nil
	}
	return json.Unmarshal(raw, result)
}

func wechatDo(method, cgiPath, contentType string, body []byte, extraQuery string) ([]byte, error) {
	mode, base, key := GetCallMode()
	if mode == ModeProxy {
		return proxyForward(method, cgiPath, contentType, body, extraQuery, base, key)
	}
	return localWechatDo(method, cgiPath, contentType, body, extraQuery)
}

func localWechatDo(method, cgiPath, contentType string, body []byte, extraQuery string) ([]byte, error) {
	var out []byte
	err := WithWeChatToken(func(token string) error {
		q := url.Values{}
		q.Set("access_token", token)
		if extraQuery != "" {
			if eq, err := url.ParseQuery(extraQuery); err == nil {
				for k, vs := range eq {
					for _, v := range vs {
						q.Add(k, v)
					}
				}
			}
		}
		full := wechatAPIHost + cgiPath + "?" + q.Encode()
		req, err := http.NewRequest(method, full, bytes.NewReader(body))
		if err != nil {
			return err
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("wechat request: %w", err)
		}
		defer resp.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
		if err != nil {
			return err
		}
		out = raw
		return nil
	})
	return out, err
}

func proxyForward(method, cgiPath, contentType string, body []byte, extraQuery, base, key string) ([]byte, error) {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return nil, fmt.Errorf("代理模式未配置 base_url")
	}
	if key == "" {
		return nil, fmt.Errorf("代理模式未配置 api_key")
	}
	u, err := url.Parse(base + "/v1/forward")
	if err != nil {
		return nil, fmt.Errorf("invalid proxy url: %w", err)
	}
	q := u.Query()
	q.Set("path", cgiPath)
	if extraQuery != "" {
		q.Set("query", extraQuery)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(method, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("proxy request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		var er struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &er)
		if er.Error != "" {
			return nil, fmt.Errorf("proxy: %s", er.Error)
		}
		return nil, fmt.Errorf("proxy http %d: %s", resp.StatusCode, truncateStr(string(raw), 200))
	}
	return raw, nil
}

// TestProxyConnection pings the remote proxy health + auth.
func TestProxyConnection(base, key string) error {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return fmt.Errorf("base_url 为空")
	}
	if key == "" {
		return fmt.Errorf("api_key 为空")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, base+"/v1/status", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("API Key 无效")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("proxy http %d: %s", resp.StatusCode, truncateStr(string(raw), 120))
	}
	var st struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		Mode  string `json:"mode"`
	}
	_ = json.Unmarshal(raw, &st)
	if st.Error != "" {
		return fmt.Errorf("%s", st.Error)
	}
	return nil
}
