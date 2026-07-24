# 如何制作 wechat-pen 主题包（给 AI / 作者）

把 JSON 主题包放到应用同目录的 `themes/` 文件夹，重启服务或调用 `POST /api/themes/reload` 后，会出现在样式包列表中。

## 快速开始

1. 在 `wechat-pen.exe` 同目录创建 `themes/`
2. 新建 `themes/my-theme.json`（可用 `themes/brand-blue.json` 复制改）
3. 运行：

```bash
go run . serve
# 或
./wechat-pen.exe serve
```

4. 打开 Web UI → 样式包下拉 → 选择你的主题

## 文件格式（JSON）

```json
{
  "id": "my-theme",
  "name": "我的主题",
  "description": "一句话描述结构差异，不要只写配色",
  "primary": "#2563eb",
  "extends": "simple",
  "base": "margin:0 auto;...;font-size:{{fontSize}};line-height:{{lineHeight}};...",
  "preCode": "display:block;font-family:...;white-space:pre;...",
  "tags": {
    "h1": "display:block;...",
    "h2": "display:block;...;border-left:4px solid {{primary}};",
    "p": "display:block;margin:0 0 {{gap}};font-size:{{fontSize}};line-height:{{lineHeight}};text-align:{{align}};{{indent}}",
    "blockquote": "display:block;...",
    "pre": "display:block;...;white-space:pre;",
    "code": "font-family:monospace;...",
    "ul": "display:block;...",
    "ol": "display:block;...",
    "li": "display:list-item;...",
    "a": "color:#576b95;...",
    "hr": "display:block;...",
    "table": "display:table;...",
    "th": "...",
    "td": "...",
    "img": "display:block;max-width:100%;...",
    "figure": "...",
    "figcaption": "..."
  }
}
```

### 字段说明

| 字段 | 必填 | 说明 |
|------|------|------|
| `id` | 是 | 唯一 ID，小写短横线，如 `brand-blue`。会作为 API `style` 值 |
| `name` | 建议 | UI 显示名 |
| `description` | 建议 | 说明**结构差异**（标题形态/引用形态/代码块形态），别只写颜色 |
| `primary` | 建议 | 默认主色；用户仍可在 UI 覆盖 |
| `extends` | 否 | 继承内置包：`simple\|magazine\|tech\|warm\|dark\|fresh\|pink\|mono\|academic\|chin`，默认 `simple` |
| `base` | 否 | 外层 `<section>` 的 style |
| `preCode` | 否 | `<pre><code>` 内层 code 的 style（高亮后很重要） |
| `tags` | 否 | 标签 → 内联 style 覆盖表 |

### 占位符

| 占位符 | 出现位置 | 含义 |
|--------|----------|------|
| `{{primary}}` | 任意 | 主色 |
| `{{fontSize}}` | `base` / `p` | 用户字号，如 `16px` |
| `{{lineHeight}}` | `base` / `p` | 用户行高，如 `1.75` |
| `{{gap}}` | `p` | 段间距，如 `1em` |
| `{{align}}` | `p` | `justify` 或 `left` |
| `{{indent}}` | `p` | 开启首行缩进时为 `text-indent:2em;`，否则空 |

## 公众号约束（必须遵守）

主题最终会变成**内联 style** 粘贴进公众号编辑器，因此：

1. **只用内联 CSS**，不要 class / 外部 CSS / JS
2. **安全属性优先**：`color font-size font-family font-weight line-height letter-spacing text-align text-decoration text-indent margin padding border border-radius background background-color width max-width height display overflow box-shadow`
3. **避免**：`position:fixed/sticky`、高 z-index 浮层、复杂动画、`script`、表单、非腾讯 iframe
4. **代码块必须**：`pre` 与 `pre>code` 带 `white-space:pre`（否则缩进/空格会丢）
5. **图片**：`max-width:100%;height:auto;display:block;`
6. **不要依赖外部字体文件**；可用系统字体栈（PingFang / Microsoft YaHei / Georgia 等）

## 如何做出“结构不同”的主题（AI 重点）

用户抱怨“只是换色”。做主题时请至少改变 **3 类结构**，而不是只改 `color`：

1. **标题形态**
   - 左边框 / 底边框 / 色块底 / 居中字距 / 胶囊小标题 / 双线夹标题
2. **引用与列表**
   - 气泡圆角 / 上下线夹引文 / 清单底板卡片 / 中文序号 `cjk-ideographic`
3. **代码与表格**
   - 暗底终端 / 虚线边框 / 圆角卡片表头反色 / 无边框杂志表

### 反例（太像换皮）

```json
{
  "id": "red-skin",
  "extends": "simple",
  "tags": {
    "h2": "color:#c00;border-left:4px solid #c00;"
  }
}
```

### 正例（结构变化）

```json
{
  "id": "editorial",
  "name": "编辑部",
  "extends": "magazine",
  "primary": "#111111",
  "tags": {
    "h1": "display:block;font-size:28px;text-align:center;letter-spacing:0.2em;font-weight:700;",
    "h2": "display:block;text-align:center;letter-spacing:0.14em;border-top:1px solid #111;border-bottom:1px solid #111;padding:0.35em 0;",
    "blockquote": "display:block;margin:1.5em 2em;border-top:1px solid #ddd;border-bottom:1px solid #ddd;text-align:center;font-style:italic;background:transparent;",
    "li": "display:list-item;margin:0.5em 0;padding:8px 10px;background:#f7f7f7;border-radius:8px;"
  }
}
```

## 与 API / UI 的关系

- 列表：`GET /api/styles` → 内置 + `themes/*.json`
- 重载：`POST /api/themes/reload`（改完 JSON 不用重启）
- 转换：`POST /api/convert`，字段 `style` 填主题 `id`
- CLI：`wechat-pen -style brand-blue article.md`

## 建议工作流（给 AI）

当用户说“帮我做一个 XX 风格主题”时：

1. 先选最接近的 `extends` 内置包
2. 明确 3 个结构差异（标题/引用/代码）
3. 输出完整 JSON 到 `themes/<id>.json`
4. 提醒用户 `POST /api/themes/reload` 或重启
5. 用一段含 `标题/列表/引用/代码/表格` 的 Markdown 预览验收

## 可覆盖标签清单

`h1-h6 p strong b em i u s del a ul ol li blockquote code pre img hr table thead tbody tr th td section div figure figcaption`

特殊 key：

- `base`：外层 section
- `preCode`：代码块内 code

## 验收清单

- [ ] 标题、正文、引用、列表、代码、表格、图片都有明确样式
- [ ] 代码缩进在公众号预览中仍在
- [ ] 不依赖 class / 外部资源
- [ ] 手机宽度 375px 下不错位
- [ ] `description` 写清结构差异
