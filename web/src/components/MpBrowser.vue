<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref } from 'vue'
import { toast } from 'vue-sonner'
import { X, Loader2, Rss, ExternalLink, Search, ArrowLeft } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { searchBiz, fetchBizArticles, type BizItem, type PublishedArticle } from '@/lib/api'

function proxyImg(url: string) { return url ? `/api/biz/image/proxy?url=${encodeURIComponent(url)}` : '' }

const emit = defineEmits<{ close: [] }>()

// --- Credentials ---
const appID = ref('')
const appSecret = ref('')
const outboundIP = ref('')

async function fetchCreds() {
  try { const res = await fetch('/api/credentials'); const d = await res.json(); if (d.appid) appID.value = d.appid; if (d.secret) appSecret.value = d.secret } catch {}
}
async function onSaveCreds() {
  try {
    const res = await fetch('/api/credentials', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ appid: appID.value, secret: appSecret.value }) })
    const d = await res.json(); if (res.ok) toast.success('凭据已保存'); else toast.error(d.error || '保存失败')
  } catch { toast.error('保存失败') }
}
async function fetchOutboundIP() {
  try { const res = await fetch('/api/ip'); const d = await res.json(); if (d.ip) outboundIP.value = d.ip } catch {}
}
async function copyIP() {
  if (!outboundIP.value) return; try { await navigator.clipboard.writeText(outboundIP.value); toast.success('已复制 IP') } catch { toast.error('复制失败') }
}
async function copyValue(text: string) {
  try { await navigator.clipboard.writeText(text); toast.success('已复制') } catch { toast.error('复制失败') }
}

// --- Login ---
const loginStatus = ref<'idle' | 'loading' | 'waiting' | 'ok' | 'error'>('idle')
const loginQRCode = ref('')
const loginError = ref('')
const loginCookie = ref('')
const loginToken = ref('')
const loginFingerprint = ref('')
let loginTimer: ReturnType<typeof setInterval> | null = null

async function checkLoginStatus() {
  try {
    const res = await fetch('/api/login/status'); const d = await res.json()
    if (d.status === 'ok') { loginStatus.value = 'ok'; loginCookie.value = d.cookies || ''; loginToken.value = d.token || ''; loginFingerprint.value = d.fingerprint || '' }
  } catch {}
}
async function startLogin() {
  loginStatus.value = 'loading'; loginQRCode.value = ''; loginError.value = ''
  try {
    const res = await fetch('/api/login/start', { method: 'POST' }); const d = await res.json()
    if (!res.ok) throw new Error(d.error || '失败')
    if (d.status === 'already_logged_in') { loginStatus.value = 'ok'; toast.success('已有登录态'); return }
    loginQRCode.value = d.qrcode_b64; loginStatus.value = 'waiting'; pollLoginStatus()
  } catch (e) { loginStatus.value = 'error'; loginError.value = e instanceof Error ? e.message : '获取二维码失败' }
}
function pollLoginStatus() {
  if (loginTimer) clearInterval(loginTimer)
  loginTimer = setInterval(async () => {
    try {
      const res = await fetch('/api/login/status'); const d = await res.json()
      if (d.status === 'ok') { loginStatus.value = 'ok'; loginCookie.value = d.cookies || ''; loginToken.value = d.token || ''; loginFingerprint.value = d.fingerprint || ''; if (loginTimer) { clearInterval(loginTimer); loginTimer = null }; toast.success('登录成功') }
      else if (d.status === 'error') { loginStatus.value = 'error'; loginError.value = d.error || '登录失败'; if (loginTimer) { clearInterval(loginTimer); loginTimer = null } }
    } catch {}
  }, 2000)
}
async function cancelLogin() { if (loginTimer) { clearInterval(loginTimer); loginTimer = null }; loginStatus.value = 'idle'; try { await fetch('/api/login/cancel') } catch {} }
async function logoutLogin() { loginStatus.value = 'idle'; loginQRCode.value = ''; loginCookie.value = ''; loginToken.value = ''; loginFingerprint.value = ''; if (loginTimer) { clearInterval(loginTimer); loginTimer = null }; try { await fetch('/api/login/logout') } catch {}; toast.message('已退出登录') }

// --- Biz Search ---
const bizQuery = ref('')
const bizResults = ref<BizItem[]>([])
const bizSearching = ref(false)
const selectedBiz = ref<BizItem | null>(null)
const bizArticles = ref<PublishedArticle[]>([])
const bizArticlesLoading = ref(false)
const bizArticlesTotal = ref(0)
const viewerUrl = ref('')
const viewerTitle = ref('')

type View = 'main' | 'articles' | 'viewer'
const view = ref<View>('main')

async function onSearchBiz() {
  if (!bizQuery.value.trim()) return; bizSearching.value = true
  try { const res = await searchBiz(bizQuery.value.trim()); bizResults.value = res.list } catch (e) { toast.error(e instanceof Error ? e.message : '搜索失败') } finally { bizSearching.value = false }
}
async function onSelectBiz(biz: BizItem) {
  selectedBiz.value = biz; bizArticlesLoading.value = true; view.value = 'articles'
  try { const res = await fetchBizArticles(biz.fakeid); bizArticles.value = res.articles; bizArticlesTotal.value = res.total } catch (e) { toast.error(e instanceof Error ? e.message : '获取文章失败') } finally { bizArticlesLoading.value = false }
}
function openViewer(url: string, title: string) { viewerUrl.value = `/api/biz/article/proxy?url=${encodeURIComponent(url)}`; viewerTitle.value = title; view.value = 'viewer' }
function backToArticles() { view.value = 'articles'; viewerUrl.value = '' }
function backToMain() { view.value = 'main'; selectedBiz.value = null; bizArticles.value = [] }

onMounted(() => { fetchCreds(); fetchOutboundIP(); checkLoginStatus() })
onBeforeUnmount(() => { if (loginTimer) clearInterval(loginTimer) })
</script>

<template>
  <div class="fixed inset-0 z-[99999] flex justify-end" @click.self="emit('close')">
    <div class="bg-background w-[min(100%,480px)] flex flex-col shadow-2xl border-l">
      <!-- Header -->
      <div class="flex items-center gap-2 border-b px-3 h-10 shrink-0">
        <Rss class="size-4 text-primary" />
        <span class="text-sm font-semibold flex-1 truncate">
          <template v-if="view === 'viewer'">{{ viewerTitle || '文章预览' }}</template>
          <template v-else-if="view === 'articles'">{{ selectedBiz?.nickname || '文章列表' }}</template>
          <template v-else>公众号</template>
        </span>
        <Button v-if="view !== 'main'" size="icon-xs" variant="ghost" @click="view === 'viewer' ? backToArticles() : backToMain()"><ArrowLeft class="size-3.5" /></Button>
        <Button size="icon-xs" variant="ghost" @click="emit('close')"><X class="size-3.5" /></Button>
      </div>

      <!-- Viewer (iframe) -->
      <div v-if="view === 'viewer'" class="flex-1 min-h-0">
        <iframe v-if="viewerUrl" :src="viewerUrl" class="w-full h-full border-0" sandbox="allow-same-origin allow-scripts" />
      </div>

      <!-- Main view -->
      <div v-else-if="view === 'main'" class="flex-1 overflow-auto px-3 py-3 space-y-4">
        <!-- Search -->
        <div class="space-y-2">
          <div class="text-[10px] font-semibold tracking-widest uppercase text-muted-foreground">公众号搜索</div>
          <div class="flex gap-1">
            <Input v-model="bizQuery" placeholder="搜索公众号..." class="settings-input flex-1" @keydown.enter="onSearchBiz" />
            <Button size="xs" variant="outline" :disabled="bizSearching || !bizQuery.trim()" @click="onSearchBiz">
              <Loader2 v-if="bizSearching" class="size-3 animate-spin" />
              <Search v-else class="size-3" />
            </Button>
          </div>
          <div v-if="bizResults.length" class="space-y-1 max-h-64 overflow-auto">
            <button v-for="biz in bizResults" :key="biz.fakeid"
              class="hover:bg-muted flex items-center gap-2 w-full rounded-md px-2 py-1.5 text-left"
              @click="onSelectBiz(biz)">
              <img :src="proxyImg(biz.round_head_img)" class="size-7 rounded-full shrink-0 bg-muted" />
              <div class="min-w-0 flex-1">
                <div class="text-[11px] font-medium truncate">{{ biz.nickname }}</div>
                <div class="text-[9px] text-muted-foreground truncate">{{ biz.signature }}</div>
              </div>
            </button>
          </div>
        </div>

        <!-- Login -->
        <div class="space-y-2">
          <div class="text-[10px] font-semibold tracking-widest uppercase text-muted-foreground">扫码登录</div>
          <p class="text-[10px] text-muted-foreground leading-relaxed">扫码后可获取后台 Cookie，用于直接发布文章。</p>

          <div v-if="loginStatus === 'idle' || loginStatus === 'loading' || loginStatus === 'waiting'">
            <Button size="xs" variant="outline" class="w-full text-[11px] h-7 rounded-lg" :disabled="loginStatus === 'loading' || loginStatus === 'waiting'" @click="startLogin">
              <Loader2 v-if="loginStatus === 'loading'" class="size-3 mr-1 animate-spin" />
              {{ loginStatus === 'waiting' ? '等待扫码...' : '获取登录二维码' }}
            </Button>
            <div v-if="loginStatus === 'waiting' && loginQRCode" class="mt-2 flex justify-center rounded-lg border bg-white p-3">
              <img :src="loginQRCode" class="h-48 w-48" />
            </div>
            <p v-if="loginStatus === 'waiting'" class="text-[10px] text-muted-foreground text-center mt-1">请用微信扫描二维码</p>
            <p v-if="loginStatus === 'waiting'" class="text-center mt-1">
              <button class="text-[10px] text-muted-foreground hover:text-foreground" @click="cancelLogin">取消</button>
            </p>
          </div>

          <div v-else-if="loginStatus === 'ok'" class="space-y-2">
            <div class="rounded-md bg-green-50 px-2.5 py-2 flex items-center justify-between">
              <span class="text-[11px] text-green-700 font-medium">已登录</span>
              <button class="text-[10px] text-muted-foreground hover:text-red-500" @click="logoutLogin">退出</button>
            </div>
            <div v-if="loginCookie" class="rounded-md bg-accent/40 px-2.5 py-2 space-y-1">
              <div class="flex items-center justify-between"><span class="text-[10px] font-medium text-muted-foreground">Cookie</span><button class="text-[10px] text-primary hover:underline" @click="copyValue(loginCookie)">复制</button></div>
              <p class="text-[9px] font-mono text-muted-foreground break-all leading-relaxed max-h-16 overflow-auto">{{ loginCookie }}</p>
            </div>
            <div v-if="loginToken" class="rounded-md bg-accent/40 px-2.5 py-2 space-y-1">
              <div class="flex items-center justify-between"><span class="text-[10px] font-medium text-muted-foreground">Token</span><button class="text-[10px] text-primary hover:underline" @click="copyValue(loginToken)">复制</button></div>
              <p class="text-[11px] font-mono text-foreground">{{ loginToken }}</p>
            </div>
            <div v-if="loginFingerprint" class="rounded-md bg-accent/40 px-2.5 py-2 space-y-1">
              <div class="flex items-center justify-between"><span class="text-[10px] font-medium text-muted-foreground">Fingerprint</span><button class="text-[10px] text-primary hover:underline" @click="copyValue(loginFingerprint)">复制</button></div>
              <p class="text-[11px] font-mono text-foreground">{{ loginFingerprint }}</p>
            </div>
          </div>

          <div v-else-if="loginStatus === 'error'" class="rounded-md bg-red-50 px-2.5 py-2">
            <p class="text-[11px] text-red-600">{{ loginError || '登录失败' }}</p>
            <button class="text-[10px] text-primary mt-1" @click="loginStatus = 'idle'; loginError = ''">重试</button>
          </div>
        </div>

        <!-- Credentials -->
        <div class="space-y-2">
          <div class="text-[10px] font-semibold tracking-widest uppercase text-muted-foreground">配置</div>
          <Input v-model="appID" placeholder="AppID" class="settings-input" />
          <div class="settings-input"><input v-model="appSecret" placeholder="AppSecret" type="password" /></div>
          <Button size="xs" variant="outline" class="w-full text-[11px] h-7 rounded-lg" @click="onSaveCreds">保存凭据</Button>
          <div v-if="outboundIP" class="flex items-center justify-between rounded-md bg-accent/50 px-2.5 py-1.5">
            <span class="text-[10px] text-muted-foreground">出口 IP</span>
            <button class="text-[11px] font-mono font-medium text-primary hover:underline" @click="copyIP">{{ outboundIP }}</button>
          </div>
        </div>
      </div>

      <!-- Articles view -->
      <div v-else-if="view === 'articles'" class="flex-1 overflow-auto px-3 py-3 space-y-2">
        <div v-if="bizArticlesLoading" class="flex items-center justify-center py-8">
          <Loader2 class="size-5 animate-spin text-muted-foreground" />
        </div>
        <div v-else-if="bizArticles.length" class="space-y-1.5">
          <div class="text-[10px] text-muted-foreground mb-1">共 {{ bizArticlesTotal }} 篇</div>
          <button v-for="art in bizArticles" :key="art.appmsg_id"
            class="hover:bg-muted rounded-md border border-border/50 px-2.5 py-2 space-y-0.5 w-full text-left"
            @click="openViewer(art.link, art.title)">
            <div class="text-[11px] font-medium leading-tight line-clamp-2">{{ art.title }}</div>
            <div v-if="art.digest" class="text-[9px] text-muted-foreground line-clamp-2 leading-snug">{{ art.digest }}</div>
            <div class="text-[9px] text-muted-foreground/60">{{ new Date(art.create_time * 1000).toLocaleDateString() }}</div>
          </button>
        </div>
        <div v-else class="text-[10px] text-muted-foreground text-center py-8">暂无文章</div>
      </div>
    </div>
  </div>
</template>
