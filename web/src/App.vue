<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import { ChevronLeft, ChevronRight, FileCode2, FolderOpen, History, Loader2, Menu, Plus, Rss, Settings2, Sparkles, Trash2, Upload, X, ExternalLink, Copy, Download, RotateCcw, Image, Send } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Toaster } from '@/components/ui/sonner'
import MarkdownEditor from '@/components/MarkdownEditor.vue'
import EditorToolbar from '@/components/EditorToolbar.vue'
import MaterialBrowser from '@/components/MaterialBrowser.vue'
import MpBrowser from '@/components/MpBrowser.vue'
import SettingsPanel from '@/components/SettingsPanel.vue'
import PreviewPanel from '@/components/PreviewPanel.vue'
import { convertMarkdown, downloadText, importTheme, getActiveDraftId, loadDrafts, loadSettings, newDraftId, openMpGuide, pushHistory, saveDrafts, saveSettings, setActiveDraftId, addDraft, fetchMaterials, uploadMaterial } from '@/lib/api'
import { SAMPLE_MD, STYLE_PRESETS, computeStats, type DraftItem, type HighlightTheme, type MaterialItem, type PreviewShell, type PreviewWidth, type StylePack, type Theme } from '@/lib/types'

const markdown = ref(SAMPLE_MD)
const theme = ref<Theme>('wechat')
const style = ref<StylePack>('simple')
const title = ref('示例文章')
const primaryColor = ref('#07c160')
const textIndent = ref(false)
const justify = ref(true)
const highlight = ref(true)
const highlightTheme = ref<HighlightTheme>('github')
const toc = ref(false)
const footer = ref(false)
const imageCaption = ref(true)
const paragraphGap = ref('1em')
const fontSizePx = ref([16])
const lineHeight = ref([1.75])
const html = ref('')
const preview = ref('')
const latency = ref(0)
const loading = ref(false)
const status = ref('就绪')
const view = ref<'preview' | 'html'>('preview')
const copyMode = ref<'rich' | 'source'>('rich')
const editorRef = ref<InstanceType<typeof MarkdownEditor> | null>(null)
const mobileSettings = ref(false)
const showMaterials = ref(false)
const showMp = ref(false)
const showDraftDialog = ref(false)
const draftAuthor = ref('')
const draftDigest = ref('')
const draftThumbMediaId = ref('')
const draftUploadImages = ref(true)
const draftSending = ref(false)
const draftMaterialPicker = ref(false)
const draftMaterials = ref<MaterialItem[]>([])
const draftMaterialsLoading = ref(false)
const draftMaterialsTotal = ref(0)
const draftMaterialsOffset = ref(0)
const draftCoverUrl = ref('')
const draftUploading = ref(false)
const draftCoverInput = ref<HTMLInputElement | null>(null)

async function openDraftMaterialPicker() {
  draftMaterialPicker.value = true
  draftMaterialsOffset.value = 0
  await loadDraftMaterials()
}

async function loadDraftMaterials() {
  draftMaterialsLoading.value = true
  try {
    const data = await fetchMaterials('image', draftMaterialsOffset.value, 20)
    draftMaterials.value = data.items
    draftMaterialsTotal.value = data.total_count
  } catch (e) {
    toast.error('加载素材失败')
  } finally {
    draftMaterialsLoading.value = false
  }
}

function selectDraftCover(mediaId: string, url?: string) {
  draftThumbMediaId.value = mediaId
  draftCoverUrl.value = url || ''
  draftMaterialPicker.value = false
  toast.success('已选择封面')
}

async function handleCoverUpload(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  draftUploading.value = true
  try {
    const result = await uploadMaterial(file)
    draftThumbMediaId.value = result.media_id
    draftCoverUrl.value = result.url
    toast.success('封面上传成功')
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '上传失败')
  } finally {
    draftUploading.value = false
    input.value = ''
  }
}

function draftPickerPrev() {
  draftMaterialsOffset.value = Math.max(0, draftMaterialsOffset.value - 20)
  loadDraftMaterials()
}
function draftPickerNext() {
  draftMaterialsOffset.value = draftMaterialsOffset.value + 20
  loadDraftMaterials()
}
const showDrafts = ref(false)
const showHistory = ref(false)
const drafts = ref<DraftItem[]>([])
const activeDraftId = ref<string | null>(null)
const batchInput = ref<HTMLInputElement | null>(null)
const themeInput = ref<HTMLInputElement | null>(null)

let timer: ReturnType<typeof setTimeout> | null = null
let draftTimer: ReturnType<typeof setTimeout> | null = null

const stats = computed(() => computeStats(markdown.value, title.value))
const styleLabel = computed(() => STYLE_PRESETS.find((s) => s.value === style.value)?.label ?? style.value)
const activeHistory = computed(() => { const d = drafts.value.find((x) => x.id === activeDraftId.value); return d?.history || [] })
const fontSize = computed(() => `${fontSizePx.value[0]}px`)
const lineHeightStr = computed(() => String(lineHeight.value[0]))

function buildReq() {
  return {
    markdown: markdown.value, theme: theme.value, title: title.value, style: style.value,
    primaryColor: primaryColor.value, textIndent: textIndent.value, justify: justify.value,
    paragraphGap: paragraphGap.value, fontSize: fontSize.value, lineHeight: lineHeightStr.value,
    highlight: highlight.value, highlightTheme: highlightTheme.value, toc: toc.value,
    footer: footer.value, imageCaption: imageCaption.value,
    previewWidth: 'phone' as PreviewWidth, previewShell: 'dark' as PreviewShell,
  }
}

function onStyleChange(v: unknown) { style.value = String(v) }
function schedule() { if (timer) clearTimeout(timer); timer = setTimeout(runConvert, 280); scheduleDraftSave() }
function scheduleDraftSave() { if (draftTimer) clearTimeout(draftTimer); draftTimer = setTimeout(persistActiveDraft, 500) }

async function runConvert() {
  loading.value = true; const t0 = performance.now()
  try {
    const data = await convertMarkdown(buildReq())
    html.value = data.html; preview.value = data.preview
    latency.value = Math.round(performance.now() - t0); status.value = `已转换 · ${data.style}`; saveSettings(currentSettings())
  } catch (e) { status.value = e instanceof Error ? e.message : '转换失败' } finally { loading.value = false }
}

function currentSettings() { return { theme: theme.value, style: style.value, title: title.value, primaryColor: primaryColor.value, textIndent: textIndent.value, justify: justify.value, highlight: highlight.value, highlightTheme: highlightTheme.value, toc: toc.value, footer: footer.value, imageCaption: imageCaption.value, paragraphGap: paragraphGap.value, fontSizePx: fontSizePx.value[0], lineHeight: lineHeight.value[0], copyMode: copyMode.value } }
function applySettings(s: Record<string, unknown>) { if (s.theme) theme.value = s.theme as Theme; if (s.style) style.value = s.style as StylePack; if (typeof s.title === 'string') title.value = s.title; if (typeof s.primaryColor === 'string') primaryColor.value = s.primaryColor; if (typeof s.textIndent === 'boolean') textIndent.value = s.textIndent; if (typeof s.justify === 'boolean') justify.value = s.justify; if (typeof s.highlight === 'boolean') highlight.value = s.highlight; if (s.highlightTheme) highlightTheme.value = s.highlightTheme as HighlightTheme; if (typeof s.toc === 'boolean') toc.value = s.toc; if (typeof s.footer === 'boolean') footer.value = s.footer; if (typeof s.imageCaption === 'boolean') imageCaption.value = s.imageCaption; if (typeof s.paragraphGap === 'string') paragraphGap.value = s.paragraphGap; if (typeof s.fontSizePx === 'number') fontSizePx.value = [s.fontSizePx]; if (typeof s.lineHeight === 'number') lineHeight.value = [s.lineHeight]; if (typeof s.lineHeightNum === 'number') lineHeight.value = [s.lineHeightNum]; if (s.copyMode === 'rich' || s.copyMode === 'source') copyMode.value = s.copyMode }

function loadSample() { markdown.value = SAMPLE_MD; schedule(); toast.message('已载入示例') }
function insertSnippet(text: string) { editorRef.value?.insertAtCursor(text); schedule(); mobileSettings.value = false }
function onToolbar(action: string) { editorRef.value?.runToolbar(action); schedule() }
function downloadHTML() { if (!html.value) { toast.error('无内容'); return }; downloadText(`${(title.value || 'article').replace(/[\\/:*?"<>|]/g, '_')}.html`, html.value); toast.success('已下载') }

async function copyHTML() {
  if (!html.value) { toast.error('无内容'); return }
  try {
    if (copyMode.value === 'rich' && navigator.clipboard && window.ClipboardItem) { const item = new ClipboardItem({ 'text/html': new Blob([html.value], { type: 'text/html' }), 'text/plain': new Blob([html.value], { type: 'text/plain' }) }); await navigator.clipboard.write([item]) }
    else await navigator.clipboard.writeText(html.value)
    toast.success('已复制 · 可粘贴到公众号编辑器', { action: { label: '打开后台', onClick: () => openMpGuide() } })
  } catch { const ta = document.createElement('textarea'); ta.value = html.value; document.body.appendChild(ta); ta.select(); document.execCommand('copy'); document.body.removeChild(ta); toast.success('已复制（降级）') }
}

async function publishDraft() {
  if (!html.value) { toast.error('请先转换文章'); return }
  if (!title.value.trim()) { toast.error('请输入标题'); return }
  if (!draftThumbMediaId.value.trim()) { toast.error('请填写封面 media_id'); return }
  draftSending.value = true
  try {
    const result = await addDraft({
      markdown: markdown.value,
      title: title.value.trim(),
      author: draftAuthor.value.trim(),
      digest: draftDigest.value.trim(),
      thumb_media_id: draftThumbMediaId.value.trim(),
      style: style.value,
      primaryColor: primaryColor.value,
      upload_images: draftUploadImages.value,
    })
    toast.success(`已发布到草稿箱，media_id: ${result.media_id}`)
    showDraftDialog.value = false
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '发布失败')
  } finally {
    draftSending.value = false
  }
}

function persistActiveDraft() {
  if (!activeDraftId.value) { const id = newDraftId(); activeDraftId.value = id; setActiveDraftId(id); drafts.value = [{ id, name: title.value || '未命名', markdown: markdown.value, updatedAt: Date.now(), settings: { style: style.value, primaryColor: primaryColor.value, textIndent: textIndent.value, justify: justify.value, highlight: highlight.value, highlightTheme: highlightTheme.value, toc: toc.value, footer: footer.value, imageCaption: imageCaption.value, fontSize: fontSize.value }, history: [] }, ...drafts.value]; saveDrafts(drafts.value); return }
  const idx = drafts.value.findIndex((d) => d.id === activeDraftId.value)
  if (idx >= 0) { let d: DraftItem = { ...drafts.value[idx], name: title.value || drafts.value[idx].name, markdown: markdown.value, updatedAt: Date.now() }; d = pushHistory(d, markdown.value, title.value); drafts.value[idx] = d; saveDrafts(drafts.value) }
}

function selectDraft(id: string) { const d = drafts.value.find((x) => x.id === id); if (!d) return; activeDraftId.value = id; setActiveDraftId(id); markdown.value = d.markdown; if (d.settings) applySettings(d.settings as Record<string, unknown>); showDrafts.value = false; showHistory.value = false; schedule() }
function restoreSnapshot(snapId: string) { const d = drafts.value.find((x) => x.id === activeDraftId.value); const snap = d?.history?.find((h) => h.id === snapId); if (!snap) return; markdown.value = snap.markdown; if (snap.title) title.value = snap.title; showHistory.value = false; schedule(); toast.message('已恢复') }
function createDraft() { persistActiveDraft(); const id = newDraftId(); drafts.value = [{ id, name: `草稿 ${drafts.value.length + 1}`, markdown: '# 新文章\n\n', updatedAt: Date.now(), history: [] }, ...drafts.value]; saveDrafts(drafts.value); selectDraft(id); toast.message('新建草稿') }
function deleteDraft(id: string) { drafts.value = drafts.value.filter((d) => d.id !== id); saveDrafts(drafts.value); if (activeDraftId.value === id) { if (drafts.value[0]) selectDraft(drafts.value[0].id); else { activeDraftId.value = null; setActiveDraftId(null); markdown.value = SAMPLE_MD } }; toast.message('已删除') }
async function onThemeFile(e: Event) { const input = e.target as HTMLInputElement; const file = input.files?.[0]; if (!file) return; try { const raw = await file.text(); const pack = JSON.parse(raw); const saved = await importTheme(pack); style.value = saved.id; if (saved.primary) primaryColor.value = saved.primary; schedule(); toast.success('已导入：' + saved.name) } catch (err) { toast.error(err instanceof Error ? err.message : '导入失败') }; input.value = '' }
async function onBatchFiles(e: Event) { const input = e.target as HTMLInputElement; const files = input.files; if (!files?.length) return; let n = 0; for (const file of Array.from(files)) { if (!/\.(md|markdown|txt)$/i.test(file.name)) continue; const text = await file.text(); try { const data = await convertMarkdown({ ...buildReq(), markdown: text, title: file.name.replace(/\.(md|markdown|txt)$/i, '') }); downloadText(file.name.replace(/\.(md|markdown|txt)$/i, '') + '.html', data.html); n++ } catch (err) { toast.error(`${file.name}: ${err instanceof Error ? err.message : '失败'}`) } }; toast.success(`已导出 ${n} 个`) }
function wrapSelection(before: string, after: string) { editorRef.value?.wrapSelection(before, after); schedule() }
function onKeydown(e: KeyboardEvent) { const mod = e.metaKey || e.ctrlKey; if (!mod) return; if (e.key === 'b') { e.preventDefault(); wrapSelection('**', '**') } else if (e.key === 'i') { e.preventDefault(); wrapSelection('*', '*') } else if (e.key === 'k') { e.preventDefault(); wrapSelection('[', '](https://)') } else if (e.shiftKey && e.key === 'C') { e.preventDefault(); copyHTML() } else if (e.key === 's') { e.preventDefault(); persistActiveDraft(); toast.success('已保存') } }

watch([markdown, theme, style, title, primaryColor, textIndent, justify, highlight, highlightTheme, toc, footer, imageCaption, paragraphGap, fontSizePx, lineHeight], () => schedule())

onMounted(() => { const settings = loadSettings(currentSettings()); applySettings(settings as Record<string, unknown>); drafts.value = loadDrafts(); const aid = getActiveDraftId(); if (aid && drafts.value.some((d) => d.id === aid)) selectDraft(aid); else if (drafts.value[0]) selectDraft(drafts.value[0].id); else runConvert(); window.addEventListener('keydown', onKeydown) })
onBeforeUnmount(() => { window.removeEventListener('keydown', onKeydown) })
</script>

<template>
  <div class="bg-background text-foreground flex h-svh max-h-svh flex-col overflow-hidden antialiased">
    <Teleport to="body"><Toaster position="top-center" rich-colors /></Teleport>

    <!-- Header -->
    <header class="app-header z-20 shrink-0">
      <div class="flex h-10 items-center gap-1.5 px-3">
        <Button variant="ghost" size="icon-xs" class="lg:hidden" @click="mobileSettings = true"><Menu class="size-3.5" /></Button>
        <div class="mr-1 flex min-w-0 items-center gap-1.5">
          <div class="brand-dot" />
          <div class="truncate text-sm font-semibold tracking-tight">wechat-pen</div>
        </div>
        <div class="ml-auto flex items-center gap-px">
          <Button variant="ghost" size="xs" class="hidden sm:inline-flex" @click="showDrafts = !showDrafts"><FolderOpen class="size-3.5 mr-1" />草稿</Button>
          <Button variant="ghost" size="xs" class="hidden md:inline-flex" @click="showHistory = !showHistory"><History class="size-3.5 mr-1" />历史</Button>
          <span class="mx-1.5 hidden w-px self-stretch bg-border sm:inline" />
          <input ref="themeInput" type="file" accept="application/json,.json" class="hidden" @change="onThemeFile" />
          <Button variant="ghost" size="xs" class="hidden lg:inline-flex" @click="themeInput?.click()">导入主题</Button>
          <Button variant="ghost" size="xs" class="hidden md:inline-flex" @click="loadSample"><RotateCcw class="size-3.5 mr-1" />示例</Button>
          <input ref="batchInput" type="file" accept=".md,.markdown,.txt" multiple class="hidden" @change="onBatchFiles" />
          <Button variant="ghost" size="xs" class="hidden sm:inline-flex" @click="batchInput?.click()"><Download class="size-3.5 mr-1" />批量</Button>
          <Button variant="ghost" size="xs" class="hidden sm:inline-flex" @click="showMaterials = !showMaterials"><Image class="size-3.5 mr-1" />素材</Button>
          <Button variant="ghost" size="xs" class="hidden sm:inline-flex" @click="showMp = !showMp"><Rss class="size-3.5 mr-1" />公众号</Button>
          <span class="mx-1.5 hidden w-px self-stretch bg-border sm:inline" />
          <Button variant="ghost" size="xs" @click="downloadHTML"><Download class="size-3.5 mr-1" />下载</Button>
          <Button size="xs" @click="copyHTML"><Copy class="size-3.5 mr-1" />复制</Button>
          <Button variant="secondary" size="xs" @click="showDraftDialog = true"><Send class="size-3.5 mr-1" />发布草稿</Button>
          <Button variant="ghost" size="icon-xs" title="公众号后台" @click="openMpGuide"><ExternalLink class="size-3" /></Button>
        </div>
      </div>
    </header>

    <!-- Drawers -->
    <div v-if="showDrafts" class="border-border bg-background/98 absolute top-10 right-0 left-0 z-30 max-h-64 overflow-auto border-b p-2.5 shadow-lg backdrop-blur sm:left-auto sm:w-72">
      <div class="mb-2 flex items-center justify-between"><div class="text-sm font-medium">草稿</div><div class="flex gap-1"><Button size="xs" variant="outline" @click="createDraft"><Plus class="size-3 mr-1" />新建</Button><Button size="xs" variant="ghost" @click="showDrafts = false"><X class="size-3" /></Button></div></div>
      <div v-if="!drafts.length" class="text-muted-foreground text-xs">暂无草稿</div>
      <div v-for="d in drafts" :key="d.id" class="hover:bg-muted flex items-center gap-1.5 rounded-md px-2 py-1">
        <button class="min-w-0 flex-1 text-left text-xs" @click="selectDraft(d.id)"><div class="truncate font-medium" :class="d.id === activeDraftId ? 'text-primary' : ''">{{ d.name }}</div><div class="text-muted-foreground text-[10px]">{{ new Date(d.updatedAt).toLocaleString() }}</div></button>
        <Button size="icon-xs" variant="ghost" @click="deleteDraft(d.id)"><Trash2 class="size-3" /></Button>
      </div>
    </div>

    <div v-if="showHistory" class="border-border bg-background/98 absolute top-10 right-0 z-30 max-h-72 w-72 overflow-auto border-b border-l p-2.5 shadow-lg backdrop-blur">
      <div class="mb-2 flex items-center justify-between"><div class="text-sm font-medium">历史版本</div><Button size="xs" variant="ghost" @click="showHistory = false"><X class="size-3" /></Button></div>
      <div v-if="!activeHistory.length" class="text-muted-foreground text-xs">暂无快照</div>
      <button v-for="h in activeHistory" :key="h.id" class="hover:bg-muted block w-full rounded-md px-2 py-1 text-left text-xs" @click="restoreSnapshot(h.id)"><div class="truncate">{{ (h.title || '未命名') }} · {{ h.markdown.length }} 字符</div><div class="text-muted-foreground text-[10px]">{{ new Date(h.at).toLocaleString() }}</div></button>
    </div>

    <!-- Main -->
    <div class="flex min-h-0 flex-1 overflow-hidden">
      <aside class="scroll-panel hidden w-[224px] shrink-0 overflow-y-auto border-r bg-white/60 lg:block">
        <SettingsPanel v-model:style="style" v-model:primary-color="primaryColor" v-model:title="title" v-model:text-indent="textIndent" v-model:justify="justify" v-model:paragraph-gap="paragraphGap" v-model:font-size-px="fontSizePx" v-model:line-height="lineHeight" v-model:highlight="highlight" v-model:highlight-theme="highlightTheme" v-model:toc="toc" v-model:footer="footer" v-model:image-caption="imageCaption" @style-change="onStyleChange" @insert="insertSnippet" />
      </aside>

      <section class="editor-pane flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden border-r">
        <div class="text-muted-foreground flex h-8 shrink-0 items-center justify-between border-b px-3 text-[11px]">
          <span class="flex items-center gap-1.5 font-medium tracking-wide"><FileCode2 class="size-3.5" />Markdown</span>
          <span class="tabular-nums">{{ stats.total }} 字 · {{ stats.readingMin }}分 <span v-if="stats.titleWarn" class="text-amber-500 ml-1">标题 {{ stats.titleLen }}字</span></span>
        </div>
        <EditorToolbar @action="onToolbar" />
        <MarkdownEditor ref="editorRef" v-model="markdown" />
      </section>

      <section class="preview-pane flex min-h-0 w-[460px] shrink-0 flex-col overflow-hidden">
        <PreviewPanel v-model:view="view" :preview="preview" :html="html" />
      </section>
    </div>

    <!-- Mobile settings -->
    <div v-if="mobileSettings" class="fixed inset-0 z-40 bg-black/40 lg:hidden" @click.self="mobileSettings = false">
      <div class="bg-background absolute inset-y-0 left-0 flex w-[min(100%,300px)] flex-col shadow-xl">
        <div class="flex h-10 items-center justify-between border-b px-3"><div class="flex items-center gap-1.5 text-sm font-medium"><Settings2 class="size-4" />设置</div><Button size="icon-xs" variant="ghost" @click="mobileSettings = false"><X class="size-3.5" /></Button></div>
        <div class="scroll-panel flex-1"><SettingsPanel v-model:style="style" v-model:primary-color="primaryColor" v-model:title="title" v-model:text-indent="textIndent" v-model:justify="justify" v-model:paragraph-gap="paragraphGap" v-model:font-size-px="fontSizePx" v-model:line-height="lineHeight" v-model:highlight="highlight" v-model:highlight-theme="highlightTheme" v-model:toc="toc" v-model:footer="footer" v-model:image-caption="imageCaption" v-model:app-id="appID" v-model:app-secret="appSecret" @style-change="onStyleChange" @insert="insertSnippet" /></div>
      </div>
    </div>

    <!-- Material browser -->
    <!-- (empty, moved after draft dialog) -->

    <!-- Draft dialog -->
    <div v-if="showDraftDialog" class="fixed inset-0 z-40 flex items-center justify-center bg-black/40" @click.self="showDraftDialog = false">
      <div class="bg-background w-full max-w-md rounded-lg shadow-xl mx-4">
        <div class="flex items-center justify-between border-b px-4 py-3">
          <span class="text-sm font-medium">发布到草稿箱</span>
          <Button variant="ghost" size="icon-xs" @click="showDraftDialog = false"><X class="size-3.5" /></Button>
        </div>
        <div class="space-y-3 px-4 py-4">
          <div>
            <label class="text-xs font-medium text-muted-foreground">标题</label>
            <p class="text-sm mt-0.5">{{ title || '（无标题）' }}</p>
          </div>
          <div>
            <label class="text-xs font-medium text-muted-foreground">作者 <span class="text-muted-foreground/60">选填</span></label>
            <input v-model="draftAuthor" class="w-full mt-1 rounded-md border px-2.5 py-1.5 text-sm outline-none focus:border-primary" placeholder="作者名称" />
          </div>
          <div>
            <label class="text-xs font-medium text-muted-foreground">摘要 <span class="text-muted-foreground/60">选填</span></label>
            <textarea v-model="draftDigest" rows="2" class="w-full mt-1 rounded-md border px-2.5 py-1.5 text-sm outline-none focus:border-primary resize-none" placeholder="文章摘要，不填则抓取正文前段" />
          </div>
          <div>
            <label class="text-xs font-medium text-muted-foreground">封面 <span class="text-red-500">*</span></label>
            <div class="flex gap-2 mt-1">
              <input v-model="draftThumbMediaId" class="flex-1 rounded-md border px-2.5 py-1.5 text-sm outline-none focus:border-primary" placeholder="media_id" />
              <Button variant="outline" size="xs" @click="openDraftMaterialPicker">选择</Button>
              <input ref="draftCoverInput" type="file" accept="image/*" class="hidden" @change="handleCoverUpload" />
              <Button variant="outline" size="xs" :disabled="draftUploading" @click="draftCoverInput?.click()">
                <Loader2 v-if="draftUploading" class="size-3 mr-1 animate-spin" />
                <Upload v-else class="size-3 mr-1" />
                上传
              </Button>
            </div>

            <!-- Cover preview -->
            <div v-if="draftCoverUrl" class="mt-2 relative inline-block rounded-lg overflow-hidden border">
              <img :src="draftCoverUrl" class="h-24 object-cover" referrerpolicy="no-referrer" />
              <button class="absolute top-1 right-1 size-5 rounded-full bg-black/50 text-white flex items-center justify-center hover:bg-black/70" @click="draftCoverUrl = ''; draftThumbMediaId = ''">
                <X class="size-3" />
              </button>
            </div>

            <!-- Inline material picker -->
            <div v-if="draftMaterialPicker" class="mt-2 rounded-lg border bg-muted/30 p-2">
              <div class="flex items-center justify-between mb-2">
                <span class="text-[11px] text-muted-foreground">
                  共 {{ draftMaterialsTotal }} 张 · 点击选择封面
                </span>
                <div class="flex items-center gap-1">
                  <Button variant="ghost" size="icon-xs" :disabled="draftMaterialsOffset === 0" @click="draftPickerPrev">
                    <ChevronLeft class="size-3" />
                  </Button>
                  <Button variant="ghost" size="icon-xs" :disabled="draftMaterialsOffset + 20 >= draftMaterialsTotal" @click="draftPickerNext">
                    <ChevronRight class="size-3" />
                  </Button>
                  <Button variant="ghost" size="icon-xs" @click="draftMaterialPicker = false">
                    <X class="size-3" />
                  </Button>
                </div>
              </div>

              <div v-if="draftMaterialsLoading" class="flex justify-center py-6">
                <Loader2 class="size-4 animate-spin text-muted-foreground" />
              </div>

              <div v-else-if="!draftMaterials.length" class="text-center py-4 text-[11px] text-muted-foreground">
                暂无素材
              </div>

              <div v-else class="grid grid-cols-4 gap-1.5 max-h-48 overflow-auto">
                <button
                  v-for="item in draftMaterials"
                  :key="item.media_id"
                  class="relative overflow-hidden rounded-md border bg-background hover:ring-2 hover:ring-primary transition-all"
                  :class="draftThumbMediaId === item.media_id ? 'ring-2 ring-primary' : ''"
                  @click="selectDraftCover(item.media_id, item.url)"
                >
                  <img :src="item.url" :alt="item.name" class="h-16 w-full object-cover" referrerpolicy="no-referrer" loading="lazy" />
                  <p class="truncate px-1 py-0.5 text-[9px] text-muted-foreground">{{ item.name }}</p>
                </button>
              </div>
            </div>
          </div>
          <div class="flex items-start gap-2">
            <input type="checkbox" id="uploadImages" v-model="draftUploadImages" class="size-3.5 mt-0.5 rounded border-gray-300" />
            <label for="uploadImages" class="text-xs text-muted-foreground leading-relaxed">
              使用快速上传（≤1MB 直接传，超过自动压缩）<br />
              <span class="text-[10px] text-muted-foreground/60">勾选：不占永久素材配额。不勾选：上传为永久素材，无大小限制，占用配额</span>
            </label>
          </div>
        </div>
        <div class="flex justify-end gap-2 border-t px-4 py-3">
          <Button variant="outline" size="sm" @click="showDraftDialog = false">取消</Button>
          <Button size="sm" :disabled="draftSending" @click="publishDraft">
            <Loader2 v-if="draftSending" class="size-3.5 mr-1 animate-spin" />
            <Send v-else class="size-3.5 mr-1" />
            发布
          </Button>
        </div>
      </div>
    </div>

    <!-- Material browser (Teleport to body to escape all stacking contexts) -->
    <Teleport to="body">
      <div v-if="showMaterials" style="z-index:99999" class="fixed inset-0" @click.self="showMaterials = false">
        <div class="bg-background absolute inset-y-0 right-0 flex w-[min(100%,420px)] flex-col shadow-xl border-l">
          <MaterialBrowser
            @insert="(text: string) => { insertSnippet(text); }"
            @close="showMaterials = false"
            @select-cover="(id: string) => { draftThumbMediaId = id; showMaterials = false }"
          />
        </div>
      </div>
    </Teleport>

    <!-- MP browser (Teleport) -->
    <Teleport to="body">
      <MpBrowser v-if="showMp" @close="showMp = false" />
    </Teleport>

    <footer class="app-footer flex h-7 shrink-0 items-center gap-2 px-3 text-[11px]">
      <Sparkles class="size-3" />{{ status }}
      <span class="ml-auto text-[10px] hidden sm:inline">{{ styleLabel }} · {{ latency }}ms</span>
    </footer>
  </div>
</template>
