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
	QRStatus int    `json:"qr_status"` // 1=waiting, 2=scanned, 7=expired
	IP       string `json:"ip"`
	Error    string `json:"error"`
	AppID    string `json:"appid"`
	Message  string `json:"message"` // granular progress text
	browser  *rod.Browser
	page     *rod.Page
	cancelCh chan struct{}

	// SSE event broadcasting
	subscribers []chan string
}

func (ws *whitelistSession) addSubscriber() chan string {
	ch := make(chan string, 8)
	ws.mu.Lock()
	ws.subscribers = append(ws.subscribers, ch)
	ws.mu.Unlock()
	return ch
}

func (ws *whitelistSession) removeSubscriber(ch chan string) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	for i, s := range ws.subscribers {
		if s == ch {
			ws.subscribers = append(ws.subscribers[:i], ws.subscribers[i+1:]...)
			break
		}
	}
	close(ch)
}

func (ws *whitelistSession) broadcast(event string) {
	ws.mu.Lock()
	subs := make([]chan string, len(ws.subscribers))
	copy(subs, ws.subscribers)
	ws.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- event:
		default:
		}
	}
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
		IP    string `json:"ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.AppID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "appid is required"})
		return
	}

	outboundIP := strings.TrimSpace(req.IP)
	if outboundIP == "" {
		outboundIP = getLastOutboundIP()
	}
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
	resetWhitelistState()
	currentWhitelist.Status = "login"
	currentWhitelist.IP = outboundIP
	currentWhitelist.AppID = req.AppID
	currentWhitelist.mu.Unlock()

	currentWhitelist.broadcastState()

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

func handleWhitelistEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rc := http.NewResponseController(w)
	rc.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch := currentWhitelist.addSubscriber()
	defer currentWhitelist.removeSubscriber(ch)

	// Send initial state snapshot
	fmt.Fprintf(w, "event: state\ndata: %s\n\n", currentWhitelist.currentStateJSON())
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
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

func cleanupWhitelist() {
	currentWhitelist.mu.Lock()
	defer currentWhitelist.mu.Unlock()
	select {
	case currentWhitelist.cancelCh <- struct{}{}:
	default:
	}
	// Don't close browser — keep it alive so the developer platform
	// login session persists across multiple whitelist operations.
	currentWhitelist.Status = "idle"
	currentWhitelist.QRCode = ""
	currentWhitelist.Flow = ""
	go currentWhitelist.broadcastState()
}

func resetWhitelistState() {
	// Must be called with currentWhitelist.mu held
	// Cancel any running automation goroutine first
	select {
	case currentWhitelist.cancelCh <- struct{}{}:
	default:
	}
	currentWhitelist.Status = "idle"
	currentWhitelist.QRCode = ""
	currentWhitelist.Flow = ""
	currentWhitelist.Error = ""
	currentWhitelist.Message = ""
	currentWhitelist.QRStatus = 0
	currentWhitelist.cancelCh = make(chan struct{}, 1)
}

// --- CDP Automation ---

func runWhitelistAutomation() {
	// Recover from rod Must* panics so the goroutine doesn't crash silently
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("[whitelist] panic recovered: %v\n", r)
			setWhitelistError(fmt.Sprintf("操作失败: %v", r))
		}
	}()

	currentWhitelist.mu.Lock()
	appID := currentWhitelist.AppID
	ip := currentWhitelist.IP
	cancelCh := currentWhitelist.cancelCh
	currentWhitelist.mu.Unlock()

	targetURL := fmt.Sprintf("https://developers.weixin.qq.com/console/product/mp/%s", appID)

	// Launch Chrome with separate whitelist profile

	setWhitelistMsg("正在启动浏览器...")
	var browser *rod.Browser
	var page *rod.Page

	// Reuse existing browser if still alive
	currentWhitelist.mu.Lock()
	existingBrowser := currentWhitelist.browser
	currentWhitelist.mu.Unlock()

	if existingBrowser != nil {
		if pages, err := existingBrowser.Pages(); err == nil && len(pages) > 0 {
			browser = existingBrowser
			fmt.Printf("[whitelist] reusing existing browser with %d pages\n", len(pages))
		} else {
			existingBrowser.Close()
			currentWhitelist.mu.Lock()
			currentWhitelist.browser = nil
			currentWhitelist.mu.Unlock()
		}
	}

		if browser == nil {
			profileDir := whitelistProfileDir()
			u, cerr := launchChrome(profileDir)
			if cerr != nil {
				u, cerr = launchChrome("")
				if cerr != nil {
					setWhitelistError("启动浏览器失败: " + cerr.Error())
					return
				}
			}

			browser = rod.New().ControlURL(u)
			if err := browser.Connect(); err != nil {
				setWhitelistError("连接浏览器失败: " + err.Error())
				return
			}

			currentWhitelist.mu.Lock()
			currentWhitelist.browser = browser
			currentWhitelist.mu.Unlock()
		}

	defer func() {
		// Never close the browser — keep it alive so the developer platform
		// login session (cookies in the profile directory) persists across
		// multiple whitelist operations.
		currentWhitelist.mu.Lock()
		currentWhitelist.page = nil
		currentWhitelist.mu.Unlock()
	}()

	fmt.Printf("[whitelist] opening %s\n", targetURL)
	setWhitelistMsg("正在打开开发者平台...")

	// Try to reuse an existing tab that's already on the target domain
	if existingBrowser != nil {
		if pages, err := browser.Pages(); err == nil {
			for _, p := range pages {
				if info, err := p.Info(); err == nil && strings.Contains(info.URL, "developers.weixin.qq.com") {
					page = p
					_ = page.Navigate(targetURL)
					break
				}
			}
		}
	}
	if page == nil {
		pages, _ := browser.Pages()
		if len(pages) > 0 {
			page = pages[0]
			_ = page.Navigate(targetURL)
		} else {
			page = browser.MustPage(targetURL)
		}
	}
	_ = page.WaitLoad()

	currentWhitelist.mu.Lock()
	currentWhitelist.page = page
	currentWhitelist.mu.Unlock()

	time.Sleep(3 * time.Second)
	if cancelled(cancelCh) {
		return
	}

	info, _ := page.Info()
	if info != nil {
		fmt.Printf("[whitelist] initial URL: %s\n", info.URL)
	}

	// --- Step 1: Handle login if needed ---
	if isLoginPage(page) {
		fmt.Printf("[whitelist] on login page, extracting QR\n")
		if !handleLoginQR(page, cancelCh) {
			return // cancelled or error (already set)
		}
		fmt.Printf("[whitelist] login done, looking for target page across all tabs...\n")
		setWhitelistMsg("登录成功，正在跳转到目标页面...")

		time.Sleep(2 * time.Second)
		if tp := findTargetPage(browser, targetURL); tp != nil {
			page = tp
			currentWhitelist.mu.Lock()
			currentWhitelist.page = page
			currentWhitelist.mu.Unlock()
		}
		fmt.Printf("[whitelist] login complete, waiting for content...\n")
	}

	// --- Step 2: Wait for whitelist content to appear ---
	setWhitelistMsg("正在等待页面加载...")
	if !waitForWhitelistContent(page, 30*time.Second, cancelCh) {
		info, _ := page.Info()
		url := ""
		if info != nil {
			url = info.URL
		}
		debugHTML, _ := page.Evaluate(rod.Eval(`function(){ var el = document.querySelector('.main') || document.querySelector('main') || document.body; return el ? el.innerHTML.substring(0, 3000) : 'no body'; }`))
		htmlSnippet := ""
		if debugHTML != nil {
			htmlSnippet = debugHTML.Value.Str()
		}
		fmt.Printf("[whitelist] whitelist content not found. URL=%s\nHTML: %s\n", url, htmlSnippet)
		setWhitelistError("登录后未能自动跳转到 IP 白名单页面，请确认 AppID 正确")
		return
	}

	fmt.Printf("[whitelist] whitelist content found, proceeding...\n")

	// --- Step 3: Edit whitelist ---
	currentWhitelist.mu.Lock()
	currentWhitelist.Status = "logged_in"
	currentWhitelist.QRCode = ""
	currentWhitelist.Flow = ""
	currentWhitelist.mu.Unlock()
	currentWhitelist.broadcastState()

	if cancelled(cancelCh) {
		return
	}

	setWhitelistMsg("正在点击编辑按钮...")
	fmt.Printf("[whitelist] clicking edit button...\n")
	clickEditResult := clickEditButton(page)
	fmt.Printf("[whitelist] clickEdit result: %s\n", clickEditResult)
	time.Sleep(3 * time.Second)

	setWhitelistMsg("正在填写 IP 地址...")
	fmt.Printf("[whitelist] filling IP textarea...\n")
	fillIPTextarea(page, ip)
	fmt.Printf("[whitelist] IP filled, clicking confirm...\n")
	time.Sleep(1 * time.Second)

	setWhitelistMsg("正在确认修改...")
	clickConfirmButton(page)
	fmt.Printf("[whitelist] confirm clicked, checking for admin QR...\n")

	// --- Step 4: Check for admin confirmation QR ---
	// Enable Network to intercept mpQRCode responses for admin QR status
	_ = proto.NetworkEnable{}.Call(page)

	// Set up mpQRCode status listener BEFORE polling for QR appearance
	adminQRReady := make(chan int, 1) // receives status from mpQRCode
	go page.EachEvent(func(e *proto.NetworkResponseReceived) {
		if !strings.Contains(e.Response.URL, "mpQRCode") {
			return
		}
		body, err := proto.NetworkGetResponseBody{RequestID: e.RequestID}.Call(page)
		if err != nil {
			return
		}
		var resp struct {
			Result struct {
				Status int `json:"status"`
			} `json:"result"`
		}
		if json.Unmarshal([]byte(body.Body), &resp) != nil {
			return
		}
		fmt.Printf("[whitelist] admin QR status: %d\n", resp.Result.Status)

		currentWhitelist.mu.Lock()
		currentWhitelist.QRStatus = resp.Result.Status
		switch resp.Result.Status {
		case 0:
			currentWhitelist.Message = "等待管理员扫码..."
		case 4:
			currentWhitelist.Message = "已扫码，等待管理员确认..."
		}
		currentWhitelist.mu.Unlock()
		currentWhitelist.broadcastState()

		select {
		case adminQRReady <- resp.Result.Status:
		default:
		}
	})()

	// Poll for QR code image and wait for mpQRCode API to confirm status
	adminQR := ""
	waited := 0
	for waited < 40 { // up to 20s
		time.Sleep(500 * time.Millisecond)
		waited++
		if cancelled(cancelCh) {
			return
		}
		if fresh := extractQRCodeFromPage(page); fresh != "" && fresh != adminQR {
			adminQR = fresh
		}
		// Check if mpQRCode API has returned (QR is ready)
		select {
		case <-adminQRReady:
			if adminQR == "" {
				time.Sleep(500 * time.Millisecond)
				adminQR = extractQRCodeFromPage(page)
			}
			break
		default:
			continue
		}
		break
	}

	if adminQR != "" {
		setWhitelistMsg("需要管理员扫码确认...")
		currentWhitelist.mu.Lock()
		currentWhitelist.QRCode = adminQR
		currentWhitelist.Flow = "admin"
		currentWhitelist.Status = "scanning_admin"
		currentWhitelist.mu.Unlock()
		currentWhitelist.broadcastState()

		// Keep polling QR for changes (loading → real) and push updates
		go func() {
			last := adminQR
			for {
				time.Sleep(1 * time.Second)
				currentWhitelist.mu.Lock()
				done := currentWhitelist.QRCode == "" || currentWhitelist.Flow != "admin"
				currentWhitelist.mu.Unlock()
				if done {
					return
				}
				if fresh := extractQRCodeFromPage(page); fresh != "" && fresh != last {
					fmt.Printf("[whitelist] admin QR updated\n")
					last = fresh
					currentWhitelist.mu.Lock()
					currentWhitelist.QRCode = fresh
					currentWhitelist.mu.Unlock()
					currentWhitelist.broadcastState()
				}
			}
		}()

		if !waitForQRGone(page, 180*time.Second, cancelCh) {
			return
		}
		time.Sleep(2 * time.Second)
	}

	// --- Step 5: Verify IP was actually written ---
	setWhitelistMsg("正在验证配置结果...")
	time.Sleep(2 * time.Second)
	verifyResult, _ := page.Evaluate(rod.Eval(`function() {
		var labels = document.querySelectorAll('p.deploy-info-item__label');
		for (var i = 0; i < labels.length; i++) {
			if (labels[i].textContent.indexOf('白名单') !== -1) {
				var content = labels[i].nextElementSibling;
				if (content) return content.textContent.trim();
			}
		}
		return '';
	}`))
	wlContent := ""
	if verifyResult != nil {
		wlContent = verifyResult.Value.Str()
	}
	fmt.Printf("[whitelist] verify: whitelist content=%q, expected IP=%q\n", wlContent, ip)

	if !strings.Contains(wlContent, ip) {
		setWhitelistError(fmt.Sprintf("配置可能失败：白名单中未找到 IP %s（当前内容: %s）", ip, wlContent))
		return
	}

	// Done
	currentWhitelist.mu.Lock()
	currentWhitelist.Status = "done"
	currentWhitelist.QRCode = ""
	currentWhitelist.Flow = ""
	currentWhitelist.Message = "白名单配置完成"
	currentWhitelist.mu.Unlock()
	currentWhitelist.broadcastState()
}

// isLoginPage checks if we're on the developers.weixin.qq.com login page.
func isLoginPage(page *rod.Page) bool {
	info, err := page.Info()
	if err != nil {
		return false
	}
	if strings.Contains(info.URL, "/platform/login") {
		return true
	}
	// Fallback: check for login-specific QR elements
	el, _ := page.Timeout(2 * time.Second).Element(`img[alt*="qrcode"], img[alt*="QR"]`)
	return el != nil
}

// handleLoginQR extracts the login QR code and waits for the user to scan it.
func handleLoginQR(page *rod.Page, cancelCh chan struct{}) bool {
	// Retry extraction — QR image may still be loading on first attempt
	var qr string
	for attempt := 0; attempt < 10; attempt++ {
		if cancelled(cancelCh) {
			return false
		}
		qr = extractQRCodeFromPage(page)
		if qr != "" {
			break
		}
		fmt.Printf("[whitelist] login QR not ready yet, retrying (%d/10)...\n", attempt+1)
		time.Sleep(1 * time.Second)
	}
	if qr == "" {
		setWhitelistError("无法获取登录二维码")
		return false
	}
	currentWhitelist.mu.Lock()
	currentWhitelist.QRCode = qr
	currentWhitelist.Flow = "login"
	currentWhitelist.QRStatus = 1
	currentWhitelist.mu.Unlock()
	currentWhitelist.broadcastState()
	fmt.Printf("[whitelist] login QR set, waiting for scan\n")

	// Poll QR for changes (loading placeholder → real QR) and push updates
	go func() {
		last := qr
		for {
			time.Sleep(1 * time.Second)
			currentWhitelist.mu.Lock()
			done := currentWhitelist.QRCode == "" || currentWhitelist.Flow != "login"
			currentWhitelist.mu.Unlock()
			if done {
				return
			}
			if fresh := extractQRCodeFromPage(page); fresh != "" && fresh != last {
				fmt.Printf("[whitelist] login QR updated\n")
				last = fresh
				currentWhitelist.mu.Lock()
				currentWhitelist.QRCode = fresh
				currentWhitelist.mu.Unlock()
				currentWhitelist.broadcastState()
			}
		}
	}()

	// Enable Network to intercept getLoginUuidStatus responses
	_ = proto.NetworkEnable{}.Call(page)

	// Listen for getLoginUuidStatus responses
	go page.EachEvent(func(e *proto.NetworkResponseReceived) {
		if !strings.Contains(e.Response.URL, "getLoginUuidStatus") {
			return
		}
		// Get response body via CDP
		body, err := proto.NetworkGetResponseBody{RequestID: e.RequestID}.Call(page)
		if err != nil {
			return
		}
		var resp struct {
			Code   int `json:"code"`
			Result struct {
				Status int `json:"status"`
			} `json:"result"`
		}
		if json.Unmarshal([]byte(body.Body), &resp) != nil {
			return
		}
		fmt.Printf("[whitelist] uuid status: %d\n", resp.Result.Status)

		currentWhitelist.mu.Lock()
		currentWhitelist.QRStatus = resp.Result.Status
		currentWhitelist.mu.Unlock()
		currentWhitelist.broadcastState()

		// If expired (7), click the refresh button and update QR
		if resp.Result.Status == 7 {
			go refreshQRCode(page)
		}
	})()

	// Wait for URL to leave /platform/login
	deadline := time.Now().Add(180 * time.Second)
	for time.Now().Before(deadline) {
		if cancelled(cancelCh) {
			return false
		}
		time.Sleep(1 * time.Second)
		info, err := page.Info()
		if err != nil {
			continue
		}
		fmt.Printf("[whitelist] login poll URL: %s\n", info.URL)
		if !strings.Contains(info.URL, "/platform/login") {
			currentWhitelist.mu.Lock()
			currentWhitelist.QRCode = ""
			currentWhitelist.Flow = ""
			currentWhitelist.mu.Unlock()
			currentWhitelist.broadcastState()
			fmt.Printf("[whitelist] login redirect detected → %s\n", info.URL)
			return true
		}
	}
	setWhitelistError("登录超时")
	return false
}

// findTargetPage walks all open pages and finds the one at our target URL.
func findTargetPage(browser *rod.Browser, targetURL string) *rod.Page {
	pages, err := browser.Pages()
	if err != nil {
		fmt.Printf("[whitelist] findTargetPage: error: %v\n", err)
		return nil
	}
	for _, p := range pages {
		info, err := p.Info()
		if err != nil {
			continue
		}
		fmt.Printf("[whitelist] findTargetPage: tab: %s\n", info.URL)
		if strings.Contains(info.URL, "/console/product/mp/") {
			return p
		}
	}
	return nil
}

// waitForWhitelistContent polls until the page contains whitelist-related text.
func waitForWhitelistContent(page *rod.Page, timeout time.Duration, cancelCh chan struct{}) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cancelled(cancelCh) {
			return false
		}
		time.Sleep(2 * time.Second)
		info, _ := page.Info()
		if info != nil {
			fmt.Printf("[whitelist] waitForContent: URL=%s\n", info.URL)
			// If WeChat opened cloud.weixin.qq.com in a new tab, it's just a side-effect
			// — our page reference still tracks the original target
		}
		state := detectPageState(page)
		fmt.Printf("[whitelist] waitForContent: state=%s\n", state)
		if state == "whitelist" {
			return true
		}
	}
	return false
}

// waitForQRGone waits for the QR code to disappear (admin scan confirmed).
func waitForQRGone(page *rod.Page, timeout time.Duration, cancelCh chan struct{}) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cancelled(cancelCh) {
			return false
		}
		time.Sleep(1 * time.Second)
		qr := extractQRCodeFromPage(page)
		if qr == "" {
			return true
		}
	}
	return true
}

// refreshQRCode clicks the "刷新" button on the developers login page and extracts the new QR.
func refreshQRCode(page *rod.Page) {
	fmt.Println("[whitelist] clicking refresh button...")
	clickResult, _ := page.Evaluate(rod.Eval(`function() {
		var spans = document.querySelectorAll('span');
		for (var i = 0; i < spans.length; i++) {
			if (spans[i].textContent.trim() === '刷新' && spans[i].classList.contains('cursor-pointer')) {
				spans[i].click();
				return 'clicked';
			}
		}
		return 'not found';
	}`))
	if clickResult != nil {
		fmt.Printf("[whitelist] refresh click: %s\n", clickResult.Value.Str())
	}
	// Wait for new QR to render
	time.Sleep(2 * time.Second)
	newQR := extractQRCodeFromPage(page)
	if newQR != "" {
		currentWhitelist.mu.Lock()
		currentWhitelist.QRCode = newQR
		currentWhitelist.QRStatus = 1
		currentWhitelist.mu.Unlock()
		currentWhitelist.broadcastState()
		fmt.Println("[whitelist] QR code refreshed")
	}
}

func setWhitelistError(msg string) {
	currentWhitelist.mu.Lock()
	currentWhitelist.Status = "error"
	currentWhitelist.Error = msg
	currentWhitelist.QRCode = ""
	currentWhitelist.Flow = ""
	currentWhitelist.mu.Unlock()
	currentWhitelist.broadcastState()
}

func setWhitelistMsg(msg string) {
	currentWhitelist.mu.Lock()
	currentWhitelist.Message = msg
	currentWhitelist.mu.Unlock()
	currentWhitelist.broadcastState()
}

func (ws *whitelistSession) currentStateJSON() string {
	data := map[string]any{
		"status":     ws.Status,
		"flow":       ws.Flow,
		"qrcode":     ws.QRCode,
		"qr_status":  ws.QRStatus,
		"ip":         ws.IP,
		"error":      ws.Error,
		"message":    ws.Message,
	}
	b, _ := json.Marshal(data)
	return string(b)
}

func (ws *whitelistSession) broadcastState() {
	ws.broadcast("event: state\ndata: " + ws.currentStateJSON() + "\n\n")
}

func cancelled(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func detectPageState(page *rod.Page) string {
	// Use rod.Eval + page.Evaluate (not page.Eval which wraps raw strings)
	const js = `function() {
		if (document.querySelector('img[src*="qrcode"], img[alt="qrcode"], img[alt*="qrcode"], img[src*="qrcode"]'))
			return 'qrcode';
		var ps = document.querySelectorAll('p');
		for (var i = 0; i < ps.length; i++) {
			if (ps[i].textContent.indexOf('白名单') !== -1)
				return 'whitelist';
		}
		return 'unknown';
	}`
	result, err := page.Evaluate(rod.Eval(js))
	if err != nil {
		fmt.Printf("[whitelist] detectPageState eval error: %v\n", err)
		return "unknown"
	}
	return result.Value.Str()
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
			continue
		}
		src, _ := el.Attribute("src")
		if src != nil && strings.HasPrefix(*src, "data:image") {
			parts := strings.SplitN(*src, ",", 2)
			if len(parts) == 2 && parts[1] != "" {
				return *src
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

func clickEditButton(page *rod.Page) string {
	debug, _ := page.Evaluate(rod.Eval(`function() {
		var labels = document.querySelectorAll('p.deploy-info-item__label');
		for (var i = 0; i < labels.length; i++) {
			if (labels[i].textContent.indexOf('白名单') !== -1) {
				var label = labels[i];
				var content = label.nextElementSibling;
				if (!content) return 'no sibling for label: ' + label.textContent;
				var link = content.querySelector('a');
				if (!link) return 'no <a> in content, content HTML: ' + content.innerHTML.substring(0, 200);
				return JSON.stringify({
					label: label.textContent.trim(),
					content: content.textContent.trim(),
					linkText: link.textContent.trim()
				});
			}
		}
		return '白名单 label not found';
	}`))
	if debug != nil {
		fmt.Printf("[whitelist] clickEdit: %s\n", debug.Value.Str())
	}

	clickResult, _ := page.Evaluate(rod.Eval(`function() {
		var labels = document.querySelectorAll('p.deploy-info-item__label');
		for (var i = 0; i < labels.length; i++) {
			if (labels[i].textContent.indexOf('白名单') !== -1) {
				var content = labels[i].nextElementSibling;
				if (!content) return 'no sibling';
				var link = content.querySelector('a.ml-12, a');
				if (!link) return 'no link';
				link.click();
				return 'clicked: ' + link.textContent.trim();
			}
		}
		return '白名单 not found';
	}`))
	if clickResult != nil {
		return clickResult.Value.Str()
	}
	return "eval failed"
}

func fillIPTextarea(page *rod.Page, ip string) {
	// Use native input via CDP — dispatches proper key events that Vue's v-model listens to
	el := page.MustElement(`textarea.weui-desktop-form__textarea, textarea[placeholder*="多个 IP"], textarea[placeholder*="IP"]`)
	fmt.Printf("[whitelist] fillIP: textarea found\n")
	el.MustClick()
	time.Sleep(200 * time.Millisecond)
	el.MustSelectAllText()
	time.Sleep(100 * time.Millisecond)
	el.MustInput(ip)
	time.Sleep(200 * time.Millisecond)
	// Verify the value was set
	verify, _ := page.Evaluate(rod.Eval(`function() {
		var ta = document.querySelector('textarea.weui-desktop-form__textarea') ||
		         document.querySelector('textarea[placeholder*="多个 IP"]');
		return ta ? 'value=' + ta.value : 'no textarea';
	}`))
	if verify != nil {
		fmt.Printf("[whitelist] fillIP verify: %s\n", verify.Value.Str())
	}
}

func clickConfirmButton(page *rod.Page) {
	clickResult, _ := page.Evaluate(rod.Eval(`function() {
		var wrps = document.querySelectorAll('.weui-desktop-dialog__wrp');
		for (var i = 0; i < wrps.length; i++) {
			var wrp = wrps[i];
			if (wrp.style.display === 'none') continue;
			var btn = wrp.querySelector('button.weui-desktop-btn_primary');
			if (!btn) continue;
			btn.click();
			return 'clicked confirm in visible dialog (index=' + i + ')';
		}
		return 'no visible dialog with confirm button (total=' + wrps.length + ')';
	}`))
	if clickResult != nil {
		fmt.Printf("[whitelist] confirm result: %s\n", clickResult.Value.Str())
	}
}
