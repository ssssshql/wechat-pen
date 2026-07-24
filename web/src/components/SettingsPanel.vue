<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { toast } from 'vue-sonner'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Button } from '@/components/ui/button'
import { Slider } from '@/components/ui/slider'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { fetchStyles, reloadThemes, importTheme, deleteTheme, type StyleOption } from '@/lib/api'
import { HIGHLIGHT_THEMES, SNIPPETS, STYLE_PRESETS, type HighlightTheme, type StylePack } from '@/lib/types'

const style = defineModel<StylePack>('style', { required: true })
const primaryColor = defineModel<string>('primaryColor', { required: true })
const title = defineModel<string>('title', { required: true })
const textIndent = defineModel<boolean>('textIndent', { required: true })
const justify = defineModel<boolean>('justify', { required: true })
const paragraphGap = defineModel<string>('paragraphGap', { required: true })
const fontSizePx = defineModel<number[]>('fontSizePx', { required: true })
const lineHeight = defineModel<number[]>('lineHeight', { required: true })
const highlight = defineModel<boolean>('highlight', { required: true })
const highlightTheme = defineModel<HighlightTheme>('highlightTheme', { required: true })
const toc = defineModel<boolean>('toc', { required: true })
const footer = defineModel<boolean>('footer', { required: true })
const imageCaption = defineModel<boolean>('imageCaption', { required: true })
const appID = defineModel<string>('appID', { default: '' })
const appSecret = defineModel<string>('appSecret', { default: '' })

const emit = defineEmits<{ styleChange: [v: unknown]; insert: [text: string] }>()
const styleOptions = ref<StyleOption[]>(STYLE_PRESETS.map((s) => ({ id: s.value, name: s.label, description: s.desc || '', primary: s.color, builtin: true })))
const loadingStyles = ref(false)
const themeFileInput = ref<HTMLInputElement | null>(null)
const current = computed(() => styleOptions.value.find((s) => s.id === style.value))
const canDelete = computed(() => !!current.value && !current.value.builtin)
const outboundIP = ref('')
const loginStatus = ref<'idle' | 'loading' | 'waiting' | 'ok' | 'error'>('idle')
const loginQRCode = ref('')
const loginError = ref('')
let loginTimer: ReturnType<typeof setInterval> | null = null

async function loadStyleOptions() { loadingStyles.value = true; try { const list = await fetchStyles(); if (list.length) styleOptions.value = list } catch (e) { console.warn(e) } finally { loadingStyles.value = false } }
async function onReloadThemes() { try { const n = await reloadThemes(); await loadStyleOptions(); toast.success(`重载 ${n} 个主题`) } catch (e) { toast.error(e instanceof Error ? e.message : '重载失败') } }
function onStyleSelect(v: unknown) { emit('styleChange', v); const opt = styleOptions.value.find((s) => s.id === v); if (opt?.primary) primaryColor.value = opt.primary }
async function onImportFile(e: Event) { const input = e.target as HTMLInputElement; const file = input.files?.[0]; if (!file) return; try { const pack = JSON.parse(await file.text()); const saved = await importTheme(pack); await loadStyleOptions(); style.value = saved.id; emit('styleChange', saved.id); if (saved.primary) primaryColor.value = saved.primary; toast.success(`导入：${saved.name}`) } catch (err) { toast.error(err instanceof Error ? err.message : '导入失败') }; input.value = '' }
async function onDeleteTheme() { if (!canDelete.value || !current.value) return; const id = current.value.id; const name = current.value.name; if (!window.confirm(`删除「${name}」？`)) return; try { await deleteTheme(id); await loadStyleOptions(); if (style.value === id) { style.value = 'simple'; emit('styleChange', 'simple'); primaryColor.value = '#07c160' }; toast.success(`已删除：${name}`) } catch (err) { toast.error(err instanceof Error ? err.message : '删除失败') } }
onMounted(() => { loadStyleOptions(); fetchCreds(); fetchOutboundIP(); checkLoginStatus() })

async function checkLoginStatus() {
  try {
    const res = await fetch('/api/login/status')
    const data = await res.json()
    if (data.status === 'ok') loginStatus.value = 'ok'
  } catch {}
}

async function fetchCreds() {
  try { const res = await fetch('/api/credentials'); const data = await res.json(); if (data.appid) appID.value = data.appid; if (data.secret) appSecret.value = data.secret } catch {}
}
async function onSaveCreds() {
  try {
    const res = await fetch('/api/credentials', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ appid: appID.value, secret: appSecret.value }) })
    const data = await res.json()
    if (res.ok) toast.success('凭据已保存并设置到环境变量')
    else toast.error(data.error || '保存失败')
  } catch (e) { toast.error('保存失败') }
}

async function fetchOutboundIP() {
  try {
    const res = await fetch('/api/ip')
    const data = await res.json()
    if (data.ip) outboundIP.value = data.ip
  } catch {}
}

async function copyIP() {
  if (!outboundIP.value) return
  try {
    await navigator.clipboard.writeText(outboundIP.value)
    toast.success('已复制 IP: ' + outboundIP.value)
  } catch {
    toast.error('复制失败')
  }
}

async function startLogin() {
  loginStatus.value = 'loading'
  loginQRCode.value = ''
  loginError.value = ''
  try {
    const res = await fetch('/api/login/start', { method: 'POST' })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '失败')
    if (data.status === 'already_logged_in') {
      loginStatus.value = 'ok'
      toast.success('已有登录态')
      return
    }
    loginQRCode.value = data.qrcode_b64
    loginStatus.value = 'waiting'
    pollLoginStatus()
  } catch (e) {
    loginStatus.value = 'error'
    loginError.value = e instanceof Error ? e.message : '获取二维码失败'
  }
}

function pollLoginStatus() {
  if (loginTimer) clearInterval(loginTimer)
  loginTimer = setInterval(async () => {
    try {
      const res = await fetch('/api/login/status')
      const data = await res.json()
      if (data.status === 'ok') {
        loginStatus.value = 'ok'
        if (loginTimer) { clearInterval(loginTimer); loginTimer = null }
        toast.success('登录成功')
      } else if (data.status === 'error') {
        loginStatus.value = 'error'
        loginError.value = data.error || '登录失败'
        if (loginTimer) { clearInterval(loginTimer); loginTimer = null }
      }
    } catch {}
  }, 2000)
}

async function cancelLogin() {
  if (loginTimer) { clearInterval(loginTimer); loginTimer = null }
  loginStatus.value = 'idle'
  try { await fetch('/api/login/cancel') } catch {}
}

async function logoutLogin() {
  loginStatus.value = 'idle'
  loginQRCode.value = ''
  if (loginTimer) { clearInterval(loginTimer); loginTimer = null }
  try { await fetch('/api/login/logout') } catch {}
  toast.message('已退出登录')
}
</script>

<template>
  <div class="divide-y divide-black/[0.04]">
    <!-- 输出配置 -->
    <div class="space-y-2.5 px-3 py-3">
      <div class="flex items-center justify-between">
        <span class="text-[10px] font-semibold tracking-widest uppercase text-muted-foreground">输出</span>
      </div>
      <div class="space-y-2">
        <div>
          <div class="mb-1 flex items-center justify-between">
            <span class="text-[11px] font-medium text-muted-foreground">样式</span>
            <div class="flex items-center gap-0.5">
              <Button size="xs" variant="ghost" class="h-5 rounded-md text-[10px]" :disabled="loadingStyles" @click="onReloadThemes">重载</Button>
              <Button size="xs" variant="ghost" class="h-5 rounded-md text-[10px]" @click="themeFileInput?.click()">导入</Button>
              <Button size="xs" variant="ghost" class="h-5 rounded-md text-[10px]" :disabled="!canDelete" @click="onDeleteTheme">删除</Button>
            </div>
          </div>
          <input ref="themeFileInput" type="file" accept="application/json,.json" class="hidden" @change="onImportFile" />
          <Select :model-value="style" @update:model-value="onStyleSelect">
            <SelectTrigger class="settings-select-trigger w-full"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem v-for="s in styleOptions" :key="s.id" :value="s.id">
                <span>{{ s.name }}</span>
                <span v-if="!s.builtin" class="text-muted-foreground ml-1 text-[10px]">外部</span>
              </SelectItem>
            </SelectContent>
          </Select>
          <p v-if="current?.description" class="text-muted-foreground mt-0.5 pl-0.5 text-[10px] leading-tight">{{ current.description }}</p>
        </div>

        <div class="flex items-center gap-2">
          <input v-model="primaryColor" type="color" class="color-picker" />
          <Input v-model="primaryColor" class="settings-input" />
        </div>

        <Input v-model="title" placeholder="文章标题" class="settings-input" />
      </div>
    </div>

    <!-- 排版 -->
    <div class="space-y-2.5 px-3 py-3">
      <span class="text-[10px] font-semibold tracking-widest uppercase text-muted-foreground">排版</span>
      <div class="space-y-2">
        <div class="flex items-center justify-between">
          <Label for="indent" class="text-xs cursor-pointer">首行缩进</Label>
          <Switch id="indent" v-model:checked="textIndent" size="sm" />
        </div>
        <div class="flex items-center justify-between">
          <Label for="justify" class="text-xs cursor-pointer">两端对齐</Label>
          <Switch id="justify" v-model:checked="justify" size="sm" />
        </div>
        <Select v-model="paragraphGap">
          <SelectTrigger class="settings-select-trigger w-full"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="0.6em">段间距 紧凑</SelectItem>
            <SelectItem value="1em">段间距 标准</SelectItem>
            <SelectItem value="1.4em">段间距 宽松</SelectItem>
          </SelectContent>
        </Select>
        <div>
          <div class="mb-1 flex items-center justify-between">
            <span class="text-[10px] text-muted-foreground">字号</span>
            <span class="text-muted-foreground font-mono text-[10px]">{{ fontSizePx[0] }}px</span>
          </div>
          <Slider v-model="fontSizePx" :min="14" :max="20" :step="1" class="[&_[data-slot=slider-track]]:h-1 [&_[data-slot=slider-thumb]]:size-3" />
        </div>
        <div>
          <div class="mb-1 flex items-center justify-between">
            <span class="text-[10px] text-muted-foreground">行高</span>
            <span class="text-muted-foreground font-mono text-[10px]">{{ lineHeight[0].toFixed(2) }}</span>
          </div>
          <Slider v-model="lineHeight" :min="1.4" :max="2.2" :step="0.05" class="[&_[data-slot=slider-track]]:h-1 [&_[data-slot=slider-thumb]]:size-3" />
        </div>
      </div>
    </div>

    <!-- 增强 -->
    <div class="space-y-2.5 px-3 py-3">
      <span class="text-[10px] font-semibold tracking-widest uppercase text-muted-foreground">增强</span>
      <div class="space-y-2">
        <div class="flex items-center justify-between">
          <Label for="hl" class="text-xs cursor-pointer">输出代码高亮</Label>
          <Switch id="hl" v-model:checked="highlight" size="sm" />
        </div>
        <Select v-model="highlightTheme" :disabled="!highlight">
          <SelectTrigger class="settings-select-trigger w-full"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem v-for="h in HIGHLIGHT_THEMES" :key="h.value" :value="h.value">{{ h.label }}</SelectItem>
          </SelectContent>
        </Select>
        <div class="flex items-center justify-between">
          <Label for="toc" class="text-xs cursor-pointer">自动目录</Label>
          <Switch id="toc" v-model:checked="toc" size="sm" />
        </div>
        <div class="flex items-center justify-between">
          <Label for="footer" class="text-xs cursor-pointer">文末引导</Label>
          <Switch id="footer" v-model:checked="footer" size="sm" />
        </div>
        <div class="flex items-center justify-between">
          <Label for="cap" class="text-xs cursor-pointer">图片图注</Label>
          <Switch id="cap" v-model:checked="imageCaption" size="sm" />
        </div>
      </div>
    </div>

    <!-- 公众号凭据 -->
    <div class="space-y-2 px-3 py-3">
      <span class="text-[10px] font-semibold tracking-widest uppercase text-muted-foreground">公众号凭据</span>
      <p class="text-[10px] text-muted-foreground leading-relaxed">设置后保存到环境中。也可通过环境变量 <code class="text-primary bg-accent px-1 rounded text-[9px]">WECHAT_PEN_APPID</code> / <code class="text-primary bg-accent px-1 rounded text-[9px]">WECHAT_PEN_SECRET</code> 或 CLI <code class="text-primary bg-accent px-1 rounded text-[9px]">--appid</code> / <code class="text-primary bg-accent px-1 rounded text-[9px]">--secret</code> 配置</p>
      <Input v-model="appID" placeholder="AppID" class="settings-input" />
      <div class="settings-input">
        <input v-model="appSecret" placeholder="AppSecret" type="password" />
      </div>
      <Button size="xs" variant="outline" class="w-full text-[11px] h-7 rounded-lg" @click="onSaveCreds">保存凭据</Button>
      <div v-if="outboundIP" class="flex items-center justify-between rounded-md bg-accent/50 px-2.5 py-1.5 mt-2">
        <span class="text-[10px] text-muted-foreground">出口 IP</span>
        <button class="text-[11px] font-mono font-medium text-primary hover:underline" title="点击复制" @click="copyIP">{{ outboundIP }}</button>
      </div>
      <div v-else class="text-[10px] text-muted-foreground/60 mt-1">获取出口 IP 中...</div>
    </div>

    <!-- 公众号登录 -->
    <div class="space-y-2 px-3 py-3">
      <span class="text-[10px] font-semibold tracking-widest uppercase text-muted-foreground">扫码登录</span>
      <p class="text-[10px] text-muted-foreground leading-relaxed">扫码后可获取后台 Cookie，用于直接发布文章（无需去后台手动点发布）。</p>

      <!-- Not logged in: show QR code -->
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

      <!-- Logged in -->
      <div v-else-if="loginStatus === 'ok'" class="rounded-md bg-green-50 px-2.5 py-2 flex items-center justify-between">
        <span class="text-[11px] text-green-700 font-medium">已登录</span>
        <button class="text-[10px] text-muted-foreground hover:text-red-500" @click="logoutLogin">退出</button>
      </div>

      <!-- Error -->
      <div v-else-if="loginStatus === 'error'" class="rounded-md bg-red-50 px-2.5 py-2">
        <p class="text-[11px] text-red-600">{{ loginError || '登录失败' }}</p>
        <button class="text-[10px] text-primary mt-1" @click="loginStatus = 'idle'; loginError = ''">重试</button>
      </div>
    </div>

    <!-- 组件 -->
    <div class="space-y-2.5 px-3 py-3">
      <span class="text-[10px] font-semibold tracking-widest uppercase text-muted-foreground">组件</span>
      <div class="flex flex-wrap gap-1">
        <Button v-for="s in SNIPPETS" :key="s.label" variant="outline" size="xs" class="snippet-btn" @click="emit('insert', s.insert)">{{ s.label }}</Button>
      </div>
    </div>
  </div>
</template>
