# wechat-pen

Markdown → 微信公众号可粘贴 HTML 工具（Go + goldmark + chroma + Vue3/shadcn-vue）。

## 功能

- **公众号内联样式**：外链 CSS/class 会被编辑器剥离，本工具把排版写成 `style="..."`
- **4 套样式包**：简约 / 杂志 / 科技 / 暖色 + 自定义主色
- **中文排版**：首行缩进、两端对齐、段间距、字号、行高
- **代码高亮**：chroma 内联色（GitHub / Xcode / Monokai / Dracula），空格用 NBSP 防粘贴丢失
- **目录 / 文末 / 组件模板**：名片、小程序链接、关注引导、步骤条、三栏要点…
- **粘贴体检**：本地图片、过长代码、剥离标签等风险提示
- **Web UI**：实时预览、笔记、可拖拽分栏、快捷键
- **CLI**：单文件 / 批量 / watch

## 快速开始

### 开发（前后端分离）

```bash
# 终端 1 — API
go run . serve -addr :8080

# 终端 2 — 前端（代理 /api → :8080）
cd web
npm install
npm run dev
# 打开 http://127.0.0.1:5173
```

### 单二进制（嵌入前端）

```powershell
# 一键：构建前端 + 嵌入 + go build
.\build.ps1

.\wechat-pen.exe serve
# 打开 http://127.0.0.1:8080
```

### Docker

```bash
cp .env.example .env   # 填 WECHAT_PROXY_API_KEY / APPID / SECRET
docker compose up -d --build
# pen → http://127.0.0.1:8080  proxy → :8090
```

容器内 `HOME=/data` 持久化笔记与配置；含 headless Chromium，可扫码登录。开放平台调用默认走同 compose 的 proxy。
## CLI

```bash
# 单文件（默认 wechat 主题）
wechat-pen -theme wechat -style tech -toc -footer article.md -o out.html

# 批量
wechat-pen batch -dir ./posts -out ./html -r

# 监听目录
wechat-pen watch -dir ./posts

# 服务
wechat-pen serve -addr :8080
```

常用参数：

| 参数 | 说明 |
|------|------|
| `-theme` | `wechat` / `default` |
| `-style` | `simple` / `magazine` / `tech` / `warm` |
| `-primary` | 主色，如 `#07c160` |
| `-indent` | 首行缩进 |
| `-justify` | 两端对齐（默认 true） |
| `-highlight` | 代码高亮（默认 true） |
| `-toc` / `-footer` | 目录 / 文末 |

## Markdown 组件语法

```md
:::toc
:::footer
:::divider
:::follow{text="欢迎关注"}
:::author{name="昵称" desc="简介"}
:::steps{1="准备" 2="编写" 3="发布"}
:::grid3{a="观点一" b="观点二" c="观点三"}
:::profile{nickname="公众号名"}
:::mp-link{appid="wx..." path="pages/index" text="打开小程序"}
:::kicker{text="导读一句话"}
:::titlebar{text="章节标题"}
:::quote{text="金句内容" from="出处"}
:::faq{q="问题" a="回答"}

![alt](https://cdn.example.com/a.png)
*图注文字*
```

## Web 能力补充

- 预览：手机 / 平板 / 全宽 + 深壳/浅壳
- 对比：两种样式包并排预览
- 工具栏：标题/加粗/列表/代码/链接/图片
- 字数：汉字数、预计阅读时长、标题长度提示
- 主题导入导出：JSON 共享主色/字号等
- 复制后可一键打开公众号后台

## Web 快捷键

| 快捷键 | 作用 |
|--------|------|
| Ctrl/⌘ + B | 加粗 |
| Ctrl/⌘ + I | 斜体 |
| Ctrl/⌘ + K | 链接 |
| Ctrl/⌘ + Shift + C | 复制 HTML |
| Ctrl/⌘ + S | 保存草稿 |

## API

`POST /api/convert` JSON：

```json
{
  "markdown": "# hi",
  "theme": "wechat",
  "style": "simple",
  "primaryColor": "#07c160",
  "highlight": true,
  "highlightTheme": "github",
  "fontSize": "16px",
  "lineHeight": "1.75",
  "textIndent": false,
  "justify": true,
  "toc": false,
  "footer": false
}
```

响应含 `html`（粘贴片段）、`preview`（完整预览页）、`report`（清洗）、`health`（体检）。

## 目录结构

```
converter/   # 转换核心
server/      # HTTP + 嵌入 SPA
web/         # Vite + Vue3 + shadcn-vue
main.go      # CLI 入口
```

## 注意

- 公众号**不支持** script、表单、复杂 iframe、外链 CSS。
- 本地图片路径无法显示，请先上传素材库换 CDN 链接。
- 小程序 / 名片组件为占位，需在公众号后台核对或替换官方组件。
