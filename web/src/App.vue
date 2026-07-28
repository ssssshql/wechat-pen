<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import { ChevronLeft, ChevronRight, FileCode2, FolderOpen, Loader2, Menu, Plus, Settings2, Sparkles, Trash2, Upload, X, ExternalLink, Copy, Download, RotateCcw, Image, Send } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Toaster } from '@/components/ui/sonner'
import MarkdownEditor from '@/components/MarkdownEditor.vue'
import EditorToolbar from '@/components/EditorToolbar.vue'
import MaterialBrowser from '@/components/MaterialBrowser.vue'
import MpBrowser from '@/components/MpBrowser.vue'
import SettingsPanel from '@/components/SettingsPanel.vue'
import PreviewPanel from '@/components/PreviewPanel.vue'
import { convertMarkdown, downloadText, importTheme, loadSettings, openMpGuide, saveSettings, addDraft, fetchMaterials, uploadMaterial, fetchNotes, createNote, updateNote, deleteNote, setActiveNote, migrateLocalDraftsIfNeeded } from '@/lib/api'
import { SAMPLE_MD, STYLE_PRESETS, computeStats, type DraftItem, type HighlightTheme, type MaterialItem, type PreviewShell, type PreviewWidth, type StylePack, type Theme } from '@/lib/types'

const markdown = ref(SAMPLE_MD)
const theme = ref<Theme>('wechat')
const style = ref<StylePack>('simple')
const title = ref('示例文章')
const primaryColor = ref('#07c160')
const textIndent = ref(false)
const justify = ref(true)
const highlightTheme = ref<HighlightTheme>('github')
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
const mpConnected = ref(false)
const mpName = ref('')
const mpAvatar = ref('')

async function checkMpStatus() {
  try {
    const res = await fetch('/api/login/status')
    const d = await res.json()
    mpConnected.value = d.status === 'ok'
    if (d.status === 'ok') {
      const r2 = await fetch('/api/account/info')
      const d2 = await r2.json()
      if (d2.ok) { mpName.value = d2.name || ''; mpAvatar.value = d2.headimg_url || '' }
    }
  } catch {}
}
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
const drafts = ref<DraftItem[]>([])
const activeDraftId = ref<string | null>(null)
const themeInput = ref<HTMLInputElement | null>(null)
const renamingId = ref<string | null>(null)
const renameDraft = ref('')

let timer: ReturnType<typeof setTimeout> | null = null
let draftTimer: ReturnType<typeof setTimeout> | null = null

const stats = computed(() => computeStats(markdown.value, title.value))
const styleLabel = computed(() => STYLE_PRESETS.find((s) => s.value === style.value)?.label ?? style.value)

function formatNoteTime(ts: number) {
  const d = new Date(ts)
  const now = new Date()
  const sameDay = d.toDateString() === now.toDateString()
  if (sameDay) return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  const y = now.getFullYear() === d.getFullYear()
  return d.toLocaleDateString([], y ? { month: 'numeric', day: 'numeric' } : { year: 'numeric', month: 'numeric', day: 'numeric' })
}

function buildReq() {
  return {
    markdown: markdown.value, theme: theme.value, title: title.value, style: style.value,
    primaryColor: primaryColor.value, textIndent: textIndent.value, justify: justify.value,
    highlightTheme: highlightTheme.value,
    toc: false, footer: false,
    previewWidth: 'phone' as PreviewWidth, previewShell: 'dark' as PreviewShell,
  }
}

function onStyleChange(v: unknown) { style.value = String(v) }
function onPrimaryColor(v: string) { primaryColor.value = v }
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

function currentSettings() { return { theme: theme.value, style: style.value, title: title.value, primaryColor: primaryColor.value, textIndent: textIndent.value, justify: justify.value, highlightTheme: highlightTheme.value, copyMode: copyMode.value } }
function applySettings(s: Record<string, unknown>) { if (s.theme) theme.value = s.theme as Theme; if (s.style) style.value = s.style as StylePack; if (typeof s.title === 'string') title.value = s.title; if (typeof s.primaryColor === 'string') primaryColor.value = s.primaryColor; if (typeof s.textIndent === 'boolean') textIndent.value = s.textIndent; if (typeof s.justify === 'boolean') justify.value = s.justify; if (s.highlightTheme) highlightTheme.value = s.highlightTheme as HighlightTheme; if (s.copyMode === 'rich' || s.copyMode === 'source') copyMode.value = s.copyMode }

function loadSample() { markdown.value = SAMPLE_MD; schedule(); toast.message('已载入示例') }
function insertSnippet(text: string) {
  const ed = editorRef.value
  if (ed?.insertAtCursor) {
    ed.insertAtCursor(text)
  } else {
    markdown.value = `${markdown.value || ''}${text}`
  }
  schedule()
  mobileSettings.value = false
}
function onToolbar(action: string) {
  const ed = editorRef.value
  if (ed?.runToolbar) ed.runToolbar(action)
  schedule()
}
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

function noteSettings() {
  return { style: style.value, primaryColor: primaryColor.value, textIndent: textIndent.value, justify: justify.value, highlightTheme: highlightTheme.value }
}

async function persistActiveDraft() {
  try {
    if (!activeDraftId.value) {
      const n = await createNote({
        name: title.value || '未命名',
        markdown: markdown.value,
        settings: noteSettings(),
      })
      activeDraftId.value = n.id
      drafts.value = [n, ...drafts.value]
      await setActiveNote(n.id)
      return
    }
    const n = await updateNote(activeDraftId.value, {
      name: title.value || drafts.value.find((d) => d.id === activeDraftId.value)?.name || '未命名',
      markdown: markdown.value,
      settings: noteSettings(),
      pushHistory: false,
    })
    const idx = drafts.value.findIndex((d) => d.id === n.id)
    if (idx >= 0) drafts.value[idx] = n
    else drafts.value = [n, ...drafts.value]
  } catch (e) {
    console.error('save note failed', e)
  }
}

async function selectDraft(id: string) {
  if (renamingId.value && renamingId.value !== id) await commitRename()
  const d = drafts.value.find((x) => x.id === id)
  if (!d) return
  if (activeDraftId.value && activeDraftId.value !== id) {
    try {
      await updateNote(activeDraftId.value, {
        name: title.value || drafts.value.find((x) => x.id === activeDraftId.value)?.name || '未命名',
        markdown: markdown.value,
        settings: noteSettings(),
        pushHistory: false,
      })
    } catch { /* ignore */ }
  }
  activeDraftId.value = id
  void setActiveNote(id)
  markdown.value = d.markdown
  title.value = d.name || '未命名'
  if (d.settings) applySettings(d.settings as Record<string, unknown>)
  showDrafts.value = false
  schedule()
}

function startRename(id: string, e?: Event) {
  e?.stopPropagation()
  e?.preventDefault()
  const d = drafts.value.find((x) => x.id === id)
  if (!d) return
  renamingId.value = id
  renameDraft.value = d.name
  void nextTick(() => {
    const el = document.querySelector<HTMLInputElement>('input[data-note-rename]')
    el?.focus()
    el?.select()
  })
}

async function commitRename() {
  const id = renamingId.value
  if (!id) return
  const name = renameDraft.value.trim() || '未命名'
  renamingId.value = null
  const d = drafts.value.find((x) => x.id === id)
  if (!d || d.name === name) {
    if (d) d.name = name
    return
  }
  try {
    const updated = await updateNote(id, {
      name,
      markdown: id === activeDraftId.value ? markdown.value : d.markdown,
      settings: id === activeDraftId.value ? noteSettings() : (d.settings as Record<string, unknown> | undefined),
      pushHistory: false,
    })
    const idx = drafts.value.findIndex((x) => x.id === id)
    if (idx >= 0) drafts.value[idx] = { ...drafts.value[idx], ...updated, name }
    if (id === activeDraftId.value) title.value = name
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '重命名失败')
  }
}

function cancelRename() {
  renamingId.value = null
}

async function createDraft() {
  await persistActiveDraft()
  try {
    const n = await createNote({
      name: `笔记 ${drafts.value.length + 1}`,
      markdown: '# 新文章\n\n',
    })
    drafts.value = [n, ...drafts.value]
    activeDraftId.value = n.id
    await setActiveNote(n.id)
    markdown.value = n.markdown
    title.value = n.name
    showDrafts.value = false
    schedule()
    startRename(n.id)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '新建失败')
  }
}

async function deleteDraft(id: string) {
  try {
    await deleteNote(id)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '删除失败')
    return
  }
  drafts.value = drafts.value.filter((d) => d.id !== id)
  if (activeDraftId.value === id) {
    if (drafts.value[0]) {
      await selectDraft(drafts.value[0].id)
    } else {
      activeDraftId.value = null
      void setActiveNote(null)
      markdown.value = SAMPLE_MD
    }
  }
  toast.message('已删除')
}
async function onThemeFile(e: Event) { const input = e.target as HTMLInputElement; const file = input.files?.[0]; if (!file) return; try { const raw = await file.text(); const pack = JSON.parse(raw); const saved = await importTheme(pack); style.value = saved.id; if (saved.primary) primaryColor.value = saved.primary; schedule(); toast.success('已导入：' + saved.name) } catch (err) { toast.error(err instanceof Error ? err.message : '导入失败') }; input.value = '' }
function wrapSelection(before: string, after: string) { editorRef.value?.wrapSelection(before, after); schedule() }
function onKeydown(e: KeyboardEvent) {
  const mod = e.metaKey || e.ctrlKey
  if (!mod) return
  if (e.key === 'b') { e.preventDefault(); wrapSelection('**', '**') }
  else if (e.key === 'i') { e.preventDefault(); wrapSelection('*', '*') }
  else if (e.key === 'k') { e.preventDefault(); wrapSelection('[', '](https://)') }
  else if (e.shiftKey && e.key === 'C') { e.preventDefault(); copyHTML() }
  else if (e.key === 's') { e.preventDefault(); void persistActiveDraft().then(() => toast.success('已保存')) }
}

watch([markdown, theme, style, title, primaryColor, textIndent, justify, highlightTheme], () => schedule())

onMounted(async () => {
  const settings = loadSettings(currentSettings())
  applySettings(settings as Record<string, unknown>)
  window.addEventListener('keydown', onKeydown)
  checkMpStatus()
  try {
    const migrated = await migrateLocalDraftsIfNeeded()
    if (migrated > 0) toast.success(`已迁移 ${migrated} 条本地笔记到服务端`)
    const { notes, activeId } = await fetchNotes()
    drafts.value = notes
    const aid = activeId && notes.some((d) => d.id === activeId) ? activeId : notes[0]?.id
    if (aid) await selectDraft(aid)
    else runConvert()
  } catch (e) {
    status.value = e instanceof Error ? e.message : '加载笔记失败'
    runConvert()
  }
})
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
          <Button variant="ghost" size="xs" class="inline-flex lg:hidden" @click="showDrafts = !showDrafts"><FolderOpen class="size-3.5 mr-1" />笔记</Button>
          <span class="mx-1.5 hidden w-px self-stretch bg-border sm:inline" />
          <input ref="themeInput" type="file" accept="application/json,.json" class="hidden" @change="onThemeFile" />
          <Button variant="ghost" size="xs" class="hidden lg:inline-flex" @click="themeInput?.click()">导入主题</Button>
          <Button variant="ghost" size="xs" class="hidden md:inline-flex" @click="loadSample"><RotateCcw class="size-3.5 mr-1" />示例</Button>
          <Button variant="ghost" size="xs" class="hidden sm:inline-flex" @click="showMaterials = !showMaterials"><Image class="size-3.5 mr-1" />素材</Button>
          <Button variant="ghost" size="xs" class="hidden sm:inline-flex gap-1.5" @click="showMp = !showMp" title="连接微信">
            <img v-if="mpConnected && mpAvatar" :src="mpAvatar" class="size-4 rounded-full object-cover" />
            <svg v-else viewBox="0 0 24 24" class="size-4" :fill="'#07c160'"><path d="M8.691 2.188C3.891 2.188 0 5.476 0 9.53c0 2.212 1.17 4.203 3.002 5.55a.59.59 0 0 1 .213.665l-.39 1.48c-.019.07-.048.141-.048.213 0 .163.13.295.29.295a.326.326 0 0 0 .167-.054l1.903-1.114a.864.864 0 0 1 .717-.098 10.16 10.16 0 0 0 2.837.403c.276 0 .543-.027.811-.05-.857-2.578.157-4.972 1.932-6.446 1.703-1.415 3.882-1.98 5.853-1.838-.576-3.583-4.196-6.348-8.596-6.348zM5.785 5.991c.642 0 1.162.529 1.162 1.18a1.17 1.17 0 0 1-1.162 1.178A1.17 1.17 0 0 1 4.623 7.17c0-.651.52-1.18 1.162-1.18zm5.813 0c.642 0 1.162.529 1.162 1.18a1.17 1.17 0 0 1-1.162 1.178 1.17 1.17 0 0 1-1.162-1.178c0-.651.52-1.18 1.162-1.18zm5.34 2.867c-1.797-.052-3.746.512-5.28 1.786-1.72 1.428-2.687 3.72-1.78 6.22.942 2.453 3.666 4.229 6.884 4.229.826 0 1.622-.12 2.361-.336a.722.722 0 0 1 .598.082l1.584.926a.272.272 0 0 0 .14.047c.134 0 .24-.11.24-.245 0-.06-.023-.12-.038-.177l-.327-1.233a.582.582 0 0 1-.023-.156.49.49 0 0 1 .201-.398C23.024 18.48 24 16.82 24 14.98c0-3.21-2.931-5.837-7.062-6.122zm-2.18 2.769c.535 0 .969.44.969.982a.976.976 0 0 1-.969.983.976.976 0 0 1-.969-.983c0-.542.434-.982.97-.982zm4.844 0c.535 0 .969.44.969.982a.976.976 0 0 1-.969.983.976.976 0 0 1-.969-.983c0-.542.434-.982.97-.982z" /></svg>
            <span v-if="mpConnected && mpName" class="text-[11px]">{{ mpName }}</span>
            <span v-else class="text-[11px]">连接微信</span>
          </Button>
          <span class="mx-1.5 hidden w-px self-stretch bg-border sm:inline" />
          <Button variant="ghost" size="xs" @click="downloadHTML"><Download class="size-3.5 mr-1" />下载</Button>
          <Button size="xs" @click="copyHTML"><Copy class="size-3.5 mr-1" />复制</Button>
          <Button variant="secondary" size="xs" @click="showDraftDialog = true"><Send class="size-3.5 mr-1" />发布草稿</Button>
          <Button variant="ghost" size="icon-xs" title="公众号后台" @click="openMpGuide"><ExternalLink class="size-3" /></Button>
        </div>
      </div>
    </header>
    <!-- Notes drawer: only when left rail is hidden -->
    <div v-if="showDrafts" class="absolute top-10 right-0 left-0 z-30 max-h-72 overflow-auto border-b border-[#eaeaea] bg-[#fbfbfa] p-2 shadow-sm lg:hidden">
      <div class="mb-2 flex items-center justify-between px-1.5">
        <span class="text-[11px] font-medium text-[#787774]">笔记</span>
        <div class="flex gap-0.5">
          <button type="button" class="flex size-7 items-center justify-center rounded-md text-[#787774] hover:bg-black/[0.04] hover:text-[#2f3437]" title="新建" @click="createDraft"><Plus class="size-3.5" /></button>
          <button type="button" class="flex size-7 items-center justify-center rounded-md text-[#787774] hover:bg-black/[0.04]" @click="showDrafts = false"><X class="size-3.5" /></button>
        </div>
      </div>
      <div v-if="!drafts.length" class="px-2 py-6 text-center text-[11px] text-[#787774]">暂无笔记</div>
      <ul class="space-y-px">
        <li
          v-for="d in drafts" :key="d.id"
          class="group flex items-center gap-1 rounded-md px-2 py-1.5"
          :class="d.id === activeDraftId ? 'bg-[#edf3ec]' : 'hover:bg-black/[0.03]'"
        >
          <div class="min-w-0 flex-1" @click="renamingId !== d.id && selectDraft(d.id)" @dblclick="startRename(d.id, $event)">
            <input
              v-if="renamingId === d.id"
              data-note-rename
              v-model="renameDraft"
              class="h-6 w-full rounded border border-[#eaeaea] bg-white px-1.5 text-[12px] text-[#2f3437] outline-none focus:border-[#07c160]"
              @click.stop
              @keydown.enter.prevent="commitRename"
              @keydown.esc.prevent="cancelRename"
              @blur="commitRename"
            />
            <div v-else class="flex min-w-0 items-baseline justify-between gap-2">
              <span class="truncate text-[12px] font-medium text-[#2f3437]">{{ d.name || '未命名' }}</span>
              <span class="shrink-0 font-mono text-[10px] text-[#a0a09a]">{{ formatNoteTime(d.updatedAt) }}</span>
            </div>
          </div>
          <button type="button" class="flex size-6 shrink-0 items-center justify-center rounded text-[#a0a09a] opacity-0 hover:bg-black/[0.04] hover:text-[#9f2f2d] group-hover:opacity-100" title="删除" @click.stop="deleteDraft(d.id)">
            <Trash2 class="size-3" />
          </button>
        </li>
      </ul>
    </div>

    <!-- Main -->
    <div class="flex min-h-0 flex-1 overflow-hidden">
      <!-- Notes rail: flat file-list, no card chrome -->
      <aside class="note-rail hidden w-[200px] shrink-0 flex-col border-r border-[#eaeaea] bg-[#fbfbfa] lg:flex">
        <div class="flex h-9 shrink-0 items-center gap-1 px-2.5">
          <span class="min-w-0 flex-1 text-[11px] font-medium tracking-[0.04em] text-[#787774]">笔记</span>
          <button
            type="button"
            class="flex size-6 items-center justify-center rounded-md text-[#787774] transition-colors hover:bg-black/[0.05] hover:text-[#2f3437]"
            title="新建笔记"
            @click="createDraft"
          >
            <Plus class="size-3.5 stroke-[2]" />
          </button>
        </div>
        <div class="scroll-panel min-h-0 flex-1 px-1.5 pb-3">
          <div v-if="!drafts.length" class="px-2 py-10 text-center text-[11px] leading-relaxed text-[#a0a09a]">
            暂无笔记
          </div>
          <ul v-else class="flex flex-col gap-px">
            <li
              v-for="d in drafts"
              :key="d.id"
              class="note-item group relative flex cursor-default items-center rounded-md transition-colors"
              :class="d.id === activeDraftId ? 'is-active' : ''"
              @click="renamingId !== d.id && selectDraft(d.id)"
              @dblclick="startRename(d.id, $event)"
            >
              <div class="min-w-0 flex-1 py-1.5 pr-6 pl-2.5">
                <input
                  v-if="renamingId === d.id"
                  data-note-rename
                  v-model="renameDraft"
                  class="h-6 w-full rounded border border-[#eaeaea] bg-white px-1.5 text-[12px] font-medium text-[#2f3437] outline-none focus:border-[#07c160]"
                  @click.stop
                  @keydown.enter.prevent="commitRename"
                  @keydown.esc.prevent="cancelRename"
                  @blur="commitRename"
                />
                <template v-else>
                  <div class="truncate text-[12.5px] leading-5 font-medium text-[#2f3437]">{{ d.name || '未命名' }}</div>
                  <div class="mt-0.5 font-mono text-[10px] leading-none tracking-tight text-[#a0a09a] tabular-nums">{{ formatNoteTime(d.updatedAt) }}</div>
                </template>
              </div>
              <button
                type="button"
                class="absolute top-1/2 right-1 flex size-6 -translate-y-1/2 items-center justify-center rounded-md text-[#a0a09a] opacity-0 transition-opacity hover:bg-black/[0.05] hover:text-[#9f2f2d] group-hover:opacity-100"
                title="删除"
                @click.stop="deleteDraft(d.id)"
              >
                <Trash2 class="size-3" />
              </button>
            </li>
          </ul>
        </div>
      </aside>

      <section class="editor-pane flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden border-r">
        <div class="text-muted-foreground flex h-8 shrink-0 items-center justify-between border-b px-3 text-[11px]">
          <span class="flex items-center gap-1.5 font-medium tracking-wide"><FileCode2 class="size-3.5" />Markdown</span>
          <span class="tabular-nums">{{ stats.total }} 字 · {{ stats.readingMin }}分 <span v-if="stats.titleWarn" class="text-amber-500 ml-1">标题 {{ stats.titleLen }}字</span></span>
        </div>
        <MarkdownEditor ref="editorRef" v-model="markdown" class="flex-1" />
        <EditorToolbar v-model:style="style" v-model:highlight-theme="highlightTheme" v-model:text-indent="textIndent" v-model:justify="justify" @action="onToolbar" @insert="insertSnippet" @style-change="onStyleChange" @primary-color="onPrimaryColor" />
      </section>

      <section class="preview-pane flex min-h-0 w-[460px] shrink-0 flex-col overflow-hidden">
        <PreviewPanel v-model:view="view" :preview="preview" :html="html" />
      </section>
    </div>

    <!-- Mobile settings -->
    <div v-if="mobileSettings" class="fixed inset-0 z-40 bg-black/40 lg:hidden" @click.self="mobileSettings = false">
      <div class="bg-background absolute inset-y-0 left-0 flex w-[min(100%,300px)] flex-col shadow-xl">
        <div class="flex h-10 items-center justify-between border-b px-3"><div class="flex items-center gap-1.5 text-sm font-medium"><Settings2 class="size-4" />设置</div><Button size="icon-xs" variant="ghost" @click="mobileSettings = false"><X class="size-3.5" /></Button></div>
        <div class="scroll-panel flex-1"><SettingsPanel v-model:style="style" v-model:primary-color="primaryColor" v-model:highlight-theme="highlightTheme" @style-change="onStyleChange" @insert="insertSnippet" /></div>
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
      <MpBrowser v-if="showMp" :open="showMp" @close="showMp = false" @login="checkMpStatus" />
    </Teleport>

    <footer class="app-footer flex h-7 shrink-0 items-center gap-2 px-3 text-[11px]">
      <Sparkles class="size-3" />{{ status }}
      <span class="ml-auto text-[10px] hidden sm:inline">{{ styleLabel }} · {{ latency }}ms</span>
    </footer>
  </div>
</template>
