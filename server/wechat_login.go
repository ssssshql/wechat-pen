package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
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
		var l *launcher.Launcher
		if attempt {
			l = launcher.New().Headless(false).UserDataDir(profileDir)
		} else {
			fmt.Println("login: retrying with fresh profile")
			l = launcher.New().Headless(false)
		}
		if _, err := os.Stat(`C:\Program Files\Google\Chrome\Application\chrome.exe`); err == nil {
			l = l.Bin(`C:\Program Files\Google\Chrome\Application\chrome.exe`)
		}
		var cerr error
		u, cerr = l.Launch()
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
		// Delete stale profile and retry
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
	// Try multiple selectors
	selectors := []string{
		"img[src*='qrcode']",
		"img[src*='showqrcode']",
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
			resp, err := http.Get(srcURL)
			if err == nil && resp.StatusCode == 200 {
				data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<19))
				resp.Body.Close()
				if len(data) > 1000 {
					qrcodeB64 = "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
				}
			}
		}
		if qrcodeB64 == "" {
			screenshot, err := qrEl.Screenshot(proto.PageCaptureScreenshotFormatPng, 100)
			if err == nil && len(screenshot) > 0 {
				qrcodeB64 = "data:image/png;base64," + base64.StdEncoding.EncodeToString(screenshot)
			}
		}
	}

	if qrcodeB64 == "" {
		// Fallback: screenshot the whole page
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
	}

	// Start network monitor AFTER login (not during, it breaks QR code loading)
	// It will be enabled in pollLogin when login succeeds

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

						// Start passive network monitor
						go startNetworkMonitor(bs)

						// Navigate to article page to trigger token-rich URLs
						go func() {
							time.Sleep(2 * time.Second)
							_ = bs.page.Navigate("https://mp.weixin.qq.com/cgi-bin/appmsg?t=media/appmsg_edit&action=edit&lang=zh_CN&type=77")
						}()

						// Now safe to enable network monitoring
						go startNetworkMonitor(bs)

						// Navigate to article management to trigger token-rich requests
						go func() {
							time.Sleep(1 * time.Second)
							bs.page.MustNavigate("https://mp.weixin.qq.com/cgi-bin/appmsg?t=media/appmsg_edit&action=edit&lang=zh_CN&type=77")
						}()

						bs.mu.Lock()
						bs.cookies = cookieStr
						bs.mu.Unlock()
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
		writeJSON(w, http.StatusOK, map[string]any{"status": "none"})
		return
	}

	bs.mu.Lock()
	defer bs.mu.Unlock()

	if bs.loggedIn {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "cookies": bs.cookies})
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
	currentCredsMu.Unlock()

	cfg, _ := loadConfigFile()
	cfg.LoginCookie = ""
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

func captureFromCookies(bs *browserSession, cookies []*proto.NetworkCookie) {
	fmt.Printf("capture: scanning %d cookies\n", len(cookies))
	for _, c := range cookies {
		name := strings.ToLower(c.Name)
		fmt.Printf("capture: cookie '%s' = %s\n", name, truncateStr(c.Value, 50))
		if name == "token" || name == "fingerprint" || name == "uin" || name == "key" || strings.Contains(name, "token") || strings.Contains(name, "finger") {
			bs.mu.Lock()
			bs.authParams["cookie_"+name] = c.Value
			bs.mu.Unlock()
			fmt.Printf("capture: extracted cookie %s=%s\n", name, truncateStr(c.Value, 30))
		}
	}
}

// startNetworkMonitor passively captures token/fingerprint from URL params.
func startNetworkMonitor(bs *browserSession) {
	bs.authParams = make(map[string]string)
	browser := bs.browser

	// Scan cookies first
	cookies, err := browser.GetCookies()
	if err == nil {
		for _, c := range cookies {
			name := strings.ToLower(c.Name)
			val := c.Value
			if name == "token" || name == "fingerprint" || name == "uin" || name == "key" {
				bs.authParams["cookie_"+name] = val
				fmt.Printf("capture: from cookie %s=%s\n", name, truncateStr(val, 30))
			}
		}
	}

	// Inject JS to intercept XHR/fetch and report URLs with token+fingerprint
	go func() {
		time.Sleep(2 * time.Second) // wait for page to settle
		_, err := bs.page.Eval(`() => {
			const origFetch = window.fetch;
			window.fetch = function(...args) {
				const url = typeof args[0] === 'string' ? args[0] : args[0].url;
				if (url.includes('token=') || url.includes('fingerprint=')) {
					console.log('__CAPTURE__' + url);
				}
				return origFetch.apply(this, args);
			};
			const origOpen = XMLHttpRequest.prototype.open;
			XMLHttpRequest.prototype.open = function(method, url) {
				if (url.includes('token=') || url.includes('fingerprint=')) {
					console.log('__CAPTURE__' + url);
				}
				return origOpen.apply(this, arguments);
			};
		}`)
		if err != nil {
			fmt.Printf("capture: JS injection failed: %v\n", err)
			return
		}
		fmt.Println("capture: JS injection done, listening for console")

		// Listen for console messages with captured URLs
		// Enable Runtime domain for console events
		bs.page.Call(context.Background(), "Runtime.enable", `{}`, nil)
		go bs.page.EachEvent(func(e *proto.RuntimeConsoleAPICalled) {
			for _, arg := range e.Args {
				v := arg.Value.Str()
				if strings.Contains(v, "__CAPTURE__") {
					prefix := "__CAPTURE__"
					idx := strings.Index(v, prefix)
					urlStr := v[idx+len(prefix):]
					fmt.Printf("capture: JS intercepted %s\n", truncateStr(urlStr, 150))
					if u, err := url.Parse(urlStr); err == nil {
						if token := u.Query().Get("token"); token != "" {
							bs.mu.Lock()
							bs.authParams["token"] = token
							bs.mu.Unlock()
							fmt.Printf("capture: token=%s\n", token)
						}
						if fp := u.Query().Get("fingerprint"); fp != "" {
							bs.mu.Lock()
							bs.authParams["fingerprint"] = fp
							bs.mu.Unlock()
							fmt.Printf("capture: fingerprint=%s\n", fp)
						}
					}
				}
			}
		})()
	}()
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
