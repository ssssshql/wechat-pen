import type { ConvertRequest, ConvertResponse, DraftItem, DraftSnapshot, MaterialListResponse } from './types'

const API = '/api/convert'
const DRAFTS_KEY = 'wechat-pen:drafts:v1'
const ACTIVE_DRAFT_KEY = 'wechat-pen:active-draft:v1'
const SETTINGS_KEY = 'wechat-pen:settings:v1'
const MAX_HISTORY = 12

export interface StyleOption {
  id: string
  name: string
  description: string
  primary: string
  builtin: boolean
}

export async function convertMarkdown(req: ConvertRequest): Promise<ConvertResponse> {
  const res = await fetch(API, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  const data = await res.json()
  if (!res.ok) {
    throw new Error(data.error || res.statusText)
  }
  return data as ConvertResponse
}

export async function fetchStyles(): Promise<StyleOption[]> {
  const res = await fetch('/api/styles')
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || res.statusText)
  return (data.styles || []) as StyleOption[]
}

export async function reloadThemes(): Promise<number> {
  const res = await fetch('/api/themes/reload', { method: 'POST' })
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || res.statusText)
  return Number(data.loaded || 0)
}

export async function importTheme(pack: Record<string, unknown>): Promise<StyleOption> {
  const res = await fetch('/api/themes/import', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(pack),
  })
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || res.statusText)
  return data.theme as StyleOption
}

export async function deleteTheme(id: string): Promise<void> {
  const res = await fetch('/api/themes/delete', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id }),
  })
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || res.statusText)
}

export interface ThemePack {
  id: string
  name: string
  description?: string
  primary?: string
  extends?: string
  base?: string
  preCode?: string
  tags?: Record<string, string>
}

export async function getTheme(id: string): Promise<ThemePack> {
  const res = await fetch(`/api/themes/get?id=${encodeURIComponent(id)}`)
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || res.statusText)
  return data.theme as ThemePack
}

export function downloadText(filename: string, content: string, mime = 'text/html;charset=utf-8') {
  const blob = new Blob([content], { type: mime })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

/** @deprecated local only — kept for one-time migration */
export function loadDrafts(): DraftItem[] {
  try {
    const raw = localStorage.getItem(DRAFTS_KEY)
    if (!raw) return []
    const list = JSON.parse(raw) as DraftItem[]
    return Array.isArray(list) ? list : []
  } catch {
    return []
  }
}

/** @deprecated local only — kept for one-time migration */
export function saveDrafts(list: DraftItem[]) {
  localStorage.setItem(DRAFTS_KEY, JSON.stringify(list))
}

/** @deprecated local only — kept for one-time migration */
export function getActiveDraftId(): string | null {
  return localStorage.getItem(ACTIVE_DRAFT_KEY)
}

/** @deprecated local only — kept for one-time migration */
export function setActiveDraftId(id: string | null) {
  if (id) localStorage.setItem(ACTIVE_DRAFT_KEY, id)
  else localStorage.removeItem(ACTIVE_DRAFT_KEY)
}

const MIGRATED_KEY = 'wechat-pen:notes-migrated:v1'

export async function fetchNotes(): Promise<{ notes: DraftItem[]; activeId: string }> {
  const res = await fetch('/api/notes')
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || res.statusText)
  return {
    notes: (data.notes || []) as DraftItem[],
    activeId: String(data.activeId || ''),
  }
}

export async function createNote(note: Partial<DraftItem>): Promise<DraftItem> {
  const res = await fetch('/api/notes', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      id: note.id,
      name: note.name || '未命名',
      markdown: note.markdown || '',
      settings: note.settings || {},
      updatedAt: note.updatedAt,
      publishStatus: note.publishStatus || 'none',
      mediaId: note.mediaId || '',
      publishedAt: note.publishedAt || 0,
      type: note.type || 'article',
    }),
  })
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || res.statusText)
  return data as DraftItem
}

export async function updateNote(
  id: string,
  patch: {
    name: string
    markdown: string
    settings?: Record<string, unknown>
    pushHistory?: boolean
    historyTitle?: string
    publishStatus?: string
    mediaId?: string
  },
): Promise<DraftItem> {
  const res = await fetch(`/api/notes/${encodeURIComponent(id)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(patch),
  })
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || res.statusText)
  return data as DraftItem
}

export async function setNotePublishStatus(
  id: string,
  publishStatus: string,
  mediaId?: string,
): Promise<DraftItem> {
  // Fetch current first so we don't wipe markdown/name
  const cur = await fetch(`/api/notes/${encodeURIComponent(id)}`)
  const note = await cur.json()
  if (!cur.ok) throw new Error(note.error || cur.statusText)
  return updateNote(id, {
    name: note.name || '未命名',
    markdown: note.markdown || '',
    settings: note.settings,
    pushHistory: false,
    publishStatus,
    mediaId: mediaId !== undefined ? mediaId : note.mediaId,
  })
}

export async function deleteNote(id: string): Promise<void> {
  const res = await fetch(`/api/notes/${encodeURIComponent(id)}`, { method: 'DELETE' })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) throw new Error((data as { error?: string }).error || res.statusText)
}

export async function setActiveNote(id: string | null): Promise<void> {
  const res = await fetch('/api/notes/active', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id: id || '' }),
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) throw new Error((data as { error?: string }).error || res.statusText)
}

export async function importNotes(
  notes: DraftItem[],
  activeId?: string | null,
): Promise<{ imported: number; activeId: string }> {
  const res = await fetch('/api/notes/import', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ notes, activeId: activeId || '' }),
  })
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || res.statusText)
  return data as { imported: number; activeId: string }
}

/** One-time: push browser localStorage drafts into SQLite, then mark migrated. */
export async function migrateLocalDraftsIfNeeded(): Promise<number> {
  if (localStorage.getItem(MIGRATED_KEY) === '1') return 0
  const local = loadDrafts()
  if (!local.length) {
    localStorage.setItem(MIGRATED_KEY, '1')
    return 0
  }
  try {
    const remote = await fetchNotes()
    if (remote.notes.length > 0) {
      // Server already has data — don't overwrite; just mark done.
      localStorage.setItem(MIGRATED_KEY, '1')
      return 0
    }
    const result = await importNotes(local, getActiveDraftId())
    localStorage.setItem(MIGRATED_KEY, '1')
    return result.imported
  } catch {
    // leave unmigrated so next load can retry
    return 0
  }
}

export function loadSettings<T extends object>(fallback: T): T {
  try {
    const raw = localStorage.getItem(SETTINGS_KEY)
    if (!raw) return fallback
    return { ...fallback, ...JSON.parse(raw) }
  } catch {
    return fallback
  }
}

export function saveSettings(settings: object) {
  localStorage.setItem(SETTINGS_KEY, JSON.stringify(settings))
}

export function newDraftId() {
  return `d_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 7)}`
}

export function pushHistory(draft: DraftItem, markdown: string, title?: string): DraftItem {
  const hist = [...(draft.history || [])]
  const last = hist[0]
  if (last && last.markdown === markdown) return draft
  const snap: DraftSnapshot = {
    id: `s_${Date.now().toString(36)}`,
    at: Date.now(),
    markdown,
    title,
  }
  hist.unshift(snap)
  if (hist.length > MAX_HISTORY) hist.length = MAX_HISTORY
  return { ...draft, history: hist }
}

export function openMpGuide() {
  window.open('https://mp.weixin.qq.com/', '_blank', 'noopener,noreferrer')
}

export async function fetchMaterials(type = 'image', offset = 0, count = 20): Promise<MaterialListResponse> {
  const params = new URLSearchParams({ type, offset: String(offset), count: String(count) })
  const res = await fetch(`/api/material/batch?${params}`)
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || res.statusText)
  return data as MaterialListResponse
}

export async function deleteMaterial(mediaId: string): Promise<void> {
  const res = await fetch('/api/material/delete', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ media_id: mediaId }),
  })
  if (!res.ok) {
    const data = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(data.error || res.statusText)
  }
}

export async function uploadMaterial(file: File): Promise<{ media_id: string; url: string }> {
  const form = new FormData()
  form.append('media', file)
  const res = await fetch('/api/material/upload', {
    method: 'POST',
    body: form,
  })
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || res.statusText)
  return data as { media_id: string; url: string }
}

export async function addDraft(req: {
  markdown: string
  title: string
  author: string
  digest: string
  thumb_media_id: string
  style: string
  primaryColor: string
  upload_images: boolean
}): Promise<{ media_id: string }> {
  const res = await fetch('/api/draft/add', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || res.statusText)
  return data as { media_id: string }
}

export interface BizItem {
  fakeid: string
  nickname: string
  alias: string
  round_head_img: string
  service_type: number
  signature: string
  verify_status: number
}

export async function searchBiz(query: string, begin = 0, count = 10): Promise<{ list: BizItem[]; total: number }> {
  const params = new URLSearchParams({ query, begin: String(begin), count: String(count) })
  const res = await fetch(`/api/biz/search?${params}`)
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || res.statusText)
  return data as { list: BizItem[]; total: number }
}

export interface PublishedArticle {
  title: string
  link: string
  cover: string
  digest: string
  create_time: number
  update_time: number
  appmsg_id: number
}

export async function fetchBizArticles(fakeid: string, begin = 0, count = 10): Promise<{ total: number; articles: PublishedArticle[] }> {
  const params = new URLSearchParams({ fakeid, begin: String(begin), count: String(count) })
  const res = await fetch(`/api/biz/articles?${params}`)
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || res.statusText)
  return data as { total: number; articles: PublishedArticle[] }
}

// --- AI ---

export interface AIConfig {
  ai_base_url: string
  ai_model: string
  has_ai_key: boolean
  ai_image_base_url: string
  ai_image_model: string
  has_ai_image_key: boolean
}

export async function getAIConfig(): Promise<AIConfig> {
  const res = await fetch('/api/ai/config')
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || res.statusText)
  return data as AIConfig
}

export async function saveAIConfig(config: {
  ai_base_url?: string; ai_api_key?: string; ai_model?: string;
  ai_image_base_url?: string; ai_image_api_key?: string; ai_image_model?: string;
}): Promise<void> {
  const res = await fetch('/api/ai/config', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(config),
  })
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || res.statusText)
}

export async function generateImage(prompt: string, n = 1, size = '1024x1024'): Promise<{ data: { url: string }[] }> {
  const res = await fetch('/api/ai/image', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ prompt, n, size }),
  })
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || data.error || res.statusText)
  return data as { data: { url: string }[] }
}

export interface ChatMessage {
  role: 'user' | 'assistant'
  content: string
}

export function aiChatStream(
  params: { messages: ChatMessage[]; currentContent: string; styleId?: string },
  onToken: (token: string) => void,
  onDone: () => void,
  onError: (err: string) => void,
): () => void {
  const controller = new AbortController()
  fetch('/api/ai/chat', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
    signal: controller.signal,
  }).then(async (res) => {
    if (!res.ok) {
      const data = await res.json().catch(() => ({ error: res.statusText }))
      onError(data.error || res.statusText)
      return
    }
    const reader = res.body?.getReader()
    if (!reader) { onError('Response body is not readable'); return }
    const decoder = new TextDecoder()
    let buf = ''
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buf += decoder.decode(value, { stream: true })
      const lines = buf.split('\n')
      buf = lines.pop() || ''
      for (const line of lines) {
        if (line.startsWith('data: ')) {
          const raw = line.slice(6)
          try {
            const obj = JSON.parse(raw)
            if (obj.done) {
              onDone()
              return
            }
            if (obj.token) onToken(obj.token)
          } catch { /* ignore parse errors */ }
        }
      }
    }
    onDone()
  }).catch((e) => {
    if (e.name !== 'AbortError') onError(e.message || String(e))
  })
  return () => controller.abort()
}

export interface WritingStyle {
  id: string
  name: string
  fakeid: string
  nickname: string
  stylePrompt: string
  sampleCount: number
  createdAt: number
  updatedAt: number
}

export async function analyzeStyle(fakeid: string, nickname: string): Promise<WritingStyle> {
  const res = await fetch('/api/ai/analyze-style', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ fakeid, nickname }),
  })
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || res.statusText)
  return data.style as WritingStyle
}

export async function fetchWritingStyles(): Promise<WritingStyle[]> {
  const res = await fetch('/api/ai/styles')
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || res.statusText)
  return (data.styles || []) as WritingStyle[]
}

export async function deleteWritingStyle(id: string): Promise<void> {
  const res = await fetch('/api/ai/styles', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id }),
  })
  const data = await res.json()
  if (!res.ok) throw new Error(data.error || res.statusText)
}



export function aiWriteStream(
  params: { styleId: string; topic: string; outline?: string; length?: string },
  onToken: (token: string) => void,
  onDone: () => void,
  onError: (err: string) => void,
): () => void {
  const controller = new AbortController()
  fetch('/api/ai/write', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
    signal: controller.signal,
  }).then(async (res) => {
    if (!res.ok) {
      const data = await res.json().catch(() => ({ error: res.statusText }))
      onError(data.error || res.statusText)
      return
    }
    const reader = res.body?.getReader()
    if (!reader) { onError('Response body is not readable'); return }
    const decoder = new TextDecoder()
    let buf = ''
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buf += decoder.decode(value, { stream: true })
      const lines = buf.split('\n')
      buf = lines.pop() || ''
      for (const line of lines) {
        if (line.startsWith('data: ')) {
          const raw = line.slice(6)
          try {
            const obj = JSON.parse(raw)
            if (obj.done) {
              onDone()
              return
            }
            if (obj.token) onToken(obj.token)
          } catch { /* ignore parse errors */ }
        }
      }
    }
    onDone()
  }).catch((err) => {
    if (err.name !== 'AbortError') onError(err.message)
  })
  return () => controller.abort()
}
