# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 重要规则

- **不要启动项目**：不要运行 `npm run dev`、`go run`、`build.ps1` 或其他构建/编译命令
- **不要编译项目**：不要运行 `vue-tsc`、`vite build`、`go build` 等编译命令
- **只修改代码**：仅通过编辑文件来完成修改，用户会自行启动、编译和测试

## 构建命令

```powershell
# 单二进制构建（前端 web/dist → server/spa 嵌入 → go build）
.\build.ps1

# 开发模式（前后端分离）
go run . serve -addr :8080          # 终端 1
cd web && npm install && npm run dev # 终端 2 → http://127.0.0.1:5173

# 仅构建前端
cd web && npm run build

# 仅构建后端
go build -o wechat-pen.exe .

# 手动复制前端产物后重启
powershell -NoProfile -Command "Remove-Item -Recurse -Force server\spa -ErrorAction SilentlyContinue; Copy-Item -Recurse web\dist server\spa"
```

## 架构

```
converter/   # Markdown→HTML 转换引擎（goldmark + chroma 语法高亮）
server/      # HTTP API + 嵌入式 SPA（go:embed）
web/         # Vue3 + Vite + shadcn-vue 前端
main.go      # Cobra CLI（根命令 convert / batch / watch / serve）
```

**核心数据流**：Markdown → `converter.ConvertEx()` → 内联样式 HTML → 前端预览 / 复制 / 发布草稿

`converter.Config` 控制所有输出参数：主题、样式包、主色、字号、行高、缩进、高亮主题等。`converter/styles.go` 定义了 10 套内置样式包，每套通过 `stylePack` 结构体（Base + Tags map + PreCode）覆盖默认标签样式。

## 微信公众号集成（server 包）

凭据存储于 `~/.wechat-pen.json`，通过环境变量 / CLI 参数 / 前端设置面板配置。

### 微信 API 模块

| 文件 | 功能 |
|------|------|
| `server/wechat_token.go` | access_token 获取与缓存（stable_token 接口），自动刷新 |
| `server/wechat_material.go` | 永久素材列表(`batchget_material`)、删除(`del_material`)、上传(`add_material`) |
| `server/wechat_upload.go` | 文章内容图片处理：下载外链图片 → >1MB 时缩放+JPEG压缩 → `uploadimg`(不占配额) 或 `add_material`(永久素材) |
| `server/wechat_draft.go` | 草稿发布：markdown→HTML→`draft/add`，可选途中上传文中图片 |
| `server/wechat_login.go` | CDP 扫码登录：Go 启动 Chrome(`go-rod/rod`) → 打开 mp.weixin.qq.com → 截取二维码 → 扫码后提取 Cookie→持久化 |

### 图片上传两种模式（发布草稿时的勾选项）
- **勾选**：走 `media/uploadimg`，≤1MB 直传，超过自动压缩，**不占**永久素材配额
- **不勾选**：走 `material/add_material`，无大小限制，**占用**配额

### 登录态 / 网络参数捕获
- 浏览器 profile 持久化在 `~/.wechat-pen-chrome-profile`，退出时删除
- 登录后注入 JS 到页面劫持 `fetch`/`XMLHttpRequest`，通过 `console.log` + CDP Runtime 事件捕获 URL 中的 `token` 和 `fingerprint` 参数
- 退出登录调用 `POST /api/login/logout` 清除 Cookie + 删除浏览器 profile

### 关键 API 路由
```
POST /api/convert          # Markdown→HTML
GET  /api/material/batch   # 素材列表（代理微信 batchget_material）
POST /api/material/upload  # 上传永久素材（封面）
POST /api/material/delete  # 删除永久素材
POST /api/draft/add        # 发布草稿
POST /api/login/start      # 获取登录二维码（启动 CDP 浏览器）
GET  /api/login/status     # 轮询登录状态
POST /api/login/logout     # 退出登录
GET  /api/ip               # 获取出口 IPv4
```

## 前端状态要点

- `App.vue` 管理全局状态（markdown、样式、草稿对话框、素材选择、封面预览）
- 草稿对话框内的素材选取器是**行内展开**（非独立遮罩），避免 z-index 冲突
- `MaterialBrowser.vue`（独立侧边栏面板）用于从编辑器头部「素材」按钮打开，`z-50`；草稿对话框 `z-40`
- 封面选择后 `draftCoverUrl` 存储预览 URL，选择素材时同时传 `media_id` 和 `url`

## Windows 注意事项

- Chrome 路径：`C:\Program Files\Google\Chrome\Application\chrome.exe`
- rod 启动浏览器时用 `launcher.New().Bin(path)` 明确指定路径
- 前端复制用 PowerShell `Copy-Item -Recurse`，`rm -rf` 在 bash 下不兼容 Windows 路径
- 端口占用检查：`netstat -ano | grep ':8080'`
