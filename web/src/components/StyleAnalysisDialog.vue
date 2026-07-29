<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Loader2, Search, Sparkles, Trash2, X } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { toast } from 'vue-sonner'
import {
  getAIConfig,
  saveAIConfig,
  analyzeStyle,
  fetchWritingStyles,
  deleteWritingStyle,
  searchBiz,
  type WritingStyle,
  type BizItem,
} from '@/lib/api'

const open = defineModel<boolean>('open', { required: true })

// --- AI Config ---
const aiBaseUrl = ref('')
const aiApiKey = ref('')
const aiModel = ref('')
const hasAiKey = ref(false)

async function loadConfig() {
  try {
    const cfg = await getAIConfig()
    aiBaseUrl.value = cfg.ai_base_url || ''
    aiModel.value = cfg.ai_model || ''
    hasAiKey.value = cfg.has_ai_key
  } catch { /* ignore */ }
}

async function saveConfig() {
  try {
    await saveAIConfig({
      ai_base_url: aiBaseUrl.value || undefined,
      ai_api_key: aiApiKey.value || undefined,
      ai_model: aiModel.value || undefined,
    })
    hasAiKey.value = true
    if (aiApiKey.value) aiApiKey.value = ''
    toast.success('AI 配置已保存')
  } catch (e: any) {
    toast.error('保存失败: ' + (e.message || e))
  }
}

// --- Search & Analyze ---
const bizQuery = ref('')
const bizResults = ref<BizItem[]>([])
const bizSearching = ref(false)
const selectedBiz = ref<BizItem | null>(null)
const analyzing = ref(false)
const savedStyles = ref<WritingStyle[]>([])

async function loadStyles() {
  try {
    savedStyles.value = await fetchWritingStyles()
  } catch { /* ignore */ }
}

async function doSearchBiz() {
  if (!bizQuery.value.trim()) return
  bizSearching.value = true
  try {
    const result = await searchBiz(bizQuery.value.trim())
    bizResults.value = result.list
    if (result.list.length === 0) toast.info('未找到匹配公众号')
  } catch (e: any) {
    toast.error('搜索失败: ' + (e.message || e))
  } finally {
    bizSearching.value = false
  }
}

function selectBiz(biz: BizItem) {
  selectedBiz.value = biz
}

async function doAnalyze() {
  if (!selectedBiz.value) return
  analyzing.value = true
  try {
    const style = await analyzeStyle(selectedBiz.value.fakeid, selectedBiz.value.nickname)
    toast.success('写作风格分析完成: ' + style.name)
    selectedBiz.value = null
    bizResults.value = []
    bizQuery.value = ''
    await loadStyles()
  } catch (e: any) {
    toast.error('分析失败: ' + (e.message || e))
  } finally {
    analyzing.value = false
  }
}

async function doDeleteStyle(id: string) {
  try {
    await deleteWritingStyle(id)
    savedStyles.value = savedStyles.value.filter((s) => s.id !== id)
    toast.success('已删除')
  } catch (e: any) {
    toast.error('删除失败: ' + (e.message || e))
  }
}

onMounted(() => {
  loadConfig()
  loadStyles()
})
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-[99999] flex items-center justify-center p-4 bg-black/45 backdrop-blur-[2px]"
    >
      <div class="relative w-full max-w-[480px] max-h-[85vh] flex flex-col rounded-xl border bg-background shadow-2xl overflow-hidden">
        <!-- Header -->
        <header class="flex items-center justify-between px-4 py-3 border-b shrink-0">
          <div class="flex items-center gap-2">
            <Sparkles class="size-4 text-primary" />
            <span class="font-semibold text-sm">分析写作风格</span>
          </div>
          <Button variant="ghost" size="icon-xs" @click="open = false">
            <X class="size-4" />
          </Button>
        </header>

        <!-- Body -->
        <div class="flex-1 overflow-y-auto p-4 space-y-4">
          <!-- AI Config -->
          <div class="p-3 border rounded-lg space-y-2 bg-muted/30">
            <p class="text-xs font-medium text-muted-foreground">AI 配置</p>
            <div class="space-y-2">
              <div>
                <Label class="text-[11px]">Base URL</Label>
                <Input
                  v-model="aiBaseUrl"
                  class="h-7 text-xs"
                  placeholder="默认 https://api.openai.com/v1"
                />
              </div>
              <div>
                <Label class="text-[11px]">Model</Label>
                <Input
                  v-model="aiModel"
                  class="h-7 text-xs"
                  placeholder="gpt-4o-mini"
                />
              </div>
              <div>
                <Label class="text-[11px]">API Key</Label>
                <Input
                  v-model="aiApiKey"
                  type="password"
                  class="h-7 text-xs"
                  :placeholder="hasAiKey ? '已保存，输入覆盖' : '输入 API Key'"
                />
              </div>
              <Button variant="outline" size="sm" class="w-full text-xs h-7" @click="saveConfig">
                保存配置
              </Button>
            </div>
          </div>

          <Separator />

          <!-- Search -->
          <div class="space-y-2">
            <p class="text-xs font-medium text-muted-foreground">搜索公众号并分析其文章风格</p>
            <div class="flex gap-2">
              <Input
                v-model="bizQuery"
                class="h-7 text-xs flex-1"
                placeholder="输入公众号名称"
                @keydown.enter="doSearchBiz"
              />
              <Button variant="outline" size="icon-xs" class="h-7 w-7 shrink-0" :disabled="bizSearching" @click="doSearchBiz">
                <Loader2 v-if="bizSearching" class="size-3.5 animate-spin" />
                <Search v-else class="size-3.5" />
              </Button>
            </div>

            <!-- Search results -->
            <div v-if="bizResults.length" class="space-y-1 max-h-48 overflow-y-auto">
              <div
                v-for="biz in bizResults"
                :key="biz.fakeid"
                class="flex items-center gap-2 p-2 rounded cursor-pointer hover:bg-muted transition-colors"
                :class="{ 'bg-muted ring-1 ring-primary': selectedBiz?.fakeid === biz.fakeid }"
                @click="selectBiz(biz)"
              >
                <img
                  v-if="biz.round_head_img"
                  :src="`/api/biz/image/proxy?url=${encodeURIComponent(biz.round_head_img)}`"
                  class="size-8 rounded-full shrink-0 object-cover"
                  @error="($event.target as HTMLImageElement).style.display = 'none'"
                />
                <div class="min-w-0">
                  <p class="text-xs font-medium truncate">{{ biz.nickname }}</p>
                  <p class="text-[10px] text-muted-foreground truncate">{{ biz.signature || biz.alias }}</p>
                </div>
              </div>
            </div>

            <Button
              v-if="selectedBiz"
              variant="default"
              size="sm"
              class="w-full text-xs h-8"
              :disabled="analyzing"
              @click="doAnalyze"
            >
              <Loader2 v-if="analyzing" class="size-3.5 mr-1 animate-spin" />
              <Sparkles v-else class="size-3.5 mr-1" />
              {{ analyzing ? '分析中...' : `分析「${selectedBiz.nickname}」写作风格` }}
            </Button>
          </div>

          <Separator />

          <!-- Saved styles -->
          <div class="space-y-2">
            <p class="text-xs font-medium text-muted-foreground">已保存风格 ({{ savedStyles.length }})</p>
            <div v-if="savedStyles.length === 0" class="text-xs text-muted-foreground py-4 text-center">
              暂无保存的写作风格
            </div>
            <div v-for="style in savedStyles" :key="style.id" class="p-2.5 border rounded-lg space-y-1">
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-1.5">
                  <span class="text-xs font-medium">{{ style.nickname }}</span>
                  <Badge variant="secondary" class="text-[10px] px-1 h-4">
                    {{ style.sampleCount }} 篇
                  </Badge>
                </div>
                <Button
                  variant="ghost"
                  size="icon-xs"
                  class="size-5 text-muted-foreground hover:text-destructive"
                  @click="doDeleteStyle(style.id)"
                >
                  <Trash2 class="size-3" />
                </Button>
              </div>
              <div class="text-[11px] text-muted-foreground max-h-24 overflow-y-auto whitespace-pre-wrap leading-relaxed">{{ style.stylePrompt }}</div>
              <p class="text-[10px] text-muted-foreground">
                {{ new Date(style.createdAt).toLocaleDateString('zh-CN') }}
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>
