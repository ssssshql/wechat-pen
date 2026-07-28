package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const stableTokenURL = "https://api.weixin.qq.com/cgi-bin/stable_token"

type tokenCache struct {
	mu        sync.RWMutex
	token     string
	expiresAt time.Time
}

func (c *tokenCache) get() (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.token != "" && time.Now().Before(c.expiresAt) {
		return c.token, true
	}
	return "", false
}

func (c *tokenCache) set(token string, expiresIn int) {
	ttl := expiresIn - 300
	if ttl < 60 {
		ttl = 60
	}
	c.mu.Lock()
	c.token = token
	c.expiresAt = time.Now().Add(time.Duration(ttl) * time.Second)
	c.mu.Unlock()
}

func (c *tokenCache) clear() {
	c.mu.Lock()
	c.token = ""
	c.expiresAt = time.Time{}
	c.mu.Unlock()
}

type wechatToken struct {
	cache  tokenCache
	appID  string
	secret string
	client *http.Client
}

func newWechatToken(appID, secret string) *wechatToken {
	return &wechatToken{
		appID:  appID,
		secret: secret,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (w *wechatToken) accessToken() (string, error) {
	if t, ok := w.cache.get(); ok {
		return t, nil
	}
	body, _ := json.Marshal(map[string]string{
		"grant_type": "client_credential",
		"appid":      w.appID,
		"secret":     w.secret,
	})
	resp, err := w.client.Post(stableTokenURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("request token: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := json.Unmarshal(raw, &tok); err != nil {
		return "", fmt.Errorf("parse token: %w", err)
	}
	if tok.ErrCode != 0 {
		return "", fmt.Errorf("wechat api error %d: %s", tok.ErrCode, tok.ErrMsg)
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("empty access_token")
	}
	w.cache.set(tok.AccessToken, tok.ExpiresIn)
	return tok.AccessToken, nil
}

func (w *wechatToken) withToken(fn func(token string) error) error {
	token, err := w.accessToken()
	if err != nil {
		return err
	}
	err = fn(token)
	if err != nil && isInvalidToken(err.Error()) {
		w.cache.clear()
		token, err = w.accessToken()
		if err != nil {
			return err
		}
		return fn(token)
	}
	return err
}

func isInvalidToken(msg string) bool {
	return strings.Contains(msg, "invalid_token") || strings.Contains(msg, "access_token expired")
}
