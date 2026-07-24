package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"wechat-pen/converter"
)

// Options configures the HTTP server.
type Options struct {
	ThemesDir string
	AppID     string
	Secret    string
}

// Credentials holds WeChat credentials. Populated from env vars, CLI flags, or config file on startup.
var currentCredsMu sync.RWMutex
var currentCreds = struct {
	AppID       string `json:"appid"`
	Secret      string `json:"secret"`
	LoginCookie string `json:"login_cookie"`
	Token       string `json:"token"`
	Fingerprint string `json:"fingerprint"`
}{}

// GetCredentials returns a snapshot of current credentials (thread-safe).
func GetCredentials() (appid, secret string) {
	currentCredsMu.RLock()
	defer currentCredsMu.RUnlock()
	return currentCreds.AppID, currentCreds.Secret
}

// LoadCredsFromEnv reads credentials from env vars, setting defaults if present.
func LoadCredsFromEnv() {
	currentCredsMu.Lock()
	defer currentCredsMu.Unlock()
	if v := os.Getenv("WECHAT_PEN_APPID"); v != "" && currentCreds.AppID == "" {
		currentCreds.AppID = v
	}
	if v := os.Getenv("WECHAT_PEN_SECRET"); v != "" && currentCreds.Secret == "" {
		currentCreds.Secret = v
	}
	// Load config file as fallback
	if cfg, err := loadConfigFile(); err == nil {
		if currentCreds.AppID == "" {
			currentCreds.AppID = cfg.AppID
		}
		if currentCreds.Secret == "" {
			currentCreds.Secret = cfg.Secret
		}
		if currentCreds.LoginCookie == "" {
			currentCreds.LoginCookie = cfg.LoginCookie
		}
		if currentCreds.Token == "" {
			currentCreds.Token = cfg.Token
		}
		if currentCreds.Fingerprint == "" {
			currentCreds.Fingerprint = cfg.Fingerprint
		}
	}
}

type configFile struct {
	AppID       string `json:"appid"`
	Secret      string `json:"secret"`
	LoginCookie string `json:"login_cookie"`
	Token       string `json:"token,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

func configPath() string {
	if home, _ := os.UserHomeDir(); home != "" {
		return filepath.Join(home, ".wechat-pen.json")
	}
	return ".wechat-pen.json"
}

func loadConfigFile() (configFile, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return configFile{}, err
	}
	var cfg configFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return configFile{}, err
	}
	return cfg, nil
}

func saveConfigFile(cfg configFile) error {
	data, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(configPath(), data, 0o600)
}

// New returns an HTTP handler for the wechat-pen API and SPA.
func New(opts ...Options) http.Handler {
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
	if o.ThemesDir == "" {
		o.ThemesDir = "themes"
	}
	if n, err := converter.LoadThemesDir(o.ThemesDir); err != nil {
		fmt.Printf("themes: load %s failed: %v\n", o.ThemesDir, err)
	} else if n > 0 {
		fmt.Printf("themes: loaded %d pack(s) from %s\n", n, o.ThemesDir)
	}
	// CLI flags take priority over env/config
	if o.AppID != "" {
		currentCreds.AppID = o.AppID
	}
	if o.Secret != "" {
		currentCreds.Secret = o.Secret
	}
	// Then fill from env/config for any missing
	LoadCredsFromEnv()
	if currentCreds.AppID != "" || currentCreds.Secret != "" {
		fmt.Printf("credentials: loaded (%s:%d chars)\n", currentCreds.AppID, len(currentCreds.Secret))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/convert", handleConvert)
	mux.HandleFunc("/api/styles", handleStyles)
	mux.HandleFunc("/api/themes/reload", handleThemesReload(o.ThemesDir))
	mux.HandleFunc("/api/themes/import", handleThemesImport(o.ThemesDir))
	mux.HandleFunc("/api/themes/delete", handleThemesDelete(o.ThemesDir))
	mux.HandleFunc("/api/credentials", handleCredentials)
	mux.HandleFunc("/api/material/batch", handleMaterialBatch)
	mux.HandleFunc("/api/material/delete", handleMaterialDelete)
	mux.HandleFunc("/api/material/upload", handleMaterialUpload)
	mux.HandleFunc("/api/draft/add", handleDraftAdd)
	mux.HandleFunc("/api/biz/search", handleSearchBiz)
	mux.HandleFunc("/api/biz/articles", handleBizArticles)
	mux.HandleFunc("/api/biz/article/proxy", handleArticleProxy)
	mux.HandleFunc("/api/biz/image/proxy", handleImageProxy)
	mux.HandleFunc("/api/login/start", handleLoginStart)
	mux.HandleFunc("/api/login/status", handleLoginStatus)
	mux.HandleFunc("/api/login/cancel", handleLoginCancel)
	mux.HandleFunc("/api/login/logout", handleLoginLogout)
	mux.HandleFunc("/api/login/params", handleLoginParams)
	mux.HandleFunc("/api/ip", handleOutboundIP)
	mux.HandleFunc("/api/healthz", handleHealth)
	mux.HandleFunc("/healthz", handleHealth)
	mux.HandleFunc("/legacy", handleIndex)
	mux.Handle("/static/", http.StripPrefix("/static/", staticHandler()))

	if sub, err := spaSubFS(); err == nil {
		fileServer := http.FileServer(http.FS(sub))
		mux.Handle("/assets/", fileServer)
		mux.Handle("/favicon.svg", fileServer)
		mux.Handle("/favicon.ico", fileServer)
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && !strings.HasPrefix(r.URL.Path, "/api") {
			if tryServeSPAFile(w, r) {
				return
			}
		}
		if b, err := tryReadSPA(); err == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			_, _ = w.Write(b)
			return
		}
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><meta charset="utf-8"><title>wechat-pen</title>
<body style="font-family:system-ui;padding:2rem;line-height:1.6">
<h1>wechat-pen API</h1>
<pre style="background:#f5f5f5;padding:1rem;border-radius:8px">cd web &amp;&amp; npm run dev
go run . serve</pre>
<p>API: <code>POST /api/convert</code> · <code>GET /api/styles</code> · <code>POST /api/themes/reload</code></p>
</body>`))
	})
	return withCORS(withLog(mux))
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// ListenAndServe starts the API on addr (e.g. ":8080").
func ListenAndServe(addr string, opts ...Options) error {
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
	if o.ThemesDir == "" {
		if abs, err := filepath.Abs("themes"); err == nil {
			o.ThemesDir = abs
		} else {
			o.ThemesDir = "themes"
		}
	}
	s := &http.Server{
		Addr:              addr,
		Handler:           New(o),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
	fmt.Printf("wechat-pen → http://127.0.0.1%s\n", normalizeAddr(addr))
	fmt.Printf("themes dir → %s\n", o.ThemesDir)
	if _, err := tryReadSPA(); err != nil {
		fmt.Printf("提示: 未嵌入前端构建产物，开发请另开: cd web && npm run dev\n")
	} else {
		fmt.Printf("已加载嵌入式 SPA\n")
	}
	return s.ListenAndServe()
}

func normalizeAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return addr
	}
	return ":" + addr
}

type convertRequest struct {
	Markdown       string `json:"markdown"`
	Theme          string `json:"theme"`
	Title          string `json:"title"`
	Complete       bool   `json:"complete"`
	Style          string `json:"style"`
	PrimaryColor   string `json:"primaryColor"`
	TextIndent     bool   `json:"textIndent"`
	Justify        *bool  `json:"justify"`
	ParagraphGap   string `json:"paragraphGap"`
	FontSize       string `json:"fontSize"`
	LineHeight     string `json:"lineHeight"`
	Highlight      *bool  `json:"highlight"`
	HighlightTheme string `json:"highlightTheme"`
	TOC            bool   `json:"toc"`
	Footer         bool   `json:"footer"`
	ImageCaption   *bool  `json:"imageCaption"`
	PreviewWidth   string `json:"previewWidth"`
	PreviewShell   string `json:"previewShell"`
}

type convertResponse struct {
	HTML      string                 `json:"html"`
	Preview   string                 `json:"preview"`
	Theme     string                 `json:"theme"`
	Style     string                 `json:"style"`
	CharCount int                    `json:"charCount"`
	Report    converter.CleanReport  `json:"report"`
	Health    converter.HealthReport `json:"health"`
}

type styleItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Primary     string `json:"primary"`
	Builtin     bool   `json:"builtin"`
}

func handleStyles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	items := make([]styleItem, 0, 32)
	for _, b := range converter.ListBuiltinStyles() {
		items = append(items, styleItem{
			ID: b.ID, Name: b.Name, Description: b.Description, Primary: b.Primary, Builtin: true,
		})
	}
	for _, e := range converter.ListExternalThemes() {
		primary := e.Primary
		if primary == "" {
			primary = "#07c160"
		}
		items = append(items, styleItem{
			ID: e.ID, Name: e.Name, Description: e.Description, Primary: primary, Builtin: false,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"styles": items})
}

func handleCredentials(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, currentCreds)
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
		defer r.Body.Close()
		var req struct {
			AppID  string `json:"appid"`
			Secret string `json:"secret"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if req.AppID != "" {
			currentCreds.AppID = req.AppID
			os.Setenv("WECHAT_PEN_APPID", req.AppID)
		}
		if req.Secret != "" {
			currentCreds.Secret = req.Secret
			os.Setenv("WECHAT_PEN_SECRET", req.Secret)
		}
		// Persist to ~/.wechat-pen.json
		if err := saveConfigFile(configFile{AppID: currentCreds.AppID, Secret: currentCreds.Secret}); err != nil {
			fmt.Printf("warn: save config: %v\n", err)
		}
		writeJSON(w, http.StatusOK, currentCreds)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleThemesReload(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		n, err := converter.LoadThemesDir(dir)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"loaded": n, "dir": dir})
	}
}

func handleThemesImport(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		defer r.Body.Close()

		var pack converter.ThemePackFile
		if err := json.NewDecoder(r.Body).Decode(&pack); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json: " + err.Error()})
			return
		}
		saved, err := converter.SaveThemePack(dir, pack)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true,
			"theme": styleItem{
				ID: saved.ID, Name: saved.Name, Description: saved.Description,
				Primary: or(saved.Primary, "#07c160"), Builtin: false,
			},
		})
	}
}

func handleThemesDelete(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
		defer r.Body.Close()

		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// also allow ?id=
			req.ID = r.URL.Query().Get("id")
		}
		if strings.TrimSpace(req.ID) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
			return
		}
		if err := converter.DeleteThemePack(dir, req.ID); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": req.ID})
	}
}

func or(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}

func handleConvert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	defer r.Body.Close()

	var req convertRequest
	ct := r.Header.Get("Content-Type")
	switch {
	case strings.Contains(ct, "application/json"):
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json: " + err.Error()})
			return
		}
	default:
		b, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		req.Markdown = string(b)
		req.Theme = r.URL.Query().Get("theme")
	}

	theme := converter.Theme(strings.ToLower(strings.TrimSpace(req.Theme)))
	if theme == "" {
		theme = converter.ThemeWeChat
	}
	switch theme {
	case converter.ThemeDefault, converter.ThemeWeChat:
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown theme: " + string(theme)})
		return
	}

	styleID := strings.ToLower(strings.TrimSpace(req.Style))
	if styleID == "" {
		styleID = string(converter.StyleSimple)
	}
	style := converter.StylePack(styleID)

	justify := true
	if req.Justify != nil {
		justify = *req.Justify
	}
	highlight := true
	if req.Highlight != nil {
		highlight = *req.Highlight
	}
	imgCap := true
	if req.ImageCaption != nil {
		imgCap = *req.ImageCaption
	}

	cfg := converter.Config{
		Theme:          theme,
		Title:          req.Title,
		Style:          style,
		PrimaryColor:   req.PrimaryColor,
		TextIndent:     req.TextIndent,
		Justify:        justify,
		ParagraphGap:   req.ParagraphGap,
		FontSize:       req.FontSize,
		LineHeight:     req.LineHeight,
		Highlight:      highlight,
		HighlightTheme: req.HighlightTheme,
		TOC:            req.TOC,
		Footer:         req.Footer,
		ImageCaption:   imgCap,
		Report:         true,
		PreviewWidth:   req.PreviewWidth,
		PreviewShell:   req.PreviewShell,
	}

	pasteCfg := cfg
	pasteCfg.Complete = false
	paste, err := converter.ConvertEx([]byte(req.Markdown), pasteCfg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	prevCfg := cfg
	prevCfg.Complete = true
	preview, err := converter.ConvertEx([]byte(req.Markdown), prevCfg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	htmlOut := string(paste.HTML)
	if req.Complete {
		htmlOut = string(preview.HTML)
	}
	previewHTML := string(preview.Preview)
	if previewHTML == "" {
		previewHTML = string(preview.HTML)
	}

	writeJSON(w, http.StatusOK, convertResponse{
		HTML:      htmlOut,
		Preview:   previewHTML,
		Theme:     string(theme),
		Style:     string(style),
		CharCount: len([]rune(req.Markdown)),
		Report:    paste.Report,
		Health:    paste.Health,
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
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

func handleOutboundIP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Try multiple IPv4-only services
	services := []string{
		"https://api4.ipify.org",
		"https://ipv4.icanhazip.com",
		"https://ipv4.ifconfig.me",
	}

	var ip string
	for _, svc := range services {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		req, _ := http.NewRequestWithContext(ctx, "GET", svc, nil)
		resp, err := http.DefaultClient.Do(req)
		cancel()
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		resp.Body.Close()
		ip = strings.TrimSpace(string(body))
		if ip != "" && resp.StatusCode == http.StatusOK {
			break
		}
	}

	if ip == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "无法获取出口 IP，请检查网络连接"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"ip": ip})
}

var _ = fs.ErrNotExist
