export type Theme = 'wechat' | 'default'
/** Builtin packs + any external themes/*.json id */
export type StylePack = string
export type HighlightTheme = string
export type PreviewWidth = 'phone' | 'tablet' | 'full'
export type PreviewShell = 'dark' | 'light'

export interface ConvertRequest {
  markdown: string
  theme: Theme
  title: string
  style: StylePack
  primaryColor: string
  textIndent: boolean
  justify: boolean
  paragraphGap: string
  fontSize: string
  lineHeight: string
  highlight: boolean
  highlightTheme: HighlightTheme
  toc: boolean
  footer: boolean
  imageCaption: boolean
  previewWidth?: PreviewWidth
  previewShell?: PreviewShell
}

export interface CleanReport {
  droppedTags?: string[]
  droppedAttrs?: string[]
}

export interface HealthWarning {
  level: 'info' | 'warn' | 'error' | string
  code: string
  message: string
}

export interface HealthReport {
  warnings?: HealthWarning[]
  ok?: boolean
}

export interface MaterialItem {
  media_id: string
  name: string
  update_time: number
  url: string
}

export interface MaterialListResponse {
  total_count: number
  item_count: number
  items: MaterialItem[]
}

export interface ConvertResponse {
  html: string
  preview: string
  theme: string
  style: string
  charCount: number
  report: CleanReport
  health: HealthReport
}

export interface DraftSnapshot {
  id: string
  at: number
  markdown: string
  title?: string
}

export interface DraftItem {
  id: string
  name: string
  markdown: string
  updatedAt: number
  settings?: Partial<ConvertRequest>
  history?: DraftSnapshot[]
}

export interface ThemePreset {
  version: 1
  name: string
  exportedAt: number
  settings: Partial<ConvertRequest> & {
    fontSizePx?: number
    lineHeightNum?: number
  }
}

export const SAMPLE_MD = `# 一款给公众号用的 Markdown 工具

写完 Markdown，一键转成**可粘贴**的公众号 HTML。

## 为什么需要内联样式？

公众号编辑器会剥离外部 CSS 和大部分 class。真正能留下来的，是标签上的 \`style="..."\`。

> 本工具基于 goldmark，输出时把排版写成内联样式，并支持代码高亮。

### 你能用到的

- 标题 / 段落 / 加粗 / 斜体 / 删除线
- 有序 & 无序列表
- 引用块、表格、图片图注
- 代码（行内与**语法高亮**代码块）
- 目录 / 文末引导 / 组件占位

### 表格示例

| 能力 | 状态 | 备注 |
| --- | --- | --- |
| GFM 表格 | 可用 | 轻量表格 |
| 代码高亮 | 可用 | chroma 内联色 |
| 主题包 | 10+ 套 | 含自定义 |

### 代码

行内：\`go run . serve\`

\`\`\`go
package main

import "fmt"

func main() {
    html, err := converter.Convert(src, converter.Config{
        Theme:     converter.ThemeWeChat,
        Style:     converter.StyleTech,
        Highlight: true,
        TOC:       true,
    })
    if err != nil {
        panic(err)
    }
    fmt.Println(string(html))
}
\`\`\`

### 链接

欢迎阅读 [goldmark 文档](https://github.com/yuin/goldmark)。

:::divider

*复制右侧 HTML，粘贴进公众号编辑器试试。*
`

export const STYLE_PRESETS: {
  value: StylePack
  label: string
  color: string
  desc?: string
}[] = [
  { value: 'simple', label: '简约', color: '#07c160', desc: '左边框章节 · 常规公众号排版' },
  { value: 'tech', label: '科技', color: '#3b82f6', desc: '色条标题 · 暗底代码 · 胶囊标签' },
  { value: 'magazine', label: '杂志', color: '#c23a2b', desc: '全居中衬线 · 无边框引用 · 装饰分割' },
  { value: 'warm', label: '暖色', color: '#d97706', desc: '胶囊小标题 · 虚线分隔 · 软阴影图' },
  { value: 'dark', label: '暗黑', color: '#58a6ff', desc: '整卡深色阅读 · 高亮描边' },
  { value: 'fresh', label: '清新', color: '#059669', desc: '实心色条 h2 · 浅绿清单底板' },
  { value: 'pink', label: '粉色', color: '#ec4899', desc: '气泡式引用 · 圆角列表卡片' },
  { value: 'mono', label: '极简灰', color: '#6b7280', desc: '等宽正文 · 线稿感 · 灰度图片' },
  { value: 'academic', label: '学术', color: '#1d4ed8', desc: '论文体 · 居中标题 · 表注风格' },
  { value: 'chin', label: '中国风', color: '#c2410c', desc: '宣纸底 · 双线标题 · 中文序号' },
]

export const HIGHLIGHT_THEMES: { value: HighlightTheme; label: string }[] = [
  { value: 'github', label: 'GitHub 浅色' },
  { value: 'xcode', label: 'Xcode 浅色' },
  { value: 'monokai', label: 'Monokai 暗色' },
  { value: 'dracula', label: 'Dracula 暗色' },
  { value: 'solarized-light', label: 'Solarized 浅色' },
  { value: 'solarized-dark', label: 'Solarized 暗色' },
  { value: 'tango', label: 'Tango 彩色' },
  { value: 'friendly', label: 'Friendly 暖色' },
  { value: 'emacs', label: 'Emacs 暗底' },
  { value: 'vim', label: 'Vim 暗底' },
  { value: 'igor', label: 'Igor 清爽' },
  { value: 'native', label: 'Native 暗色' },
]

export const SNIPPETS: { label: string; insert: string }[] = [
  { label: '目录', insert: '\n:::toc\n' },
  { label: '分割线', insert: '\n:::divider\n' },
  { label: '文末', insert: '\n:::footer\n' },
  { label: '导读', insert: '\n:::kicker{text="一句话概括本文核心观点。"}\n' },
  { label: '彩条标题', insert: '\n:::titlebar{text="章节标题"}\n' },
  { label: '金句', insert: '\n:::quote{text="把复杂留给自己，把简单留给用户。" from="wechat-pen"}\n' },
  { label: 'FAQ', insert: '\n:::faq{q="为什么要用内联样式？" a="因为公众号会剥掉外部 CSS。"}\n' },
  { label: '关注引导', insert: '\n:::follow{text="欢迎关注，获取更多干货"}\n' },
  { label: '作者卡', insert: '\n:::author{name="作者昵称" desc="一句话介绍"}\n' },
  { label: '步骤条', insert: '\n:::steps{1="准备素材" 2="编写 Markdown" 3="复制粘贴"}\n' },
  { label: '三栏要点', insert: '\n:::grid3{a="观点一" b="观点二" c="观点三"}\n' },
  { label: '公众号名片', insert: '\n:::profile{nickname="老鼠杂货铺" headimg="https://i.pravatar.cc/96?img=3" alias="bhrwga" signature="一个奇奇怪怪的杂货铺" id="MzIxMTY5ODgwNg==" from="0" is_biz_ban="0"}\n' },
  { label: '小程序链接', insert: '\n:::mp-link{appid="wxAPPID" path="pages/index" text="打开小程序"}\n' },
  { label: '图注', insert: '\n![说明](https://example.com/img.png)\n*图片说明文字*\n' },
]

/** Chinese-aware stats for WeChat-style writing. */
export function computeStats(md: string, title = '') {
  const noCode = md.replace(/```[\s\S]*?```/g, ' ')
  const plain = noCode
    .replace(/!\[[^\]]*\]\([^)]*\)/g, ' ')
    .replace(/\[[^\]]*\]\([^)]*\)/g, ' ')
    .replace(/[#>*_`~\-|[\]()]/g, ' ')
    .replace(/\s+/g, '')
  const chinese = (plain.match(/[一-鿿]/g) || []).length
  const other = Math.max(0, [...plain].length - chinese)
  const readingMin = Math.max(1, Math.ceil((chinese + other * 0.5) / 400))
  const titleLen = [...title].length
  return {
    chinese,
    total: chinese + other,
    readingMin,
    titleLen,
    titleWarn: titleLen > 64,
  }
}

