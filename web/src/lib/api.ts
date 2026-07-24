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

export function saveDrafts(list: DraftItem[]) {
  localStorage.setItem(DRAFTS_KEY, JSON.stringify(list))
}

export function getActiveDraftId(): string | null {
  return localStorage.getItem(ACTIVE_DRAFT_KEY)
}

export function setActiveDraftId(id: string | null) {
  if (id) localStorage.setItem(ACTIVE_DRAFT_KEY, id)
  else localStorage.removeItem(ACTIVE_DRAFT_KEY)
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
