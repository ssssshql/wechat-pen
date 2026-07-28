package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

type browserSession struct {
	mu        sync.Mutex
	browser   *rod.Browser
	page      *rod.Page
	loggedIn  bool
	cookies   string
	err       string
	cancel    chan struct{}
	authParams map[string]string // captured token, fingerprint, etc.
	qrB64     string            // current QR base64 data URI
	qrStatus  int               // 0=waiting, 3=expired, 4=scanned
	message   string            // granular progress text

	// SSE event broadcasting
	subscribers []chan string // each SSE client gets its own channel
}

func (bs *browserSession) broadcastState() {
	bs.mu.Lock()
	status := "waiting"
	if bs.loggedIn {
		status = "ok"
	} else if bs.err != "" {
		status = "error"
	}
	data := map[string]any{
		"status":     status,
		"qr_status":  bs.qrStatus,
		"message":    bs.message,
	}
	if bs.loggedIn {
		data["cookies"] = bs.cookies
		if bs.authParams != nil {
			if t, ok := bs.authParams["token"]; ok {
				data["token"] = t
			}
			if fp, ok := bs.authParams["fingerprint"]; ok {
				data["fingerprint"] = fp
			}
		}
	}
	if bs.err != "" {
		data["error"] = bs.err
	}
	bs.mu.Unlock()

	b, _ := json.Marshal(data)
	bs.broadcast("event: state\ndata: " + string(b) + "\n\n")
}

func (bs *browserSession) addSubscriber() chan string {
	ch := make(chan string, 8)
	bs.mu.Lock()
	bs.subscribers = append(bs.subscribers, ch)
	bs.mu.Unlock()
	return ch
}

func (bs *browserSession) removeSubscriber(ch chan string) {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	for i, s := range bs.subscribers {
		if s == ch {
			bs.subscribers = append(bs.subscribers[:i], bs.subscribers[i+1:]...)
			break
		}
	}
	close(ch)
}

func (bs *browserSession) broadcast(event string) {
	bs.mu.Lock()
	subs := make([]chan string, len(bs.subscribers))
	copy(subs, bs.subscribers)
	bs.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- event:
		default:
			// drop if subscriber is slow (buffer full)
		}
	}
}

var loginSession = struct {
	mu sync.Mutex
	bs *browserSession
}{}

func handleLoginStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// If already have a stored cookie, skip login
	if cookie := GetLoginCookie(); cookie != "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"status":  "already_logged_in",
		})
		return
	}

	// Clean up existing session
	cancelLoginSession()

	// Use persistent profile so login state survives restarts
	home, _ := os.UserHomeDir()
	profileDir := filepath.Join(home, ".wechat-pen-chrome-profile")
	os.MkdirAll(profileDir, 0700)

	// Launch browser (try persistent profile first, fallback to fresh)
	var u string
	for _, attempt := range []bool{true, false} {
		dir := profileDir
		if !attempt {
			fmt.Println("login: retrying with fresh profile")
			dir = ""
		}
		var cerr error
		u, cerr = launchChrome(dir)
		if cerr == nil {
			break
		}
		if !attempt {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "启动浏览器失败，请关闭已打开的 Chrome 窗口后重试: " + cerr.Error(),
			})
			return
		}
		fmt.Printf("login: launch with profile failed: %v\n", cerr)
		os.RemoveAll(profileDir)
	}

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "连接浏览器失败: " + err.Error(),
		})
		return
	}

	// Create a page and navigate to MP login
	page, err := browser.Page(proto.TargetCreateTarget{URL: "https://mp.weixin.qq.com/"})
	if err != nil {
		browser.Close()
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "打开页面失败: " + err.Error(),
		})
		return
	}

	// Wait for the page to load
	if err := page.WaitLoad(); err != nil {
		page.Close()
		browser.Close()
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "页面加载失败: " + err.Error(),
		})
		return
	}

	// Give JS time to render the QR code
	time.Sleep(2 * time.Second)

	// Check if browser is already logged in (persistent profile kept session)
	info, _ := page.Info()
	currentURL := info.URL
	fmt.Printf("login: current URL = %s\n", currentURL)
	if strings.Contains(currentURL, "mp.weixin.qq.com") && (strings.Contains(currentURL, "/cgi-bin/") || strings.Contains(currentURL, "/home")) {
		fmt.Println("login: browser already logged in, extracting cookies")
		cookies, err := browser.GetCookies()
		if err == nil {
			var pairs []string
			for _, c := range cookies {
				pairs = append(pairs, fmt.Sprintf("%s=%s", c.Name, c.Value))
			}
			cookieStr := strings.Join(pairs, "; ")
			storeCookie(cookieStr)
		}
		page.Close()
		browser.Close()
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     true,
			"status": "already_logged_in",
		})
		return
	}

	fmt.Println("login: on login page, looking for QR code")
	time.Sleep(3 * time.Second)

	// Try to find the QR code element and screenshot it
	var qrcodeB64 string

	// The QR code is typically an img with class or inside a specific div
	// Try multiple selectors — exact class match first, then broader fallbacks
	selectors := []string{
		"img.login__type__container__scan__qrcode",
		"img[src*='showqrcode']",
		"img[src*='qrcode']",
		".qrcode img",
		"#qrcode img",
		".login_qrcode img",
		".js_qrcode img",
		"img.qrcode",
		".wr_code img",
		".weui-desktop-qr__img",
	}

	var qrEl *rod.Element
	for _, sel := range selectors {
		el, err := page.Element(sel)
		if err == nil && el != nil {
			qrEl = el
			fmt.Printf("login: QR element found with selector %q\n", sel)
			break
		}
	}

	if qrEl != nil {
		src, _ := qrEl.Attribute("src")
		if src != nil && *src != "" {
			srcURL := *src
			if strings.HasPrefix(srcURL, "//") {
				srcURL = "https:" + srcURL
			} else if strings.HasPrefix(srcURL, "/") {
				srcURL = "https://mp.weixin.qq.com" + srcURL
			}
			// Extract the QR image via canvas (already loaded in the <img> element)
			fmt.Printf("login: extracting QR via canvas\n")
			js := `function() {
				try {
					var img = document.querySelector('img.login__type__container__scan__qrcode');
					if (!img) return 'err:no img';
					if (!img.naturalWidth) return 'err:img not loaded';
					var c = document.createElement('canvas');
					c.width = img.naturalWidth;
					c.height = img.naturalHeight;
					c.getContext('2d').drawImage(img, 0, 0);
					var d = c.toDataURL('image/png');
					var idx = d.indexOf(',');
					return idx > 0 ? d.substring(idx + 1) : 'err:no data';
				} catch(e) { return 'err:' + e.message; }
			}`
			result, err := page.Evaluate(rod.Eval(js))
			if err == nil && result != nil {
				val := result.Value.Str()
				fmt.Printf("login: browser fetch result: %s (len=%d)\n", val[:min(len(val), 50)], len(val))
				if !strings.HasPrefix(val, "err:") && len(val) > 200 {
					qrcodeB64 = "data:image/png;base64," + val
					fmt.Printf("login: browser fetch ok\n")
				} else {
					fmt.Printf("login: browser fetch failed: %s\n", val)
				}
			} else {
				fmt.Printf("login: browser fetch error: %v\n", err)
			}
		}
		if qrcodeB64 == "" {
			fmt.Println("login: falling back to element screenshot")
			screenshot, err := qrEl.Screenshot(proto.PageCaptureScreenshotFormatPng, 100)
			if err == nil && len(screenshot) > 0 {
				fmt.Printf("login: screenshot %d bytes\n", len(screenshot))
				qrcodeB64 = "data:image/png;base64," + base64.StdEncoding.EncodeToString(screenshot)
			}
		}
	}

	if qrcodeB64 == "" {
		// Fallback: screenshot the whole page
		fmt.Println("login: falling back to full page screenshot")
		screenshot, err := page.Screenshot(true, &proto.PageCaptureScreenshot{
			Format: proto.PageCaptureScreenshotFormatPng,
			Quality: func() *int { v := 80; return &v }(),
		})
		if err == nil && len(screenshot) > 0 {
			qrcodeB64 = "data:image/png;base64," + base64.StdEncoding.EncodeToString(screenshot)
		}
	}

	if qrcodeB64 == "" {
		page.MustClose()
		browser.MustClose()
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "无法获取登录二维码，请重试"})
		return
	}

	bs := &browserSession{
		browser: browser,
		page:    page,
		cancel:  make(chan struct{}),
		qrB64:   qrcodeB64,
		message: "二维码已获取，等待扫码",
	}

	// Enable CDP Network early — BEFORE login — so we capture the first post-login API calls
	// that contain token+fingerprint in URL. Network.enable is a passive observer.
	_ = proto.NetworkEnable{}.Call(bs.page)

	// Listen for scanloginqrcode?action=ask responses to track QR status
	go bs.page.EachEvent(func(e *proto.NetworkResponseReceived) {
		if !strings.Contains(e.Response.URL, "scanloginqrcode") || !strings.Contains(e.Response.URL, "action=ask") {
			return
		}
		body, err := proto.NetworkGetResponseBody{RequestID: e.RequestID}.Call(bs.page)
		if err != nil {
			return
		}
		var resp struct {
			Status int `json:"status"`
		}
		if json.Unmarshal([]byte(body.Body), &resp) != nil {
			return
		}
		fmt.Printf("login: QR scan status: %d\n", resp.Status)

		bs.mu.Lock()
		bs.qrStatus = resp.Status
		switch resp.Status {
		case 0:
			bs.message = "等待扫码"
		case 3:
			bs.message = "二维码已失效，正在刷新..."
		case 4:
			bs.message = "已扫码，请在手机上确认授权"
		}
		bs.mu.Unlock()
		bs.broadcastState()

		// If expired (3), click refresh and re-extract QR
		if resp.Status == 3 {
			go refreshLoginQR(bs)
		}
	})()

	// Start listening for network events — capture token+fingerprint from URL params
	go bs.page.EachEvent(func(e *proto.NetworkRequestWillBeSent) {
		rawURL := e.Request.URL
		if !strings.Contains(rawURL, "token=") && !strings.Contains(rawURL, "fingerprint=") {
			return
		}
		u, err := url.Parse(rawURL)
		if err != nil {
			return
		}
		bs.mu.Lock()
		if bs.authParams == nil {
			bs.authParams = make(map[string]string)
		}
		if token := u.Query().Get("token"); token != "" {
			bs.authParams["token"] = token
		}
		if fp := u.Query().Get("fingerprint"); fp != "" {
			bs.authParams["fingerprint"] = fp
		}
		bs.mu.Unlock()
	})()

	// Record the initial login URL so we can detect change
	initialURL := ""
	if info, err := page.Info(); err == nil {
		initialURL = info.URL
	}

	// Background poll for login success
	go pollLogin(bs, initialURL)

	loginSession.mu.Lock()
	loginSession.bs = bs
	loginSession.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"qrcode_b64": qrcodeB64,
	})
}

func pollLogin(bs *browserSession, initialURL string) {
	// Wait a minimum time before polling (user needs time to scan)
	time.Sleep(5 * time.Second)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-bs.cancel:
			return
		case <-ticker.C:
			bs.mu.Lock()
			if bs.loggedIn {
				bs.mu.Unlock()
				return
			}
			bs.mu.Unlock()

			info, err := bs.page.Info()
			if err != nil {
				bs.mu.Lock()
				bs.err = "页面异常"
				bs.mu.Unlock()
				errJSON, _ := json.Marshal(map[string]string{"error": "页面异常"})
				bs.broadcast("event: login_error\ndata: " + string(errJSON) + "\n\n")
				return
			}

			url := info.URL

			// Check if URL has changed from the initial login page AND we're on a dashboard page
			if url != initialURL && !strings.Contains(url, "login") && !strings.Contains(url, "showqrcode") && !strings.Contains(url, "qrconnect") {
				// Verify we're actually on the MP dashboard, not some intermediate redirect
				if strings.Contains(url, "mp.weixin.qq.com") &&
					(strings.Contains(url, "/cgi-bin/") || strings.Contains(url, "home") || strings.Contains(url, "appmsg")) {
					bs.mu.Lock()
					bs.loggedIn = true
					bs.mu.Unlock()

					// Extract cookies
					cookies, err := bs.browser.GetCookies()
					if err == nil {
						var pairs []string
						for _, c := range cookies {
							pairs = append(pairs, fmt.Sprintf("%s=%s", c.Name, c.Value))
						}
						cookieStr := strings.Join(pairs, "; ")

						storeCookie(cookieStr)

						bs.mu.Lock()
						bs.cookies = cookieStr
						bs.mu.Unlock()

						// Broadcast status ok
						statusJSON, _ := json.Marshal(map[string]string{"status": "ok"})
						bs.broadcast("event: status\ndata: " + string(statusJSON) + "\n\n")

						// Extract token/fingerprint from the page URL after navigation.
						// No JS injection, no CDP Network domain — zero detection surface.
						go extractAuthParams(bs)
					}
					return
				}
			}
		}
	}
}

func handleLoginStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	loginSession.mu.Lock()
	bs := loginSession.bs
	loginSession.mu.Unlock()

	if bs == nil {
		// Browser session ended, but we may have persisted cookie/token/fingerprint
		if cookie := GetLoginCookie(); cookie != "" {
			currentCredsMu.RLock()
			resp := map[string]any{"status": "ok", "cookies": cookie}
			if currentCreds.Token != "" {
				resp["token"] = currentCreds.Token
			}
			if currentCreds.Fingerprint != "" {
				resp["fingerprint"] = currentCreds.Fingerprint
			}
			currentCredsMu.RUnlock()
			writeJSON(w, http.StatusOK, resp)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "none"})
		return
	}

	bs.mu.Lock()
	defer bs.mu.Unlock()

	if bs.loggedIn {
		resp := map[string]any{"status": "ok", "cookies": bs.cookies}
		if bs.authParams != nil {
			if t, ok := bs.authParams["token"]; ok {
				resp["token"] = t
			}
			if fp, ok := bs.authParams["fingerprint"]; ok {
				resp["fingerprint"] = fp
			}
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if bs.err != "" {
		writeJSON(w, http.StatusOK, map[string]any{"status": "error", "error": bs.err})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "waiting"})
}

func handleLoginCancel(w http.ResponseWriter, r *http.Request) {
	cancelLoginSession()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func handleLoginLogout(w http.ResponseWriter, r *http.Request) {
	fmt.Println("login: logout requested")
	cancelLoginSession()

	// Delete persistent profile to clear all browser cookies
	home, _ := os.UserHomeDir()
	profileDir := filepath.Join(home, ".wechat-pen-chrome-profile")
	os.RemoveAll(profileDir)
	fmt.Println("login: browser profile deleted")

	currentCredsMu.Lock()
	currentCreds.LoginCookie = ""
	currentCreds.Token = ""
	currentCreds.Fingerprint = ""
	currentCredsMu.Unlock()

	cfg, _ := loadConfigFile()
	cfg.LoginCookie = ""
	cfg.Token = ""
	cfg.Fingerprint = ""
	saveConfigFile(cfg)

	fmt.Println("login: logout complete")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func cancelLoginSession() {
	loginSession.mu.Lock()
	bs := loginSession.bs
	loginSession.bs = nil
	loginSession.mu.Unlock()

	if bs != nil {
		// Broadcast cancel before closing
		cancelJSON, _ := json.Marshal(map[string]string{"status": "cancelled"})
		bs.broadcast("event: cancel\ndata: " + string(cancelJSON) + "\n\n")

		close(bs.cancel)
		_ = bs.page.Close()
		_ = bs.browser.Close()
	}
}

// GetLoginCookie returns the stored login cookie from the session.
func GetLoginCookie() string {
	currentCredsMu.RLock()
	defer currentCredsMu.RUnlock()
	return currentCreds.LoginCookie
}

func storeCookie(cookieStr string) {
	currentCredsMu.Lock()
	currentCreds.LoginCookie = cookieStr
	currentCredsMu.Unlock()

	cfg, _ := loadConfigFile()
	cfg.LoginCookie = cookieStr
	saveConfigFile(cfg)
	fmt.Printf("login: cookie stored (%d chars)\n", len(cookieStr))
}

// extractAuthParams saves token/fingerprint and persists to config.
func extractAuthParams(bs *browserSession) {
	// Read token from page URL if not yet captured from network events
	if info, err := bs.page.Info(); err == nil {
		if u, _ := url.Parse(info.URL); u != nil {
			bs.mu.Lock()
			if bs.authParams == nil {
				bs.authParams = make(map[string]string)
			}
			if token := u.Query().Get("token"); token != "" && bs.authParams["token"] == "" {
				bs.authParams["token"] = token
			}
			if fp := u.Query().Get("fingerprint"); fp != "" && bs.authParams["fingerprint"] == "" {
				bs.authParams["fingerprint"] = fp
			}
			bs.mu.Unlock()
		}
	}

	bs.mu.Lock()
	token := bs.authParams["token"]
	fp := bs.authParams["fingerprint"]
	bs.mu.Unlock()

	// Persist to in-memory creds and config file
	currentCredsMu.Lock()
	currentCreds.Token = token
	currentCreds.Fingerprint = fp
	currentCredsMu.Unlock()

	cfg, _ := loadConfigFile()
	cfg.Token = token
	cfg.Fingerprint = fp
	saveConfigFile(cfg)

	fmt.Printf("login: captured token=%s fingerprint=%s (persisted)\n", token, fp)

	// Broadcast credentials to SSE subscribers
	bs.mu.Lock()
	cookieStr := bs.cookies
	bs.mu.Unlock()
	credsJSON, _ := json.Marshal(map[string]string{
		"cookies":     cookieStr,
		"token":       token,
		"fingerprint": fp,
	})
	bs.broadcast("event: credentials\ndata: " + string(credsJSON) + "\n\n")
}

// refreshLoginQR clicks the refresh link on mp.weixin.qq.com login page and re-extracts QR.
func refreshLoginQR(bs *browserSession) {
	fmt.Println("login: clicking refresh button...")
	clickResult, _ := bs.page.Evaluate(rod.Eval(`function() {
		var links = document.querySelectorAll('a, span');
		for (var i = 0; i < links.length; i++) {
			var t = links[i].textContent.trim();
			if (t === '刷新' || t === '点击刷新' || t === 'Refresh') {
				links[i].click();
				return 'clicked: ' + t;
			}
		}
		return 'not found';
	}`))
	if clickResult != nil {
		fmt.Printf("login: refresh click: %s\n", clickResult.Value.Str())
	}
	time.Sleep(2 * time.Second)

	// Re-extract QR via canvas
	js := `function() {
		try {
			var img = document.querySelector('img.login__type__container__scan__qrcode');
			if (!img) return 'err:no img';
			if (!img.naturalWidth) return 'err:img not loaded';
			var c = document.createElement('canvas');
			c.width = img.naturalWidth;
			c.height = img.naturalHeight;
			c.getContext('2d').drawImage(img, 0, 0);
			var d = c.toDataURL('image/png');
			var idx = d.indexOf(',');
			return idx > 0 ? d.substring(idx + 1) : 'err:no data';
		} catch(e) { return 'err:' + e.message; }
	}`
	result, err := bs.page.Evaluate(rod.Eval(js))
	if err == nil && result != nil {
		val := result.Value.Str()
		if !strings.HasPrefix(val, "err:") && len(val) > 200 {
			newQR := "data:image/png;base64," + val
			bs.mu.Lock()
			bs.qrB64 = newQR
			bs.qrStatus = 0
			bs.message = "二维码已刷新，等待扫码"
			// Broadcast new QR via a special event
			qrJSON, _ := json.Marshal(map[string]any{
				"qrcode":    newQR,
				"qr_status": 0,
				"message":   "二维码已刷新，等待扫码",
			})
			bs.mu.Unlock()
			bs.broadcast("event: qrcode\ndata: " + string(qrJSON) + "\n\n")
			fmt.Println("login: QR refreshed ok")
		}
	}
}

// PublishDraftWithCookie publishes a draft using the stored login cookie.
func PublishDraftWithCookie(mediaID string) error {
	cookie := GetLoginCookie()
	if cookie == "" {
		return fmt.Errorf("未登录，请先在设置中扫码登录")
	}

	// Use the cookie to call the internal publish API
	// The web interface uses this endpoint to publish drafts
	client := &http.Client{Timeout: 15 * time.Second}

	body := fmt.Sprintf(`{"media_id":"%s"}`, mediaID)
	req, _ := http.NewRequest("POST",
		"https://mp.weixin.qq.com/cgi-bin/free_publish?action=publish&lang=zh_CN",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", cookie)
	req.Header.Set("Referer", "https://mp.weixin.qq.com/")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求发布接口失败: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("发布失败，状态码: %d（Cookie 可能已过期）", resp.StatusCode)
	}

	return nil
}

func handleLoginParams(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	loginSession.mu.Lock()
	bs := loginSession.bs
	loginSession.mu.Unlock()

	resp := map[string]any{
		"has_cookie": GetLoginCookie() != "",
	}

	if bs != nil {
		bs.mu.Lock()
		if bs.authParams != nil && len(bs.authParams) > 0 {
			// Return a copy
			params := make(map[string]string, len(bs.authParams))
			for k, v := range bs.authParams {
				params[k] = v
			}
			resp["params"] = params
		}
		bs.mu.Unlock()
	}

	writeJSON(w, http.StatusOK, resp)
}

func handleLoginEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Remove write deadline for this long-lived SSE connection
	rc := http.NewResponseController(w)
	rc.SetWriteDeadline(time.Time{})

	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	loginSession.mu.Lock()
	bs := loginSession.bs
	loginSession.mu.Unlock()

	// No active session — send stored credentials if available, then wait
	if bs == nil {
		if cookie := GetLoginCookie(); cookie != "" {
			currentCredsMu.RLock()
			data := map[string]any{"cookies": cookie}
			if currentCreds.Token != "" {
				data["token"] = currentCreds.Token
			}
			if currentCreds.Fingerprint != "" {
				data["fingerprint"] = currentCreds.Fingerprint
			}
			currentCredsMu.RUnlock()
			jsonBytes, _ := json.Marshal(data)
			fmt.Fprintf(w, "event: credentials\ndata: %s\n\n", jsonBytes)
			flusher.Flush()
		}
		<-r.Context().Done()
		return
	}

	// Subscribe to active session events
	ch := bs.addSubscriber()
	defer bs.removeSubscriber(ch)

	// Send initial state snapshot
	bs.mu.Lock()
	switch {
	case bs.loggedIn:
		data := map[string]any{"status": "ok", "cookies": bs.cookies}
		if bs.authParams != nil {
			if t, ok := bs.authParams["token"]; ok {
				data["token"] = t
			}
			if fp, ok := bs.authParams["fingerprint"]; ok {
				data["fingerprint"] = fp
			}
		}
		jsonBytes, _ := json.Marshal(data)
		fmt.Fprintf(w, "event: credentials\ndata: %s\n\n", jsonBytes)
	case bs.err != "":
		jsonBytes, _ := json.Marshal(map[string]string{"error": bs.err})
		fmt.Fprintf(w, "event: login_error\ndata: %s\n\n", jsonBytes)
	default:
		data := map[string]any{"status": "waiting", "qr_status": bs.qrStatus, "message": bs.message}
		if bs.qrB64 != "" {
			data["qrcode"] = bs.qrB64
		}
		jsonBytes, _ := json.Marshal(data)
		fmt.Fprintf(w, "event: state\ndata: %s\n\n", jsonBytes)
	}
	bs.mu.Unlock()
	flusher.Flush()

	// Stream events until disconnect or session ends
	for {
		select {
		case <-r.Context().Done():
			return
		case <-bs.cancel:
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprint(w, event)
			flusher.Flush()
		}
	}
}
