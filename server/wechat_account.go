package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// handleAccountInfo calls the WeChat switchacct API to fetch the current account name and avatar.
func handleAccountInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentCredsMu.RLock()
	token := currentCreds.Token
	fingerprint := currentCreds.Fingerprint
	cookie := currentCreds.LoginCookie
	currentCredsMu.RUnlock()

	if cookie == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "未登录，请先扫码登录"})
		return
	}
	if token == "" || fingerprint == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "缺少 token 或 fingerprint，请重新登录"})
		return
	}

	apiURL := fmt.Sprintf(
		"https://mp.weixin.qq.com/cgi-bin/switchacct?action=get_acct_list&fingerprint=%s&token=%s&lang=zh_CN&f=json&ajax=1",
		fingerprint, token,
	)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "构建请求失败: " + err.Error()})
		return
	}
	req.Header.Set("Cookie", cookie)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "请求失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "读取响应失败: " + err.Error()})
		return
	}

	if resp.StatusCode != http.StatusOK {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": fmt.Sprintf("微信接口返回状态码 %d", resp.StatusCode)})
		return
	}

	type bizItem struct {
		Nickname   string `json:"nickname"`
		HeadimgURL string `json:"headimgurl"`
	}

	var result struct {
		BaseResp struct {
			ErrMsg string `json:"err_msg"`
			Ret    int    `json:"ret"`
		} `json:"base_resp"`
		BizList struct {
			Length int        `json:"length"`
			List   []bizItem  `json:"list"`
		} `json:"biz_list"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "解析响应失败"})
		return
	}

	if result.BaseResp.Ret != 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": fmt.Sprintf("微信接口错误: %s (code=%d)", result.BaseResp.ErrMsg, result.BaseResp.Ret)})
		return
	}

	if len(result.BizList.List) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": "未获取到账号信息"})
		return
	}

	first := result.BizList.List[0]
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"name":        first.Nickname,
		"headimg_url": first.HeadimgURL,
	})
}
