package server

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

const wechatTokenURL = "https://api.weixin.qq.com/cgi-bin/stable_token"

type wechatTokenResp struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

// tokenCache holds the cached WeChat access token.
var tokenCache = struct {
	sync.RWMutex
	token     string
	expiresAt time.Time
}{}

// GetWeChatAccessToken fetches or returns a cached access_token.
// Uses AppID/Secret from currentCreds. Returns error if creds are empty.
func GetWeChatAccessToken() (string, error) {
	tokenCache.RLock()
	if tokenCache.token != "" && time.Now().Before(tokenCache.expiresAt) {
		t := tokenCache.token
		tokenCache.RUnlock()
		return t, nil
	}
	tokenCache.RUnlock()

	appid, secret := GetCredentials()

	if appid == "" || secret == "" {
		return "", fmt.Errorf("公众号 AppID/Secret 未配置")
	}

	body, _ := json.Marshal(map[string]string{
		"grant_type": "client_credential",
		"appid":      appid,
		"secret":     secret,
	})

	resp, err := http.Post(wechatTokenURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("request token: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var tok wechatTokenResp
	if err := json.Unmarshal(respBody, &tok); err != nil {
		return "", fmt.Errorf("parse token response: %w", err)
	}
	if tok.ErrCode != 0 {
		return "", fmt.Errorf("wechat api error %d: %s", tok.ErrCode, tok.ErrMsg)
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("empty access_token from wechat api")
	}

	// Refresh a bit early to be safe
	ttl := tok.ExpiresIn - 300
	if ttl < 60 {
		ttl = 60
	}

	tokenCache.Lock()
	tokenCache.token = tok.AccessToken
	tokenCache.expiresAt = time.Now().Add(time.Duration(ttl) * time.Second)
	tokenCache.Unlock()

	return tok.AccessToken, nil
}

// InvalidateToken clears the cached token (useful after detecting an invalid_token error).
func InvalidateToken() {
	tokenCache.Lock()
	tokenCache.token = ""
	tokenCache.expiresAt = time.Time{}
	tokenCache.Unlock()
}

// WithWeChatToken runs fn with a fresh token. On invalid_token, clears cache once and retries.
func WithWeChatToken(fn func(token string) error) error {
	token, err := GetWeChatAccessToken()
	if err != nil {
		return err
	}
	err = fn(token)
	if err != nil && containsInvalidToken(err.Error()) {
		InvalidateToken()
		token, err = GetWeChatAccessToken()
		if err != nil {
			return err
		}
		return fn(token)
	}
	return err
}

func containsInvalidToken(msg string) bool {
	return strings.Contains(msg, "invalid_token") || strings.Contains(msg, "access_token expired")
}
