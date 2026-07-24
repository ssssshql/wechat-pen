const SAMPLE = [
  "# 一款给公众号用的 Markdown 工具",
  "",
  "写完 Markdown，一键转成**可粘贴**的公众号 HTML。",
  "",
  "## 为什么需要内联样式？",
  "",
  "公众号编辑器会剥离外部 CSS 和大部分 class。真正能留下来的，是标签上的 `style=\"...\"`。",
  "",
  "> 本工具基于 goldmark，输出时把排版写成内联样式。",
  "",
  "### 你能用到的",
  "",
  "- 标题 / 段落 / 加粗 / 斜体 / 删除线",
  "- 有序 & 无序列表",
  "- 引用块",
  "- 表格",
  "- 代码（行内与代码块）",
  "- 图片与链接",
  "",
  "### 表格示例",
  "",
  "| 能力 | 状态 | 备注 |",
  "| --- | --- | --- |",
  "| GFM 表格 | 可用 | 轻量表格 |",
  "| 任务列表 | 可用 | 建议当普通列表 |",
  "| 脚注 | 可用 | 文末自动编号 |",
  "",
  "### 代码",
  "",
  "行内：`go run . serve`",
  "",
  "```go",
  "html, err := converter.Convert(src, converter.Config{",
  "    Theme: converter.ThemeWeChat,",
  "})",
  "```",
  "",
  "### 链接",
  "",
  "欢迎阅读 [goldmark 文档](https://github.com/yuin/goldmark)。",
  "",
  "---",
  "",
  "*复制右侧 HTML，粘贴进公众号编辑器试试。*",
  "",
].join("\n");

const $ = (id) => document.getElementById(id);
const editor = $("editor");
const themeEl = $("theme");
const titleEl = $("title");
const frame = $("previewFrame");
const htmlView = $("htmlView");
const charCount = $("charCount");
const latency = $("latency");
const statusText = $("statusText");
const btnCopy = $("btnCopy");
const btnSample = $("btnSample");

let lastHTML = "";
let lastPreview = "";
let timer = null;
let view = "preview";

function setStatus(msg, cls) {
  statusText.className = cls || "";
  statusText.textContent = msg;
}

async function convert() {
  const markdown = editor.value;
  charCount.textContent = [...markdown].length + " 字";
  const t0 = performance.now();
  try {
    const res = await fetch("/api/convert", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        markdown,
        theme: themeEl.value,
        title: titleEl.value,
      }),
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || res.statusText);
    lastHTML = data.html || "";
    lastPreview = data.preview || data.html || "";
    htmlView.textContent = lastHTML;
    frame.srcdoc = lastPreview;
    latency.textContent = Math.round(performance.now() - t0) + " ms";
    setStatus("已转换 · 主题 " + data.theme, "ok");
  } catch (e) {
    setStatus("转换失败: " + e.message, "err");
  }
}

function schedule() {
  clearTimeout(timer);
  timer = setTimeout(convert, 280);
}

editor.addEventListener("input", schedule);
themeEl.addEventListener("change", convert);
titleEl.addEventListener("change", convert);

btnSample.addEventListener("click", () => {
  editor.value = SAMPLE;
  convert();
});

btnCopy.addEventListener("click", async () => {
  if (!lastHTML) {
    setStatus("没有可复制的内容", "err");
    return;
  }
  try {
    if (navigator.clipboard && window.ClipboardItem) {
      const item = new ClipboardItem({
        "text/html": new Blob([lastHTML], { type: "text/html" }),
        "text/plain": new Blob([lastHTML], { type: "text/plain" }),
      });
      await navigator.clipboard.write([item]);
    } else {
      await navigator.clipboard.writeText(lastHTML);
    }
    btnCopy.classList.add("ok");
    btnCopy.textContent = "已复制";
    setStatus("已复制 HTML · 去公众号编辑器粘贴", "ok");
    setTimeout(() => {
      btnCopy.classList.remove("ok");
      btnCopy.textContent = "复制 HTML";
    }, 1600);
  } catch (e) {
    const ta = document.createElement("textarea");
    ta.value = lastHTML;
    document.body.appendChild(ta);
    ta.select();
    document.execCommand("copy");
    document.body.removeChild(ta);
    setStatus("已复制（降级方案）", "ok");
  }
});

document.querySelectorAll(".tabs button").forEach((btn) => {
  btn.addEventListener("click", () => {
    document.querySelectorAll(".tabs button").forEach((b) => b.classList.remove("active"));
    btn.classList.add("active");
    view = btn.dataset.view;
    if (view === "html") {
      frame.style.display = "none";
      htmlView.style.display = "block";
    } else {
      frame.style.display = "block";
      htmlView.style.display = "none";
    }
  });
});

editor.addEventListener("keydown", (e) => {
  if (e.key === "Tab") {
    e.preventDefault();
    const s = editor.selectionStart;
    const epos = editor.selectionEnd;
    editor.value = editor.value.slice(0, s) + "  " + editor.value.slice(epos);
    editor.selectionStart = editor.selectionEnd = s + 2;
    schedule();
  }
});

editor.value = SAMPLE;
convert();
