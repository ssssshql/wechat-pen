package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const wechatHost = "https://api.weixin.qq.com"

type server struct {
	apiKey string
	token  *wechatToken
	appID  string
	client *http.Client
}

func newServer(apiKey, appID, secret string) *server {
	return &server{
		apiKey: apiKey,
		appID:  appID,
		token:  newWechatToken(appID, secret),
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

func (s *server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/v1/status", s.withAPIKey(s.handleStatus))
	mux.HandleFunc("/v1/forward", s.withAPIKey(s.handleForward))
	return withCORS(withLog(mux))
}

func (s *server) withAPIKey(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer"))
		if got == "" {
			got = strings.TrimSpace(r.Header.Get("X-API-Key"))
		}
		if got == "" || got != s.apiKey {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (s *server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token, err := s.token.accessToken()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": false, "error": err.Error(), "mode": "proxy-server",
		})
		return
	}
	prefix := token
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "mode": "proxy-server", "token_prefix": prefix, "appid": s.appID,
	})
}

// handleForward: any method /v1/forward?path=/cgi-bin/...&query=type=image
// Injects access_token and forwards body/content-type to api.weixin.qq.com.
func (s *server) handleForward(w http.ResponseWriter, r *http.Request) {
	cgiPath := r.URL.Query().Get("path")
	if cgiPath == "" || !strings.HasPrefix(cgiPath, "/cgi-bin/") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path must be /cgi-bin/..."})
		return
	}
	// basic path traversal guard
	if strings.Contains(cgiPath, "..") || strings.Contains(cgiPath, "://") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid path"})
		return
	}
	extra := r.URL.Query().Get("query")
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	ct := r.Header.Get("Content-Type")
	method := r.Method
	if method == http.MethodGet || method == http.MethodHead {
		method = http.MethodPost
	}

	var out []byte
	err = s.token.withToken(func(token string) error {
		q := url.Values{}
		q.Set("access_token", token)
		if extra != "" {
			if eq, e := url.ParseQuery(extra); e == nil {
				for k, vs := range eq {
					for _, v := range vs {
						q.Add(k, v)
					}
				}
			}
		}
		full := wechatHost + cgiPath + "?" + q.Encode()
		req, e := http.NewRequest(method, full, bytes.NewReader(body))
		if e != nil {
			return e
		}
		if ct != "" {
			req.Header.Set("Content-Type", ct)
		}
		resp, e := s.client.Do(req)
		if e != nil {
			return fmt.Errorf("wechat request: %w", e)
		}
		defer resp.Body.Close()
		raw, e := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
		if e != nil {
			return e
		}
		// surface invalid token for retry
		var probe struct {
			ErrCode int    `json:"errcode"`
			ErrMsg  string `json:"errmsg"`
		}
		_ = json.Unmarshal(raw, &probe)
		if probe.ErrCode == 40001 || probe.ErrCode == 42001 ||
			strings.Contains(probe.ErrMsg, "access_token") {
			return fmt.Errorf("invalid_token: %s", probe.ErrMsg)
		}
		out = raw
		return nil
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		fmt.Printf("%s %s %s\n", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}
