package converter

import (
	"fmt"
	"strings"
)

func wrapWeChatPreview(body string, cfg Config) ([]byte, error) {
	title := cfg.Title
	if title == "" {
		title = "公众号预览"
	}
	styleLabel := string(cfg.Style)
	if styleLabel == "" {
		styleLabel = "simple"
	}

	mode := strings.ToLower(strings.TrimSpace(cfg.PreviewWidth))
	if mode == "" {
		mode = "phone"
	}
	shell := strings.ToLower(strings.TrimSpace(cfg.PreviewShell))
	if shell == "" {
		shell = "dark"
	}

	width := "375px"
	radiusOuter := "36px"
	radiusInner := "26px"
	padFrame := "12px"
	minH := "640px"
	pagePad := "20px 12px 32px"
	bodyPad := "8px 16px 48px"
	showChrome := true
	shadow := "0 0 0 1px rgba(255,255,255,.08) inset, 0 20px 50px rgba(0,0,0,.22), 0 2px 8px rgba(0,0,0,.12)"
	frameBG := "#111"
	notchBG := "#111"
	if shell == "light" {
		frameBG = "#d8dbe2"
		notchBG = "#c5c9d2"
	}

	switch mode {
	case "tablet":
		width = "768px"
		radiusOuter = "28px"
		radiusInner = "20px"
		padFrame = "10px"
		minH = "720px"
	case "full":
		width = "100%"
		radiusOuter = "0"
		radiusInner = "0"
		padFrame = "0"
		minH = "100%"
		pagePad = "0"
		bodyPad = "24px 28px 56px"
		showChrome = false
		shadow = "none"
		frameBG = "transparent"
	case "phone":
	default:
		if strings.HasSuffix(mode, "px") || mode == "100%" {
			width = mode
		}
	}

	chrome := "flex"
	notchDisp := "block"
	notchClass := "device-notch"
	if !showChrome {
		chrome = "none"
		notchDisp = "none"
		notchClass = ""
	}

	return []byte(fmt.Sprintf(previewTpl,
		htmlEscape(title), pagePad,
		width, frameBG, radiusOuter, padFrame, shadow,
		radiusInner, minH,
		chrome, chrome, // status-bar, nav
		bodyPad, chrome, // phone-body padding, home-indicator
		notchBG, notchDisp, notchClass,
		htmlEscape(title), // nav title only
		body,
	)), nil
}

const previewTpl = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s</title>
<style>
  * { box-sizing: border-box; }
  html, body {
    margin: 0;
    height: 100%%;
    background:
      radial-gradient(1200px 600px at 20%% -10%%, #dfe7f3 0%%, transparent 55%%),
      radial-gradient(900px 500px at 100%% 0%%, #e8f5ee 0%%, transparent 50%%),
      #e8eaed;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif;
  }
  body {
    display: flex;
    align-items: flex-start;
    justify-content: center;
    padding: %s;
    min-height: 100%%;
  }
  .device {
    width: %s;
    max-width: 100%%;
    background: %s;
    border-radius: %s;
    padding: %s;
    box-shadow: %s;
  }
  .device-screen {
    background: #fff;
    border-radius: %s;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    min-height: %s;
    max-height: calc(100vh - 48px);
  }
  .status-bar {
    height: 44px;
    padding: 0 18px;
    display: %s;
    align-items: center;
    justify-content: space-between;
    font-size: 12px;
    font-weight: 600;
    color: #111;
    background: #fff;
    flex-shrink: 0;
  }
  .status-bar .dots { opacity: .75; font-size: 11px; font-weight: 500; }
  .nav {
    height: 44px;
    display: %s;
    align-items: center;
    justify-content: center;
    position: relative;
    border-bottom: 1px solid #f0f0f0;
    background: #fff;
    flex-shrink: 0;
  }
  .nav-title {
    font-size: 16px;
    font-weight: 600;
    color: #111;
    max-width: 60%%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .nav-back { position: absolute; left: 12px; color: #576b95; font-size: 14px; }
  .nav-more { position: absolute; right: 14px; color: #111; font-size: 18px; letter-spacing: 2px; line-height: 1; }
  .phone-body {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: %s;
    -webkit-overflow-scrolling: touch;
    scrollbar-width: thin;
    scrollbar-color: rgba(0,0,0,.18) transparent;
  }
  .phone-body::-webkit-scrollbar { width: 6px; }
  .phone-body::-webkit-scrollbar-thumb { background: rgba(0,0,0,.15); border-radius: 999px; }
  .home-indicator {
    height: 20px;
    display: %s;
    align-items: flex-end;
    justify-content: center;
    padding-bottom: 6px;
    background: #fff;
    flex-shrink: 0;
  }
  .home-indicator i {
    display: block; width: 108px; height: 4px; border-radius: 999px; background: #111; opacity: .85;
  }
  .device-notch { position: relative; }
  .device-notch::before {
    content: "";
    position: absolute;
    top: 8px; left: 50%%; transform: translateX(-50%%);
    width: 96px; height: 22px;
    background: %s;
    border-radius: 0 0 14px 14px;
    z-index: 2;
    display: %s;
  }
  /* WeChat profile card preview */
  .weui-msg-card {
    display: flex; align-items: center; gap: 0;
    margin: 0 0 1em; padding: 12px 14px;
    border: 1px solid #ededed; border-radius: 4px;
    background: #fff; cursor: pointer;
    flex-wrap: wrap;
  }
  .weui-msg-card .card-avatar {
    width: 44px; height: 44px; border-radius: 4px;
    flex-shrink: 0; object-fit: cover; margin-right: 12px;
  }
  .weui-msg-card .card-body {
    flex: 1; min-width: 0;
    display: flex; flex-direction: column; justify-content: center;
  }
  .weui-msg-card .card-name {
    font-size: 15px; font-weight: 500; color: #000; line-height: 1.4;
    overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  }
  .weui-msg-card .card-sig {
    font-size: 13px; color: #999; line-height: 1.4; margin-top: 2px;
    overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  }
  .weui-msg-card .card-arrow {
    flex-shrink: 0; margin-left: 8px;
    font-size: 16px; color: #c7c7cc;
  }
  .weui-msg-card .card-divider {
    width: 100%%; height: 1px; background: #ededed;
    margin: 10px 0 8px 0; flex-basis: 100%%;
  }
  .weui-msg-card .card-tag {
    font-size: 12px; color: #b2b2b2; flex-basis: 100%%;
  }
</style>
</head>
<body>
  <div class="device %s">
    <div class="device-screen">
      <div class="status-bar"><span>9:41</span><span class="dots">5G</span></div>
      <div class="nav"><span class="nav-back">‹</span><span class="nav-title">%s</span><span class="nav-more">···</span></div>
      <div class="phone-body">%s</div>
      <div class="home-indicator"><i></i></div>
    </div>
  </div>
<script>
(function() {
  var cards = document.querySelectorAll('mp-common-profile');
  if (!cards.length) return;
  for (var i = 0; i < cards.length; i++) {
    var el = cards[i];
    var nick = el.getAttribute('data-nickname') || '';
    var img  = el.getAttribute('data-headimg') || '';
    var sig  = el.getAttribute('data-signature') || '';
    var div  = document.createElement('div');
    div.className = 'weui-msg-card';
    div.innerHTML =
      '<img class="card-avatar" src="' + img + '" onerror="this.remove()"/>' +
      '<div class="card-body">' +
      '<div class="card-name">' + (nick || '公众号') + '</div>' +
      (sig ? '<div class="card-sig">' + sig + '</div>' : '') +
      '</div>' +
      '<span class="card-arrow">&gt;</span>' +
      '<span class="card-divider"></span>' +
      '<span class="card-tag">公众号</span>';
    el.parentNode.replaceChild(div, el);
  }
})();
</script>
</body>
</html>
`
