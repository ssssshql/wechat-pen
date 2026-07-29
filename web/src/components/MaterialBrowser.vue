<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { toast } from 'vue-sonner'
import { Image, Copy, Loader2, RefreshCw, ChevronLeft, ChevronRight, X, Trash2, ImageUp, Search, ArrowLeft, Rss, Sparkles } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { fetchMaterials, deleteMaterial, searchBiz, fetchBizArticles, analyzeStyle, type BizItem, type PublishedArticle } from '@/lib/api'
import type { MaterialItem } from '@/lib/types'

function proxyImg(url: string) { return url ? `/api/biz/image/proxy?url=${encodeURIComponent(url)}` : '' }

const emit = defineEmits<{ insert: [text: string]; close: []; selectCover: [mediaId: string]; styleAnalyzed: [] }>()

type Tab = 'materials' | 'biz'
const tab = ref<Tab>('materials')

// --- Materials ---
const items = ref<MaterialItem[]>([])
const totalCount = ref(0)
const offset = ref(0)
const loading = ref(false)
const deleting = ref<Set<string>>(new Set())
const selectedURL = ref<string | null>(null)
const pageSize = 20

async function loadList() {
  loading.value = true
  try {
    const data = await fetchMaterials('image', offset.value, pageSize)
    items.value = data.items
    totalCount.value = data.total_count
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '获取素材列表失败')
  } finally {
    loading.value = false
  }
}

function insertImage(item: MaterialItem) {
  selectedURL.value = item.url
  const md = `![${item.name}](${item.url})`
  emit('insert', md)
  toast.success('已插入图片')
  setTimeout(() => { selectedURL.value = null }, 1500)
}

async function doDelete(item: MaterialItem) {
  if (!window.confirm(`确定删除「${item.name}」？此操作不可撤销。`)) return
  deleting.value.add(item.media_id)
  try {
    await deleteMaterial(item.media_id)
    items.value = items.value.filter(i => i.media_id !== item.media_id)
    totalCount.value--
    toast.success('已删除')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '删除失败')
  } finally {
    deleting.value.delete(item.media_id)
  }
}

function prevPage() {
  offset.value = Math.max(0, offset.value - pageSize)
  loadList()
}
function nextPage() {
  offset.value = offset.value + pageSize
  loadList()
}

const totalPages = () => Math.max(1, Math.ceil(totalCount.value / pageSize))
const currentPage = () => Math.floor(offset.value / pageSize) + 1

// --- Biz search ---
type BizView = 'search' | 'articles' | 'viewer'
const bizView = ref<BizView>('search')
const bizQuery = ref('')
const bizResults = ref<BizItem[]>([])
const bizSearching = ref(false)
const selectedBiz = ref<BizItem | null>(null)
const bizArticles = ref<PublishedArticle[]>([])
const bizArticlesLoading = ref(false)
const bizArticlesTotal = ref(0)
const viewerUrl = ref('')
const viewerTitle = ref('')
const analyzing = ref(false)
const analyzingBizId = ref('')
const searchHistory = ref<string[]>([])

function loadSearchHistory() {
  try {
    const raw = localStorage.getItem('wechat-pen:mp-search-history')
    searchHistory.value = raw ? JSON.parse(raw) : []
  } catch { searchHistory.value = [] }
}

function saveSearchHistory(query: string) {
  if (!query.trim()) return
  const list = searchHistory.value.filter(q => q !== query.trim())
  list.unshift(query.trim())
  if (list.length > 10) list.length = 10
  searchHistory.value = list
  localStorage.setItem('wechat-pen:mp-search-history', JSON.stringify(list))
}

async function doAnalyze(fakeid: string, nickname: string) {
  analyzing.value = true
  analyzingBizId.value = fakeid
  try {
    await analyzeStyle(fakeid, nickname)
    toast.success('写作风格分析完成: ' + nickname)
    emit('styleAnalyzed')
  } catch (e: any) {
    toast.error('分析失败: ' + (e.message || e))
  } finally {
    analyzing.value = false
    analyzingBizId.value = ''
  }
}

async function onSearchBiz() {
  if (!bizQuery.value.trim()) return
  bizSearching.value = true
  saveSearchHistory(bizQuery.value)
  try { const res = await searchBiz(bizQuery.value.trim()); bizResults.value = res.list } catch (e) { toast.error(e instanceof Error ? e.message : '搜索失败') } finally { bizSearching.value = false }
}
async function onSelectBiz(biz: BizItem) {
  selectedBiz.value = biz; bizArticlesLoading.value = true; bizView.value = 'articles'
  try { const res = await fetchBizArticles(biz.fakeid); bizArticles.value = res.articles; bizArticlesTotal.value = res.total } catch (e) { toast.error(e instanceof Error ? e.message : '获取文章失败') } finally { bizArticlesLoading.value = false }
}
function openViewer(url: string, title: string) { viewerUrl.value = `/api/biz/article/proxy?url=${encodeURIComponent(url)}`; viewerTitle.value = title; bizView.value = 'viewer' }
function backToSearch() { bizView.value = 'search'; viewerUrl.value = '' }
function backToArticles() { bizView.value = 'articles'; viewerUrl.value = '' }

function switchTab(t: Tab) {
  tab.value = t
  if (t === 'biz' && bizView.value === 'search' && !bizResults.value.length) {
    // no-op, just show empty search
  }
}

onMounted(() => { loadList(); loadSearchHistory() })
</script>

<template>
  <div class="flex h-full flex-col bg-background">
    <!-- Header with tabs -->
    <div class="flex h-9 shrink-0 items-center border-b">
      <div class="flex flex-1">
        <button class="flex items-center gap-1.5 px-3 text-xs font-medium transition-colors" :class="tab === 'materials' ? 'text-foreground border-b-2 border-primary' : 'text-muted-foreground hover:text-foreground'" @click="switchTab('materials')">
          <Image class="size-3.5" />素材库
        </button>
        <button class="flex items-center gap-1.5 px-3 text-xs font-medium transition-colors" :class="tab === 'biz' ? 'text-foreground border-b-2 border-primary' : 'text-muted-foreground hover:text-foreground'" @click="switchTab('biz')">
          <Rss class="size-3.5" />公众号
        </button>
      </div>
      <Button variant="ghost" size="icon-xs" class="mr-2" @click="emit('close')">
        <X class="size-3.5" />
      </Button>
    </div>

    <!-- ======== Materials tab ======== -->
    <template v-if="tab === 'materials'">
      <!-- Toolbar -->
      <div class="flex shrink-0 items-center justify-between px-3 py-2">
        <span class="text-[11px] text-muted-foreground">
          共 {{ totalCount }} 张
          <template v-if="totalCount > 0">· 第 {{ currentPage() }}/{{ totalPages() }} 页</template>
        </span>
        <Button variant="ghost" size="icon-xs" :disabled="loading" @click="loadList">
          <RefreshCw class="size-3" :class="loading ? 'animate-spin' : ''" />
        </Button>
      </div>

      <!-- Loading -->
      <div v-if="loading" class="flex flex-1 items-center justify-center py-8">
        <Loader2 class="size-5 animate-spin text-muted-foreground" />
      </div>

      <!-- Empty -->
      <div v-else-if="!items.length" class="flex flex-1 items-center justify-center py-8">
        <p class="text-xs text-muted-foreground">暂无素材</p>
      </div>

      <!-- Grid -->
      <div v-else class="grid flex-1 auto-rows-min grid-cols-3 gap-2 overflow-auto px-3 pb-2">
        <div
          v-for="item in items"
          :key="item.media_id"
          class="group relative overflow-hidden rounded-lg border"
          :class="selectedURL === item.url ? 'ring-2 ring-primary' : 'bg-muted/30'"
        >
          <img
            :src="item.url"
            :alt="item.name"
            class="h-24 w-full object-cover"
            referrerpolicy="no-referrer"
            loading="lazy"
          />
          <div class="p-1.5">
            <p class="truncate text-[10px] font-medium">{{ item.name }}</p>
            <p class="text-[9px] text-muted-foreground">{{ new Date(item.update_time * 1000).toLocaleDateString() }}</p>
          </div>
          <!-- Overlay actions -->
          <div class="absolute inset-0 flex items-center justify-center gap-1 bg-black/0 opacity-0 transition-all group-hover:bg-black/30 group-hover:opacity-100">
            <Button
              size="xs"
              variant="secondary"
              class="h-7 gap-1 text-[10px]"
              @click="emit('selectCover', item.media_id); toast.success('已选择封面')"
            >
              <ImageUp class="size-3" />封面
            </Button>
            <Button
              size="xs"
              variant="secondary"
              class="h-7 gap-1 text-[10px]"
              @click="insertImage(item)"
            >
              <Copy class="size-3" />插入
            </Button>
            <Button
              size="icon-xs"
              variant="secondary"
              class="size-7"
              :disabled="deleting.has(item.media_id)"
              @click="doDelete(item)"
            >
              <Loader2 v-if="deleting.has(item.media_id)" class="size-3 animate-spin" />
              <Trash2 v-else class="size-3" />
            </Button>
          </div>
        </div>
      </div>

      <!-- Pagination -->
      <div v-if="totalCount > pageSize" class="flex shrink-0 items-center justify-between border-t px-3 py-2">
        <Button variant="ghost" size="xs" :disabled="offset === 0" @click="prevPage">
          <ChevronLeft class="size-3 mr-1" />上一页
        </Button>
        <Button variant="ghost" size="xs" :disabled="offset + pageSize >= totalCount" @click="nextPage">
          下一页<ChevronRight class="size-3 ml-1" />
        </Button>
      </div>
    </template>

    <!-- ======== Biz tab ======== -->
    <template v-if="tab === 'biz'">
      <!-- Article viewer -->
      <div v-if="bizView === 'viewer'" class="flex flex-1 min-h-0 flex-col">
        <div class="flex items-center gap-2 border-b px-3 py-1.5 shrink-0">
          <Button size="icon-xs" variant="ghost" @click="backToArticles"><ArrowLeft class="size-3.5" /></Button>
          <span class="text-xs font-medium truncate flex-1">{{ viewerTitle || '文章预览' }}</span>
        </div>
        <div class="flex-1 min-h-0">
          <iframe v-if="viewerUrl" :src="viewerUrl" class="w-full h-full border-0" sandbox="allow-same-origin allow-scripts" />
        </div>
      </div>

      <!-- Article list -->
      <div v-else-if="bizView === 'articles'" class="flex flex-1 min-h-0 flex-col">
        <div class="flex items-center gap-2 border-b px-3 py-1.5 shrink-0">
          <Button size="icon-xs" variant="ghost" @click="backToSearch"><ArrowLeft class="size-3.5" /></Button>
          <div class="flex items-center gap-1.5 min-w-0 flex-1">
            <img v-if="selectedBiz?.round_head_img" :src="proxyImg(selectedBiz.round_head_img)" class="size-5 rounded-full shrink-0 bg-muted" />
            <span class="text-xs font-medium truncate">{{ selectedBiz?.nickname || '文章列表' }}</span>
          </div>
          <Button variant="outline" size="xs" class="h-6 text-[10px] shrink-0 gap-1" :disabled="analyzing" @click="doAnalyze(selectedBiz!.fakeid, selectedBiz!.nickname)">
            <Loader2 v-if="analyzing" class="size-3 animate-spin" />
            <Sparkles v-else class="size-3" />
            {{ analyzing ? '分析中' : '提取风格' }}
          </Button>
          <span class="text-[10px] text-muted-foreground shrink-0">{{ bizArticlesTotal }} 篇</span>
        </div>
        <div v-if="bizArticlesLoading" class="flex flex-1 items-center justify-center py-8">
          <Loader2 class="size-5 animate-spin text-muted-foreground" />
        </div>
        <div v-else-if="bizArticles.length" class="flex-1 overflow-auto px-3 py-2 space-y-1.5">
          <button v-for="art in bizArticles" :key="art.appmsg_id"
            class="hover:bg-muted rounded-md border border-border/50 px-2.5 py-2 w-full text-left flex gap-2.5"
            @click="openViewer(art.link, art.title)">
            <div class="flex-1 min-w-0 space-y-0.5">
              <div class="text-[11px] font-medium leading-tight line-clamp-2">{{ art.title }}</div>
              <div v-if="art.digest" class="text-[9px] text-muted-foreground line-clamp-1 leading-snug">{{ art.digest }}</div>
              <div class="text-[9px] text-muted-foreground/60">{{ new Date(art.create_time * 1000).toLocaleDateString() }}</div>
            </div>
            <img v-if="art.cover" :src="proxyImg(art.cover)" class="w-16 h-16 rounded shrink-0 object-cover bg-muted" referrerpolicy="no-referrer" loading="lazy" @error="($event.target as HTMLImageElement).style.display = 'none'" />
          </button>
        </div>
        <div v-else class="flex flex-1 items-center justify-center py-8">
          <p class="text-xs text-muted-foreground">暂无文章</p>
        </div>
      </div>

      <!-- Search view -->
      <div v-else class="flex flex-1 min-h-0 flex-col overflow-auto px-3 py-2 space-y-2">
        <div class="flex gap-1">
          <Input v-model="bizQuery" placeholder="搜索公众号..." class="settings-input flex-1" @keydown.enter="onSearchBiz" />
          <Button size="xs" variant="outline" :disabled="bizSearching || !bizQuery.trim()" @click="onSearchBiz">
            <Loader2 v-if="bizSearching" class="size-3 animate-spin" />
            <Search v-else class="size-3" />
          </Button>
        </div>
        <!-- Search history -->
        <div v-if="searchHistory.length && !bizQuery" class="flex items-center gap-1 flex-wrap">
          <span class="text-[10px] text-muted-foreground shrink-0">近期:</span>
          <button
            v-for="h in searchHistory"
            :key="h"
            class="text-[10px] px-1.5 py-0.5 rounded bg-muted hover:bg-muted/80 text-muted-foreground transition-colors"
            @click="bizQuery = h; onSearchBiz()"
          >{{ h }}</button>
        </div>
        <div v-if="bizResults.length" class="space-y-1">
          <button v-for="biz in bizResults" :key="biz.fakeid"
            class="hover:bg-muted flex items-center gap-2 w-full rounded-md px-2 py-1.5 text-left"
            @click="onSelectBiz(biz)">
            <img :src="proxyImg(biz.round_head_img)" class="size-8 rounded-full shrink-0 bg-muted" />
            <div class="min-w-0 flex-1">
              <div class="text-[11px] font-medium truncate">{{ biz.nickname }}</div>
              <div class="text-[9px] text-muted-foreground line-clamp-1">{{ biz.signature }}</div>
            </div>
            <Button variant="ghost" size="icon-xs" class="shrink-0 text-muted-foreground hover:text-primary" :disabled="analyzing" @click.stop="doAnalyze(biz.fakeid, biz.nickname)">
              <Loader2 v-if="analyzing && analyzingBizId === biz.fakeid" class="size-3 animate-spin" />
              <Sparkles v-else class="size-3" />
            </Button>
          </button>
        </div>
        <div v-else-if="!bizSearching" class="flex flex-1 items-center justify-center">
          <p class="text-xs text-muted-foreground">搜索公众号，浏览已发布文章</p>
        </div>
      </div>
    </template>
  </div>
</template>
