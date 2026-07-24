package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// --- Last outbound IP (tracked in handleOutboundIP) ---

var lastOutboundIP string

func setLastOutboundIP(ip string) {
	currentCredsMu.Lock()
	lastOutboundIP = ip
	currentCredsMu.Unlock()
}

func getLastOutboundIP() string {
	currentCredsMu.RLock()
	defer currentCredsMu.RUnlock()
	return lastOutboundIP
}

// --- Whitelist CDP Automation ---

type whitelistSession struct {
	mu       sync.Mutex
	Status   string `json:"status"` // idle | login | logged_in | scanning_admin | done | error
	Flow     string `json:"flow"`   // login | admin | ""
	QRCode   string `json:"qrcode"`
	IP       string `json:"ip"`
	Error    string `json:"error"`
	AppID    string `json:"appid"`
	browser  *rod.Browser
	page     *rod.Page
	cancelCh chan struct{}
}

var currentWhitelist = &whitelistSession{Status: "idle", cancelCh: make(chan struct{}, 1)}

func whitelistProfileDir() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "."
	}
	return filepath.Join(home, ".wechat-pen-chrome-profile-whitelist")
}

// --- HTTP Handlers ---

func handleWhitelistStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	defer r.Body.Close()
	var req struct {
		AppID string `json:"appid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.AppID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "appid is required"})
		return
	}

	outboundIP := getLastOutboundIP()
	if outboundIP == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "出口 IP 未获取，请先获取出口 IP"})
		return
	}

	currentWhitelist.mu.Lock()
	if currentWhitelist.Status == "login" || currentWhitelist.Status == "scanning_admin" {
		currentWhitelist.mu.Unlock()
		writeJSON(w, http.StatusConflict, map[string]string{"error": "白名单配置已在进行中"})
		return
	}
	currentWhitelist.Status = "login"
	currentWhitelist.IP = outboundIP
	currentWhitelist.AppID = req.AppID
	currentWhitelist.QRCode = ""
	currentWhitelist.Error = ""
	currentWhitelist.Flow = ""
	currentWhitelist.cancelCh = make(chan struct{}, 1)
	currentWhitelist.mu.Unlock()

	go runWhitelistAutomation()
	writeJSON(w, http.StatusOK, map[string]string{"status": "login", "appid": req.AppID})
}

func handleWhitelistStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	currentWhitelist.mu.Lock()
	s := currentWhitelist.Status
	qr := currentWhitelist.QRCode
	ip := currentWhitelist.IP
	flow := currentWhitelist.Flow
	errMsg := currentWhitelist.Error
	currentWhitelist.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"status": s, "qrcode": qr, "ip": ip, "flow": flow, "error": errMsg})
}

func handleWhitelistCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cleanupWhitelist()
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func cleanupWhitelist() {
	currentWhitelist.mu.Lock()
	defer currentWhitelist.mu.Unlock()
	select {
	case currentWhitelist.cancelCh <- struct{}{}:
	default:
	}
	if currentWhitelist.browser != nil {
		_ = currentWhitelist.browser.Close()
		currentWhitelist.browser = nil
	}
	currentWhitelist.Status = "idle"
	currentWhitelist.QRCode = ""
	currentWhitelist.Flow = ""
}

// --- CDP Automation ---

func runWhitelistAutomation() {
	currentWhitelist.mu.Lock()
	appID := currentWhitelist.AppID
	ip := currentWhitelist.IP
	cancelCh := currentWhitelist.cancelCh
	currentWhitelist.mu.Unlock()

	targetURL := fmt.Sprintf("https://developers.weixin.qq.com/console/product/mp/%s", appID)

	// Launch Chrome with separate whitelist profile
	profileDir := whitelistProfileDir()
	os.MkdirAll(profileDir, 0o700)

	var l *launcher.Launcher
	l = launcher.New().Headless(false).UserDataDir(profileDir)
	if _, err := os.Stat(`C:\Program Files\Google\Chrome\Application\chrome.exe`); err == nil {
		l = l.Bin(`C:\Program Files\Google\Chrome\Application\chrome.exe`)
	}
	u, cerr := l.Launch()
	if cerr != nil {
		// Delete stale profile and retry
		os.RemoveAll(profileDir)
		l = launcher.New().Headless(false)
		if _, err := os.Stat(`C:\Program Files\Google\Chrome\Application\chrome.exe`); err == nil {
			l = l.Bin(`C:\Program Files\Google\Chrome\Application\chrome.exe`)
		}
		u, cerr = l.Launch()
		if cerr != nil {
			setWhitelistError("启动浏览器失败: " + cerr.Error())
			return
		}
	}

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		setWhitelistError("连接浏览器失败: " + err.Error())
		return
	}

	currentWhitelist.mu.Lock()
	currentWhitelist.browser = browser
	currentWhitelist.mu.Unlock()

	defer func() {
		currentWhitelist.mu.Lock()
		if currentWhitelist.browser != nil {
			_ = currentWhitelist.browser.Close()
			currentWhitelist.browser = nil
		}
		currentWhitelist.mu.Unlock()
	}()

	fmt.Printf("[whitelist] launched chrome, opening %s\n", targetURL)
	// Open target URL — will redirect to /platform/login?redirect=... if not logged in
	page, err := browser.Page(proto.TargetCreateTarget{URL: targetURL})
	if err != nil {
		setWhitelistError("打开页面失败: " + err.Error())
		return
	}
	if err := page.WaitLoad(); err != nil {
		setWhitelistError("页面加载失败: " + err.Error())
		return
	}

	currentWhitelist.mu.Lock()
	currentWhitelist.page = page
	currentWhitelist.mu.Unlock()

	time.Sleep(3 * time.Second)

	if cancelled(cancelCh) {
		return
	}

	// Detect initial state — login page shows QR code, redirect target may show whitelist directly
	info, _ := page.Info()
	currentURL := ""
	if info != nil {
		currentURL = info.URL
	}
	fmt.Printf("[whitelist] page URL after load: %s\n", currentURL)

	state := detectPageState(page)
	fmt.Printf("[whitelist] initial page state: %s\n", state)

	if state == "qrcode" {
		qr := extractQRCodeFromPage(page)
		fmt.Printf("[whitelist] QR code extracted: %v (len=%d)\n", qr != "", len(qr))
		if qr == "" {
			setWhitelistError("无法获取登录二维码")
			return
		}
		currentWhitelist.mu.Lock()
		currentWhitelist.QRCode = qr
		currentWhitelist.Flow = "login"
		currentWhitelist.mu.Unlock()
		fmt.Printf("[whitelist] QR code set, waiting for scan (current status=%s flow=%s)\n", currentWhitelist.Status, currentWhitelist.Flow)

		// Wait for user to scan QR — detect when URL changes away from /platform/login
		if !waitForLoginRedirect(page, 180*time.Second, cancelCh) {
			fmt.Printf("[whitelist] login redirect wait timed out or cancelled\n")
			return
		}

		if cancelled(cancelCh) {
			return
		}

		// Page navigated back to target URL — wait for it to fully load
		time.Sleep(3 * time.Second)
		info2, _ := page.Info()
		if info2 != nil {
			fmt.Printf("[whitelist] after login redirect, URL: %s\n", info2.URL)
		}
		state = detectPageState(page)
		fmt.Printf("[whitelist] state after login: %s\n", state)
	}

	if state != "whitelist" {
		// Try waiting a bit more — page might still be rendering
		time.Sleep(3 * time.Second)
		state = detectPageState(page)
		fmt.Printf("[whitelist] state after extra wait: %s\n", state)
	}

	if state != "whitelist" {
		// Dump page content for debugging
		debugHTML, _ := page.Eval(`document.documentElement.outerHTML.substring(0, 2000)`)
		if debugHTML != nil {
			fmt.Printf("[whitelist] page HTML (first 2000 chars): %s\n", debugHTML.Value.Str())
		}
		setWhitelistError("登录后未能自动跳转到 IP 白名单页面，请确认 AppID 正确")
		return
	}

	// User is logged in, now click edit and fill IP
	currentWhitelist.mu.Lock()
	currentWhitelist.Status = "logged_in"
	currentWhitelist.QRCode = ""
	currentWhitelist.Flow = ""
	currentWhitelist.mu.Unlock()

	if cancelled(cancelCh) {
		return
	}

	time.Sleep(1 * time.Second)
	clickEditButton(page)
	time.Sleep(2 * time.Second)

	fillIPTextarea(page, ip)
	time.Sleep(1 * time.Second)

	clickConfirmButton(page)
	time.Sleep(2 * time.Second)

	// Check for admin confirmation QR
	adminQR := extractQRCodeFromPage(page)
	if adminQR != "" {
		currentWhitelist.mu.Lock()
		currentWhitelist.QRCode = adminQR
		currentWhitelist.Flow = "admin"
		currentWhitelist.Status = "scanning_admin"
		currentWhitelist.mu.Unlock()

		if !waitQRCodeGone(page, 180*time.Second, cancelCh) {
			return
		}
		time.Sleep(2 * time.Second)
	}

	// Done
	currentWhitelist.mu.Lock()
	currentWhitelist.Status = "done"
	currentWhitelist.QRCode = ""
	currentWhitelist.Flow = ""
	currentWhitelist.mu.Unlock()
}

func setWhitelistError(msg string) {
	currentWhitelist.mu.Lock()
	currentWhitelist.Status = "error"
	currentWhitelist.Error = msg
	currentWhitelist.QRCode = ""
	currentWhitelist.Flow = ""
	currentWhitelist.mu.Unlock()
}

func cancelled(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

// waitForLoginRedirect waits until the page navigates away from the login URL,
// which means the user scanned the QR code and logged in.
func waitForLoginRedirect(page *rod.Page, timeout time.Duration, cancelCh chan struct{}) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cancelled(cancelCh) {
			return false
		}
		time.Sleep(1 * time.Second)
		info, err := page.Info()
		if err != nil {
			fmt.Printf("[whitelist] waitForRedirect: page.Info error: %v\n", err)
			continue
		}
		fmt.Printf("[whitelist] waitForRedirect: URL = %s\n", info.URL)
		if !strings.Contains(info.URL, "/platform/login") {
			fmt.Printf("[whitelist] login redirect detected, now at: %s\n", info.URL)
			return true
		}
	}
	fmt.Printf("[whitelist] waitForRedirect: timed out\n")
	return false
}

func waitQRCodeGone(page *rod.Page, timeout time.Duration, cancelCh chan struct{}) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cancelled(cancelCh) {
			return false
		}
		time.Sleep(1 * time.Second)
		state := detectPageState(page)
		if state != "qrcode" {
			return true
		}
	}
	return true
}

func detectPageState(page *rod.Page) string {
	const js = `(function() {
		if (document.querySelector('img[src*="qrcode"], img[alt="qrcode"], img[alt*="qrcode"], img[src*="qrcode"]'))
			return 'qrcode';
		var ps = document.querySelectorAll('p');
		for (var i = 0; i < ps.length; i++) {
			if (ps[i].textContent.indexOf('白名单') !== -1)
				return 'whitelist';
		}
		return 'unknown';
	})()`
	result, err := page.Eval(js)
	if err != nil {
		fmt.Printf("[whitelist] detectPageState eval error: %v\n", err)
		return "unknown"
	}
	state := result.Value.Str()
	return state
}

func extractQRCodeFromPage(page *rod.Page) string {
	// Try img[alt="qrcode"] first (exact match)
	selectors := []string{
		`img[alt="qrcode"]`,
		`img[alt*="qrcode"]`,
		`img[src*="qrcode"]`,
	}
	for _, sel := range selectors {
		el, err := page.Timeout(3 * time.Second).Element(sel)
		if err != nil || el == nil {
			fmt.Printf("[whitelist] QR selector %q: not found\n", sel)
			continue
		}
		fmt.Printf("[whitelist] QR selector %q: found\n", sel)
		src, _ := el.Attribute("src")
		if src != nil && strings.HasPrefix(*src, "data:image") {
			parts := strings.SplitN(*src, ",", 2)
			if len(parts) == 2 && parts[1] != "" {
				return "data:image/png;base64," + parts[1]
			}
		}
		// Screenshot the QR element
		screenshot, err := el.Screenshot(proto.PageCaptureScreenshotFormatPng, 100)
		if err == nil && len(screenshot) > 100 {
			return "data:image/png;base64," + base64.StdEncoding.EncodeToString(screenshot)
		}
	}
	return ""
}

func clickEditButton(page *rod.Page) {
	// Find the whitelist row by looking for p with "白名单" text, then find the sibling content link
	const js = `(function() {
		var ps = document.querySelectorAll('p');
		for (var i = 0; i < ps.length; i++) {
			if (ps[i].textContent.indexOf('白名单') !== -1) {
				var row = ps[i].closest('.deploy-info-item') || ps[i].parentElement;
				if (!row) continue;
				var link = row.querySelector('a');
				if (link) { link.click(); return true; }
			}
		}
		return false;
	})()`
	var ok bool
	result, err := page.Eval(js)
	if err == nil {
		ok = result.Value.Bool()
	}
	if !ok {
		// Fallback: try xpath
		el, err := page.Timeout(5 * time.Second).Element(`xpath/.//p[contains(.,'白名单')]/..//a`)
		if err == nil {
			el.MustClick()
		}
	}
}

func fillIPTextarea(page *rod.Page, ip string) {
	el, err := page.Timeout(5 * time.Second).Element(`textarea.weui-desktop-form__textarea`)
	if err != nil {
		el, err = page.Timeout(5 * time.Second).Element(`textarea[placeholder*="多个 IP"]`)
		if err != nil {
			return
		}
	}
	el.MustSelectAllText()
	el.MustInput(ip)
}

func clickConfirmButton(page *rod.Page) {
	el, err := page.Timeout(5 * time.Second).Element(`button.weui-desktop-btn_primary`)
	if err != nil {
		return
	}
	el.MustClick()
}
