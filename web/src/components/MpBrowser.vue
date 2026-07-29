<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import { X, Loader2, ShieldCheck, ChevronDown, Info } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: []; login: [] }>()

// WeChat icon SVG path
const wechatPath = 'M8.691 2.188C3.891 2.188 0 5.476 0 9.53c0 2.212 1.17 4.203 3.002 5.55a.59.59 0 0 1 .213.665l-.39 1.48c-.019.07-.048.141-.048.213 0 .163.13.295.29.295a.326.326 0 0 0 .167-.054l1.903-1.114a.864.864 0 0 1 .717-.098 10.16 10.16 0 0 0 2.837.403c.276 0 .543-.027.811-.05-.857-2.578.157-4.972 1.932-6.446 1.703-1.415 3.882-1.98 5.853-1.838-.576-3.583-4.196-6.348-8.596-6.348zM5.785 5.991c.642 0 1.162.529 1.162 1.18a1.17 1.17 0 0 1-1.162 1.178A1.17 1.17 0 0 1 4.623 7.17c0-.651.52-1.18 1.162-1.18zm5.813 0c.642 0 1.162.529 1.162 1.18a1.17 1.17 0 0 1-1.162 1.178 1.17 1.17 0 0 1-1.162-1.178c0-.651.52-1.18 1.162-1.18zm5.34 2.867c-1.797-.052-3.746.512-5.28 1.786-1.72 1.428-2.687 3.72-1.78 6.22.942 2.453 3.666 4.229 6.884 4.229.826 0 1.622-.12 2.361-.336a.722.722 0 0 1 .598.082l1.584.926a.272.272 0 0 0 .14.047c.134 0 .24-.11.24-.245 0-.06-.023-.12-.038-.177l-.327-1.233a.582.582 0 0 1-.023-.156.49.49 0 0 1 .201-.398C23.024 18.48 24 16.82 24 14.98c0-3.21-2.931-5.837-7.062-6.122zm-2.18 2.769c.535 0 .969.44.969.982a.976.976 0 0 1-.969.983.976.976 0 0 1-.969-.983c0-.542.434-.982.97-.982zm4.844 0c.535 0 .969.44.969.982a.976.976 0 0 1-.969.983.976.976 0 0 1-.969-.983c0-.542.434-.982.97-.982z'

// --- Account info ---
const wechatName = ref('')
const wechatAvatar = ref('')

async function fetchAccountInfo() {
  try {
    const res = await fetch('/api/account/info')
    const d = await res.json()
    if (d.ok) { wechatName.value = d.name || ''; wechatAvatar.value = d.headimg_url || '' }
  } catch {}
}

// --- Credentials / call mode ---
const appID = ref('')
const appSecret = ref('')
const callMode = ref<'local' | 'proxy'>('local')
const proxyBaseURL = ref('')
const proxyAPIKey = ref('')
const testingProxy = ref(false)
const checkingIP = ref(false)
const checkedRealIP = ref('')
const showCreds = ref(false)

async function fetchCreds() {
  try {
    const res = await fetch('/api/credentials')
    const d = await res.json()
    if (d.appid) appID.value = d.appid
    if (d.secret) appSecret.value = d.secret
    if (d.mode === 'proxy' || d.mode === 'local') callMode.value = d.mode
    if (d.proxy_base_url) proxyBaseURL.value = d.proxy_base_url
    if (d.proxy_api_key) proxyAPIKey.value = d.proxy_api_key
  } catch {}
}
async function onSaveCreds() {
  try {
    const body: Record<string, string> = {
      mode: callMode.value,
      proxy_base_url: proxyBaseURL.value,
      proxy_api_key: proxyAPIKey.value,
    }
    if (callMode.value === 'local') {
      body.appid = appID.value
      body.secret = appSecret.value
    }
    const res = await fetch('/api/credentials', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
    const d = await res.json()
    if (res.ok) toast.success(callMode.value === 'proxy' ? '已切换为远程代理' : '凭据已保存')
    else toast.error(d.error || '保存失败')
  } catch { toast.error('保存失败') }
}
async function testProxy() {
  testingProxy.value = true
  try {
    const res = await fetch('/api/proxy/test', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ proxy_base_url: proxyBaseURL.value, proxy_api_key: proxyAPIKey.value }),
    })
    const d = await res.json()
    if (res.ok) toast.success('代理连接正常')
    else toast.error(d.error || '连接失败')
  } catch (e) {
    toast.error('连接失败: ' + (e instanceof Error ? e.message : String(e)))
  } finally {
    testingProxy.value = false
  }
}
async function checkIP() {
  checkingIP.value = true; checkedRealIP.value = ''
  try {
    const res = await fetch('/api/check_ip')
    const d = await res.json()
    if (!res.ok) { toast.error(d.error || '检查失败'); return }
    if (d.ok) { toast.success('当前 IP 已在白名单') }
    else if (d.real_ip) { checkedRealIP.value = d.real_ip }
    else { toast.error(d.error || '检查失败') }
  } catch (e) { toast.error('检查失败: ' + (e instanceof Error ? e.message : String(e))) } finally { checkingIP.value = false }
}


const loginStatus = ref<'idle' | 'loading' | 'waiting' | 'ok' | 'error'>('idle')
const loginQRCode = ref('')
const loginError = ref('')
const loginCookie = ref('')
const loginToken = ref('')
const loginFingerprint = ref('')
const loginQRStatus = ref(0) // 0=waiting, 3=expired, 4=scanned
const loginMessage = ref('')
let loginEventSource: EventSource | null = null
let loginStarted = false

async function checkLoginStatus() {
  try {
    const res = await fetch('/api/login/status'); const d = await res.json()
    if (d.status === 'ok') { loginStatus.value = 'ok'; loginCookie.value = d.cookies || ''; loginToken.value = d.token || ''; loginFingerprint.value = d.fingerprint || ''; fetchAccountInfo() }
  } catch {}
}
async function startLogin() {
  if (loginStarted) return
  loginStarted = true
  loginStatus.value = 'loading'; loginQRCode.value = ''; loginError.value = ''
  try {
    const res = await fetch('/api/login/start', { method: 'POST' }); const d = await res.json()
    if (!res.ok) throw new Error(d.error || '失败')
    if (d.status === 'already_logged_in') { loginStatus.value = 'ok'; toast.success('已有登录态'); fetchAccountInfo(); emit('login'); return }
    loginQRCode.value = d.qrcode_b64; loginStatus.value = 'waiting'
    connectLoginSSE()
  } catch (e) { loginStatus.value = 'error'; loginError.value = e instanceof Error ? e.message : '获取二维码失败' }
}
function connectLoginSSE() {
  if (loginEventSource) { loginEventSource.close(); loginEventSource = null }
  loginEventSource = new EventSource('/api/login/events')

  loginEventSource.addEventListener('state', (e) => {
    const d = JSON.parse((e as MessageEvent).data)
    if (d.qr_status !== undefined) loginQRStatus.value = d.qr_status
    if (d.message !== undefined) loginMessage.value = d.message
    if (d.qrcode) loginQRCode.value = d.qrcode
    if (d.status === 'ok') {
      loginStatus.value = 'ok'
      loginCookie.value = d.cookies || ''
      loginToken.value = d.token || ''
      loginFingerprint.value = d.fingerprint || ''
      toast.success('登录成功')
      fetchAccountInfo()
      emit('login')
      loginEventSource?.close(); loginEventSource = null
    }
    if (d.status === 'error') {
      loginStatus.value = 'error'
      loginError.value = d.error || '登录失败'
      loginEventSource?.close(); loginEventSource = null
    }
  })

  loginEventSource.addEventListener('qrcode', (e) => {
    const d = JSON.parse((e as MessageEvent).data)
    if (d.qrcode) loginQRCode.value = d.qrcode
    if (d.qr_status !== undefined) loginQRStatus.value = d.qr_status
    if (d.message !== undefined) loginMessage.value = d.message
  })

  loginEventSource.addEventListener('credentials', (e) => {
    const d = JSON.parse((e as MessageEvent).data)
    loginStatus.value = 'ok'
    loginCookie.value = d.cookies || ''
    loginToken.value = d.token || ''
    loginFingerprint.value = d.fingerprint || ''
    toast.success('登录成功')
    fetchAccountInfo()
    emit('login')
    loginEventSource?.close(); loginEventSource = null
  })

  loginEventSource.addEventListener('login_error', (e) => {
    const d = JSON.parse((e as MessageEvent).data)
    loginStatus.value = 'error'
    loginError.value = d.error || '登录失败'
    loginEventSource?.close(); loginEventSource = null
  })

  loginEventSource.addEventListener('cancel', () => {
    loginStatus.value = 'idle'; loginStarted = false
    loginEventSource?.close(); loginEventSource = null
  })
}
async function cancelLogin() {
  if (loginEventSource) { loginEventSource.close(); loginEventSource = null }
  loginStatus.value = 'idle'; loginStarted = false; loginQRStatus.value = 0; loginMessage.value = ''
  try { await fetch('/api/login/cancel') } catch {}
}
async function logoutLogin() {
  if (loginEventSource) { loginEventSource.close(); loginEventSource = null }
  loginStatus.value = 'idle'; loginQRCode.value = ''; loginCookie.value = ''; loginToken.value = ''; loginFingerprint.value = ''; loginStarted = false; loginQRStatus.value = 0; loginMessage.value = ''
  wechatName.value = ''; wechatAvatar.value = ''
  try { await fetch('/api/login/logout') } catch {}; toast.message('已退出登录')
}

// --- Whitelist ---
const showWhitelist = ref(false)
const wlStatus = ref('idle') // idle | login | logged_in | scanning_admin | done | error
const wlFlow = ref('') // login | admin
const wlQRCode = ref('')
const wlIP = ref('')
const wlError = ref('')
const wlQRStatus = ref(0) // 1=waiting, 2=scanned, 7=expired
const wlMessage = ref('')
let wlEventSource: EventSource | null = null

async function onStartWhitelist(ip?: string) {
  if (!appID.value.trim()) { toast.error('请先填写 AppID'); return }
  const targetIP = ip || ''
  showWhitelist.value = true
  wlStatus.value = 'login'; wlFlow.value = ''; wlQRCode.value = ''; wlError.value = ''; wlQRStatus.value = 0
  try {
    const res = await fetch('/api/whitelist/start', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ appid: appID.value.trim(), ip: targetIP }) })
    const d = await res.json()
    if (!res.ok) throw new Error(d.error || '启动失败')
    connectWhitelistSSE()
  } catch (e) { wlStatus.value = 'error'; wlError.value = e instanceof Error ? e.message : '启动失败' }
}
function connectWhitelistSSE() {
  if (wlEventSource) { wlEventSource.close(); wlEventSource = null }
  wlEventSource = new EventSource('/api/whitelist/events')
  wlEventSource.addEventListener('state', (e) => {
    const d = JSON.parse((e as MessageEvent).data)
    wlStatus.value = d.status || 'idle'
    wlFlow.value = d.flow || ''
    wlQRCode.value = d.qrcode || ''
    wlIP.value = d.ip || ''
    wlError.value = d.error || ''
    wlQRStatus.value = d.qr_status || 0
    wlMessage.value = d.message || ''
    if (d.status === 'done') { toast.success('白名单配置完成'); wlEventSource?.close(); wlEventSource = null }
    if (d.status === 'error') { wlEventSource?.close(); wlEventSource = null }
    if (d.status === 'idle') { wlEventSource?.close(); wlEventSource = null }
  })
  wlEventSource.onerror = () => {}
}
async function onCancelWhitelist() {
  if (wlEventSource) { wlEventSource.close(); wlEventSource = null }
  try { await fetch('/api/whitelist/cancel', { method: 'POST' }) } catch {}
  showWhitelist.value = false; wlStatus.value = 'idle'; wlQRCode.value = ''; wlFlow.value = ''
}
function closeWhitelist() { if (wlStatus.value === 'done' || wlStatus.value === 'error' || wlStatus.value === 'idle') { showWhitelist.value = false } else { onCancelWhitelist() } }

// Auto-start login when dialog opens and not yet logged in
watch(() => props.open, (val) => {
  if (val && loginStatus.value !== 'ok' && loginStatus.value !== 'loading' && loginStatus.value !== 'waiting') {
    loginStarted = false
    startLogin()
  }
})

onMounted(() => { fetchCreds(); checkLoginStatus() })
onBeforeUnmount(() => { if (loginEventSource) loginEventSource.close(); if (wlEventSource) wlEventSource.close() })
</script>

<template>
  <div class="fixed inset-0 z-[99999] flex items-center justify-center bg-black/40">
    <div class="bg-background w-full max-w-md rounded-xl shadow-2xl mx-4 flex flex-col max-h-[85vh]" @click.stop>
      <!-- Header -->
      <div class="flex items-center gap-2.5 border-b px-4 py-3 shrink-0">
        <svg viewBox="0 0 24 24" class="size-5 shrink-0" :fill="'#07c160'"><path :d="wechatPath" /></svg>
        <span class="text-sm font-semibold flex-1 truncate">连接微信公众号</span>
        <Button size="icon-xs" variant="ghost" class="shrink-0" @click="emit('close')"><X class="size-3.5" /></Button>
      </div>

      <!-- Content -->
      <TooltipProvider>
      <div class="flex-1 overflow-auto px-4 py-4 space-y-4">
        <!-- Not logged in / loading / waiting -->
        <div v-if="loginStatus !== 'ok'" class="flex flex-col items-center py-4 space-y-4">
          <!-- WeChat logo -->
          <svg viewBox="0 0 24 24" class="size-16" :fill="'#07c160'" :opacity="0.2"><path :d="wechatPath" /></svg>

          <!-- Loading -->
          <div v-if="loginStatus === 'loading'" class="flex items-center gap-2">
            <Loader2 class="size-4 animate-spin text-muted-foreground" />
            <span class="text-[12px] text-muted-foreground">{{ loginMessage || '正在启动浏览器...' }}</span>
          </div>

          <!-- Waiting for scan -->
          <div v-if="loginStatus === 'waiting'" class="flex flex-col items-center space-y-3 w-full">
            <div class="flex items-center gap-1">
              <span class="text-[11px] text-muted-foreground text-center">请用微信扫描二维码登录</span>
              <Tooltip>
                <TooltipTrigger as-child><Info class="size-3 text-muted-foreground/60 cursor-help" /></TooltipTrigger>
                <TooltipContent side="top" class="max-w-52 text-[10px] z-[100001]"><p>登录后可发表草稿箱内容、搜索公众号文章等。</p></TooltipContent>
              </Tooltip>
            </div>
            <div v-if="loginQRCode" class="flex justify-center rounded-lg border bg-white p-3 relative">
              <img :src="loginQRCode" class="h-48 w-48" />
              <div v-if="loginQRStatus === 3" class="absolute inset-0 flex items-center justify-center bg-white/90 rounded-lg">
                <p class="text-[11px] text-muted-foreground">二维码已失效，正在刷新...</p>
              </div>
              <div v-if="loginQRStatus === 4" class="absolute inset-0 flex items-center justify-center bg-white/90 rounded-lg">
                <p class="text-[11px] text-muted-foreground">已扫码，请在手机上确认授权</p>
              </div>
            </div>
          </div>

          <!-- Idle (no login started) -->
          <div v-if="loginStatus === 'idle'" class="flex flex-col items-center space-y-2">
            <div class="flex items-center gap-1">
              <p class="text-[12px] text-muted-foreground">连接微信公众号</p>
              <Tooltip>
                <TooltipTrigger as-child><Info class="size-3 text-muted-foreground/60 cursor-help" /></TooltipTrigger>
                <TooltipContent side="top" class="max-w-52 text-[10px] z-[100001]"><p>登录后可发表草稿箱内容、搜索公众号文章等。</p></TooltipContent>
              </Tooltip>
            </div>
            <Button size="sm" variant="outline" @click="startLogin">
              <svg viewBox="0 0 24 24" class="size-4 mr-1.5" :fill="'#07c160'"><path :d="wechatPath" /></svg>
              扫码登录
            </Button>
          </div>

          <!-- Error -->
          <div v-if="loginStatus === 'error'" class="rounded-md bg-red-50 px-3 py-2 w-full">
            <p class="text-[11px] text-red-600">{{ loginError || '登录失败' }}</p>
            <Button size="xs" variant="outline" class="mt-2 h-6 text-[10px]" @click="loginStarted = false; loginStatus = 'idle'; startLogin()">重试</Button>
          </div>
        </div>

        <!-- Logged in -->
        <div v-if="loginStatus === 'ok'">
          <!-- Account card -->
          <div class="flex items-center gap-3 rounded-lg bg-accent/40 px-3 py-3">
            <img v-if="wechatAvatar" :src="wechatAvatar" class="size-12 rounded-full object-cover shrink-0" />
            <div v-else class="size-12 rounded-full bg-muted shrink-0 flex items-center justify-center">
              <svg viewBox="0 0 24 24" class="size-6" :fill="'#07c160'"><path :d="wechatPath" /></svg>
            </div>
            <div class="flex-1 min-w-0">
              <div class="text-sm font-semibold truncate">{{ wechatName || '已连接' }}</div>
              <div class="text-[10px] text-muted-foreground mt-0.5">公众号已连接</div>
            </div>
            <button class="text-[10px] text-muted-foreground hover:text-red-500 shrink-0" @click="logoutLogin">退出</button>
          </div>
        </div>

        <!-- Credentials (always visible, collapsible) -->
        <div class="space-y-2">
          <button class="flex items-center gap-1.5 text-[10px] font-semibold tracking-widest uppercase text-muted-foreground hover:text-foreground" @click="showCreds = !showCreds">
            <ChevronDown class="size-3 transition-transform" :class="showCreds ? 'rotate-0' : '-rotate-90'" />
            凭据配置
            <Tooltip>
              <TooltipTrigger as-child><Info class="size-3 text-muted-foreground/60 cursor-help" /></TooltipTrigger>
              <TooltipContent side="top" class="max-w-52 text-[10px] z-[100001]"><p>提供发布草稿、转存素材等能力。</p></TooltipContent>
            </Tooltip>
          </button>
          <div v-if="showCreds" class="space-y-2 pl-4.5">
            <div class="flex rounded-lg border p-0.5 gap-0.5">
              <button
                class="flex-1 h-6 rounded-md text-[10px] font-medium transition-colors"
                :class="callMode === 'local' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'"
                @click="callMode = 'local'"
              >本机直连</button>
              <button
                class="flex-1 h-6 rounded-md text-[10px] font-medium transition-colors"
                :class="callMode === 'proxy' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-muted'"
                @click="callMode = 'proxy'"
              >远程代理</button>
            </div>

            <template v-if="callMode === 'local'">
              <Input v-model="appID" placeholder="AppID" class="h-7 text-[11px]" />
              <input v-model="appSecret" placeholder="AppSecret" type="password" class="h-7 text-[11px] w-full rounded-md border bg-transparent px-2.5 outline-none focus:ring-1 focus:ring-ring" />
              <Button size="xs" variant="outline" class="w-full text-[11px] h-7 rounded-lg" @click="onSaveCreds">保存凭据</Button>
              <div class="rounded-md bg-accent/40 px-2.5 py-2 space-y-1.5">
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-1.5">
                    <ShieldCheck class="size-3 text-muted-foreground" />
                    <span class="text-[10px] font-medium">白名单检测</span>
                    <Tooltip>
                      <TooltipTrigger as-child><Info class="size-3 text-muted-foreground/60 cursor-help" /></TooltipTrigger>
                      <TooltipContent side="top" class="max-w-52 text-[10px] z-[100001]"><p>本机直连时，出口 IP 需在开放平台白名单内。</p></TooltipContent>
                    </Tooltip>
                  </div>
                  <Button size="xs" variant="outline" class="h-5 text-[10px] rounded" :disabled="checkingIP" @click="checkIP">
                    <Loader2 v-if="checkingIP" class="size-3 animate-spin" />
                    <span v-else>检测</span>
                  </Button>
                </div>
                <div v-if="checkedRealIP" class="rounded bg-yellow-50 border border-yellow-200 px-2 py-1.5 space-y-1.5">
                  <p class="text-[10px] text-yellow-700">当前 IP <span class="font-mono font-medium">{{ checkedRealIP }}</span> 未在白名单中</p>
                  <Button size="xs" variant="outline" class="h-5 text-[10px] rounded" @click="onStartWhitelist(checkedRealIP)">添加到白名单</Button>
                </div>
              </div>
            </template>

            <template v-else>
              <p class="text-[10px] text-muted-foreground leading-relaxed">开放平台调用经 VPS 固定出口，AppID/Secret 只存在代理侧。本机只需代理地址与 API Key。</p>
              <Input v-model="proxyBaseURL" placeholder="https://proxy.example.com" class="h-7 text-[11px]" />
              <input v-model="proxyAPIKey" placeholder="API Key" type="password" class="h-7 text-[11px] w-full rounded-md border bg-transparent px-2.5 outline-none focus:ring-1 focus:ring-ring" />
              <div class="flex gap-1.5">
                <Button size="xs" variant="outline" class="flex-1 text-[11px] h-7 rounded-lg" :disabled="testingProxy" @click="testProxy">
                  <Loader2 v-if="testingProxy" class="size-3 mr-1 animate-spin" />
                  测试连接
                </Button>
                <Button size="xs" variant="outline" class="flex-1 text-[11px] h-7 rounded-lg" @click="onSaveCreds">保存并启用</Button>
              </div>
            </template>
          </div>
        </div>
      </div>
      </TooltipProvider>

      <!-- Footer -->
      <div class="flex items-center justify-end border-t px-4 py-2.5 shrink-0">
        <div v-if="loginStatus === 'loading' || loginStatus === 'waiting'" class="flex-1">
          <Button variant="ghost" size="sm" class="text-[11px] h-7" @click="cancelLogin">取消登录</Button>
        </div>
        <Button variant="outline" size="sm" @click="emit('close')">关闭</Button>
      </div>
    </div>
  </div>

  <!-- Whitelist dialog -->
  <div v-if="showWhitelist" class="fixed inset-0 z-[100000] flex items-center justify-center bg-black/40">
    <div class="bg-background w-full max-w-sm rounded-lg shadow-xl mx-4" @click.stop>
      <div class="flex items-center justify-between border-b px-4 py-3">
        <span class="text-sm font-medium">IP 白名单配置</span>
        <Button variant="ghost" size="icon-xs" @click="closeWhitelist"><X class="size-3.5" /></Button>
      </div>
      <div class="px-4 py-4 space-y-3">
        <div v-if="wlIP" class="flex items-center justify-between rounded-md bg-accent/50 px-2.5 py-1.5">
          <span class="text-[10px] text-muted-foreground">将添加 IP</span>
          <span class="text-[11px] font-mono font-medium">{{ wlIP }}</span>
        </div>

        <!-- Login flow QR -->
        <div v-if="wlFlow === 'login' && wlQRCode" class="space-y-2">
          <p class="text-[11px] text-muted-foreground text-center">请用微信扫码登录开发者平台</p>
          <div class="flex justify-center rounded-lg border bg-white p-3 relative">
            <img :src="wlQRCode" class="h-48 w-48" />
            <div v-if="wlQRStatus === 7" class="absolute inset-0 flex items-center justify-center bg-white/90 rounded-lg">
              <p class="text-[11px] text-muted-foreground">二维码已过期，正在刷新...</p>
            </div>
            <div v-if="wlQRStatus === 2" class="absolute inset-0 flex items-center justify-center bg-white/90 rounded-lg">
              <p class="text-[11px] text-muted-foreground">已扫码，请在手机上确认</p>
            </div>
          </div>
        </div>

        <!-- Admin approval QR -->
        <div v-if="wlFlow === 'admin' && wlQRCode" class="space-y-2">
          <p class="text-[11px] text-muted-foreground text-center">请管理员扫码确认修改白名单</p>
          <div class="flex justify-center rounded-lg border bg-white p-3">
            <img :src="wlQRCode" class="h-48 w-48" />
          </div>
        </div>

        <!-- Processing states -->
        <div v-if="wlStatus === 'login' && !wlQRCode" class="flex items-center gap-2 py-4 justify-center">
          <Loader2 class="size-4 animate-spin text-muted-foreground" />
          <span class="text-[11px] text-muted-foreground">{{ wlMessage || '正在打开开发者平台...' }}</span>
        </div>
        <div v-if="wlStatus === 'logged_in'" class="flex items-center gap-2 py-4 justify-center">
          <Loader2 class="size-4 animate-spin text-muted-foreground" />
          <span class="text-[11px] text-muted-foreground">{{ wlMessage || '正在填写白名单...' }}</span>
        </div>

        <!-- Done -->
        <div v-if="wlStatus === 'done'" class="rounded-md bg-green-50 px-2.5 py-3 text-center">
          <p class="text-[11px] text-green-700 font-medium">白名单配置完成</p>
        </div>

        <!-- Error -->
        <div v-if="wlStatus === 'error'" class="rounded-md bg-red-50 px-2.5 py-3">
          <p class="text-[11px] text-red-600">{{ wlError || '配置失败' }}</p>
        </div>
      </div>
      <div class="flex justify-end gap-2 border-t px-4 py-3">
        <Button v-if="wlStatus !== 'done' && wlStatus !== 'error'" variant="outline" size="sm" @click="onCancelWhitelist">取消</Button>
        <Button v-else variant="outline" size="sm" @click="closeWhitelist">关闭</Button>
      </div>
    </div>
  </div>
</template>
