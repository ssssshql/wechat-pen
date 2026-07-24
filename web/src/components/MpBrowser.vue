<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref } from 'vue'
import { toast } from 'vue-sonner'
import { X, Loader2, Rss } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

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

// --- Whitelist ---
const showWhitelist = ref(false)
const wlStatus = ref('idle') // idle | login | logged_in | scanning_admin | done | error
const wlFlow = ref('') // login | admin
const wlQRCode = ref('')
const wlIP = ref('')
const wlError = ref('')
let wlPollTimer: ReturnType<typeof setInterval> | null = null

async function onStartWhitelist() {
  if (!appID.value.trim()) { toast.error('请先填写 AppID'); return }
  showWhitelist.value = true
  wlStatus.value = 'login'; wlFlow.value = ''; wlQRCode.value = ''; wlError.value = ''
  try {
    const res = await fetch('/api/whitelist/start', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ appid: appID.value.trim() }) })
    const d = await res.json()
    if (!res.ok) throw new Error(d.error || '启动失败')
    startWhitelistPoll()
  } catch (e) { wlStatus.value = 'error'; wlError.value = e instanceof Error ? e.message : '启动失败' }
}
function startWhitelistPoll() {
  if (wlPollTimer) clearInterval(wlPollTimer)
  wlPollTimer = setInterval(async () => {
    try {
      const res = await fetch('/api/whitelist/status'); const d = await res.json()
      wlStatus.value = d.status || 'idle'; wlFlow.value = d.flow || ''; wlQRCode.value = d.qrcode || ''; wlIP.value = d.ip || ''; wlError.value = d.error || ''
      if (d.status === 'done' || d.status === 'error' || d.status === 'idle') { if (wlPollTimer) { clearInterval(wlPollTimer); wlPollTimer = null }; if (d.status === 'done') toast.success('白名单配置完成') }
    } catch {}
  }, 2000)
}
async function onCancelWhitelist() {
  if (wlPollTimer) { clearInterval(wlPollTimer); wlPollTimer = null }
  try { await fetch('/api/whitelist/cancel', { method: 'POST' }) } catch {}
  showWhitelist.value = false; wlStatus.value = 'idle'; wlQRCode.value = ''; wlFlow.value = ''
}
function closeWhitelist() { if (wlStatus.value === 'done' || wlStatus.value === 'error' || wlStatus.value === 'idle') { showWhitelist.value = false } else { onCancelWhitelist() } }

onMounted(() => { fetchCreds(); fetchOutboundIP(); checkLoginStatus() })
onBeforeUnmount(() => { if (loginTimer) clearInterval(loginTimer); if (wlPollTimer) clearInterval(wlPollTimer) })
</script>

<template>
  <div class="fixed inset-0 z-[99999] flex justify-end" @click.self="emit('close')">
    <div class="bg-background w-[min(100%,480px)] flex flex-col shadow-2xl border-l">
      <!-- Header -->
      <div class="flex items-center gap-2 border-b px-3 h-10 shrink-0">
        <Rss class="size-4 text-primary" />
        <span class="text-sm font-semibold flex-1 truncate">公众号配置</span>
        <Button size="icon-xs" variant="ghost" @click="emit('close')"><X class="size-3.5" /></Button>
      </div>

      <!-- Content -->
      <div class="flex-1 overflow-auto px-3 py-3 space-y-4">
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
            <div class="flex items-center gap-1.5">
              <button class="text-[11px] font-mono font-medium text-primary hover:underline" @click="copyIP">{{ outboundIP }}</button>
              <Button size="xs" variant="outline" class="h-5 text-[10px] rounded" @click="onStartWhitelist">配置白名单</Button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>

  <!-- Whitelist dialog -->
  <div v-if="showWhitelist" class="fixed inset-0 z-[100000] flex items-center justify-center bg-black/40" @click.self="closeWhitelist">
    <div class="bg-background w-full max-w-sm rounded-lg shadow-xl mx-4">
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
          <div class="flex justify-center rounded-lg border bg-white p-3">
            <img :src="wlQRCode" class="h-48 w-48" />
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
          <span class="text-[11px] text-muted-foreground">正在打开开发者平台...</span>
        </div>
        <div v-if="wlStatus === 'logged_in'" class="flex items-center gap-2 py-4 justify-center">
          <Loader2 class="size-4 animate-spin text-muted-foreground" />
          <span class="text-[11px] text-muted-foreground">正在填写白名单...</span>
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
