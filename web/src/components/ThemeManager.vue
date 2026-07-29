<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import {
  Download,
  Loader2,
  Palette,
  RefreshCw,
  Trash2,
  Upload,
  X,
  Check,
} from '@lucide/vue'
import { Button } from '@/components/ui/button'
import {
  convertMarkdown,
  fetchStyles,
  reloadThemes,
  importTheme,
  deleteTheme,
  getTheme,
  downloadText,
  type StyleOption,
} from '@/lib/api'

const props = defineProps<{ currentStyle: string }>()
const emit = defineEmits<{
  close: []
  apply: [opt: StyleOption]
  changed: []
}>()

/** Short sample used only for card thumbnails — keep tiny for speed. */
const PREVIEW_MD = `# 一级标题
## 二级标题
正文段落示意，用于观察字号、行高与主色。
> 引用块样式预览
- 列表项一 · 列表项二
行内 \`code\` 与 **加粗** 示意。`

const items = ref<StyleOption[]>([])
const loading = ref(false)
const importing = ref(false)
const deleting = ref<Set<string>>(new Set())
const fileInput = ref<HTMLInputElement | null>(null)
const filter = ref<'all' | 'builtin' | 'external'>('all')

/** styleId → preview HTML fragment (section body) */
const previews = ref<Record<string, string>>({})
const previewLoading = ref<Set<string>>(new Set())
const previewErrors = ref<Set<string>>(new Set())

const filtered = computed(() => {
  if (filter.value === 'builtin') return items.value.filter((i) => i.builtin)
  if (filter.value === 'external') return items.value.filter((i) => !i.builtin)
  return items.value
})

const externalCount = computed(() => items.value.filter((i) => !i.builtin).length)

async function loadList() {
  loading.value = true
  try {
    const list = await fetchStyles()
    if (list.length) items.value = list
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '加载主题失败')
  } finally {
    loading.value = false
  }
}

async function loadPreview(opt: StyleOption) {
  if (previews.value[opt.id] || previewLoading.value.has(opt.id)) return
  previewLoading.value = new Set(previewLoading.value).add(opt.id)
  previewErrors.value.delete(opt.id)
  try {
    const res = await convertMarkdown({
      markdown: PREVIEW_MD,
      theme: 'wechat',
      title: opt.name,
      style: opt.id,
      primaryColor: opt.primary || '#07c160',
      textIndent: false,
      justify: true,
      highlightTheme: 'github',
      toc: false,
      footer: false,
      previewWidth: 'phone',
      previewShell: 'light',
    })
    // Prefer raw article html for denser card; fall back to full preview frame.
    previews.value = { ...previews.value, [opt.id]: res.html || res.preview || '' }
  } catch {
    previewErrors.value = new Set(previewErrors.value).add(opt.id)
  } finally {
    const next = new Set(previewLoading.value)
    next.delete(opt.id)
    previewLoading.value = next
  }
}

/** Load visible previews with a small concurrency pool. */
async function loadVisiblePreviews(list: StyleOption[]) {
  const queue = list.filter((o) => !previews.value[o.id] && !previewLoading.value.has(o.id))
  const concurrency = 3
  let i = 0
  async function worker() {
    while (i < queue.length) {
      const opt = queue[i++]
      if (opt) await loadPreview(opt)
    }
  }
  await Promise.all(Array.from({ length: Math.min(concurrency, queue.length) }, () => worker()))
}

async function onReload() {
  try {
    const n = await reloadThemes()
    previews.value = {}
    await loadList()
    emit('changed')
    toast.success(`已重载，外部主题 ${n} 个`)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '重载失败')
  }
}

function apply(opt: StyleOption) {
  emit('apply', opt)
  toast.success(`已应用：${opt.name}`)
}

async function onImportFile(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  importing.value = true
  try {
    const pack = JSON.parse(await file.text())
    const saved = await importTheme(pack)
    delete previews.value[saved.id]
    await loadList()
    emit('changed')
    emit('apply', saved)
    toast.success(`已导入并应用：${saved.name}`)
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '导入失败')
  } finally {
    importing.value = false
    input.value = ''
  }
}

async function onExport(opt: StyleOption) {
  try {
    const pack = await getTheme(opt.id)
    downloadText(
      `${pack.id || opt.id}.json`,
      JSON.stringify(pack, null, 2),
      'application/json;charset=utf-8',
    )
    toast.success(`已导出：${opt.name}`)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '导出失败')
  }
}

async function onDelete(opt: StyleOption) {
  if (opt.builtin) return
  if (!window.confirm(`删除外部主题「${opt.name}」？此操作不可撤销。`)) return
  deleting.value.add(opt.id)
  try {
    await deleteTheme(opt.id)
    items.value = items.value.filter((i) => i.id !== opt.id)
    const { [opt.id]: _, ...rest } = previews.value
    previews.value = rest
    emit('changed')
    if (props.currentStyle === opt.id) {
      const fallback = items.value.find((i) => i.id === 'simple') || items.value[0]
      if (fallback) emit('apply', fallback)
    }
    toast.success(`已删除：${opt.name}`)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '删除失败')
  } finally {
    deleting.value.delete(opt.id)
  }
}

watch(filtered, (list) => {
  void loadVisiblePreviews(list)
})

onMounted(async () => {
  await loadList()
  void loadVisiblePreviews(filtered.value)
})
</script>

<template>
  <div
    class="bg-background flex max-h-[min(88vh,820px)] w-full max-w-4xl flex-col overflow-hidden rounded-xl border shadow-2xl"
    @click.stop
  >
    <!-- Header -->
    <div class="flex shrink-0 items-center gap-2 border-b px-4 py-3">
      <Palette class="size-4 text-muted-foreground" />
      <div class="min-w-0 flex-1">
        <div class="text-sm font-semibold tracking-tight">主题管理</div>
        <div class="text-[11px] text-muted-foreground">选择样式包 · 预览真实转换效果 · 导入导出 JSON</div>
      </div>
      <Button variant="ghost" size="icon-xs" @click="emit('close')">
        <X class="size-3.5" />
      </Button>
    </div>

    <!-- Toolbar -->
    <div class="flex shrink-0 flex-wrap items-center gap-1.5 border-b px-4 py-2">
      <button
        v-for="f in ([
          ['all', '全部'],
          ['builtin', '内置'],
          ['external', '外部'],
        ] as const)"
        :key="f[0]"
        type="button"
        class="rounded-md px-2.5 py-1 text-[11px] transition-colors"
        :class="
          filter === f[0]
            ? 'bg-foreground text-background'
            : 'text-muted-foreground hover:bg-muted hover:text-foreground'
        "
        @click="filter = f[0]"
      >
        {{ f[1] }}
        <span v-if="f[0] === 'external'" class="ml-0.5 opacity-70">{{ externalCount }}</span>
      </button>

      <div class="ml-auto flex items-center gap-1">
        <Button
          variant="ghost"
          size="icon-xs"
          :disabled="loading"
          title="重载 themes/"
          @click="onReload"
        >
          <RefreshCw class="size-3" :class="loading ? 'animate-spin' : ''" />
        </Button>
        <input
          ref="fileInput"
          type="file"
          accept="application/json,.json"
          class="hidden"
          @change="onImportFile"
        />
        <Button
          variant="secondary"
          size="xs"
          class="h-7 gap-1 text-[11px]"
          :disabled="importing"
          @click="fileInput?.click()"
        >
          <Loader2 v-if="importing" class="size-3 animate-spin" />
          <Upload v-else class="size-3" />
          导入
        </Button>
      </div>
    </div>

    <!-- Body -->
    <div class="min-h-0 flex-1 overflow-auto p-4">
      <div v-if="loading && !items.length" class="flex items-center justify-center py-16">
        <Loader2 class="size-5 animate-spin text-muted-foreground" />
      </div>

      <div v-else-if="!filtered.length" class="flex items-center justify-center py-16">
        <p class="text-xs text-muted-foreground">
          {{ filter === 'external' ? '暂无外部主题，点击导入添加' : '暂无主题' }}
        </p>
      </div>

      <div v-else class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <div
          v-for="opt in filtered"
          :key="opt.id"
          class="group flex flex-col overflow-hidden rounded-xl border transition-all"
          :class="
            currentStyle === opt.id
              ? 'border-primary/50 ring-2 ring-primary/20 shadow-sm'
              : 'border-border/70 hover:border-border hover:shadow-sm'
          "
        >
          <!-- Live style preview -->
          <div
            class="relative h-[148px] overflow-hidden border-b bg-[#f3f4f6]"
            :style="{ backgroundImage: `linear-gradient(180deg, ${opt.primary || '#07c160'}14, transparent 48%)` }"
          >
            <div
              v-if="previewLoading.has(opt.id) && !previews[opt.id]"
              class="absolute inset-0 flex items-center justify-center"
            >
              <Loader2 class="size-4 animate-spin text-muted-foreground" />
            </div>
            <div
              v-else-if="previewErrors.has(opt.id) && !previews[opt.id]"
              class="absolute inset-0 flex items-center justify-center px-3 text-center text-[10px] text-muted-foreground"
            >
              预览失败
            </div>
            <div
              v-else-if="previews[opt.id]"
              class="theme-preview-scale pointer-events-none origin-top-left p-2"
              v-html="previews[opt.id]"
            />
            <div
              v-else
              class="absolute inset-0 flex flex-col justify-center gap-1.5 px-4"
            >
              <div class="h-2.5 w-2/3 rounded" :style="{ background: opt.primary || '#07c160' }" />
              <div class="h-2 w-full rounded bg-black/10" />
              <div class="h-2 w-5/6 rounded bg-black/8" />
              <div class="mt-1 h-8 w-full rounded-md bg-black/5" />
            </div>

            <div
              v-if="currentStyle === opt.id"
              class="absolute top-2 right-2 flex items-center gap-0.5 rounded-full bg-primary px-1.5 py-0.5 text-[9px] font-medium text-primary-foreground shadow"
            >
              <Check class="size-2.5" />使用中
            </div>
          </div>

          <!-- Meta -->
          <div class="flex flex-1 flex-col gap-1.5 p-3">
            <div class="flex items-start gap-2">
              <div
                class="mt-0.5 size-3.5 shrink-0 rounded-full border border-black/10 shadow-sm"
                :style="{ background: opt.primary || '#07c160' }"
              />
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-1.5">
                  <span class="truncate text-[12px] font-semibold leading-tight">{{ opt.name }}</span>
                  <span
                    v-if="!opt.builtin"
                    class="shrink-0 rounded bg-muted px-1 py-px text-[9px] text-muted-foreground"
                  >外部</span>
                </div>
                <p
                  v-if="opt.description"
                  class="mt-0.5 line-clamp-2 text-[10px] leading-snug text-muted-foreground"
                >
                  {{ opt.description }}
                </p>
              </div>
            </div>

            <div class="mt-auto flex items-center gap-1 pt-1">
              <Button
                size="xs"
                :variant="currentStyle === opt.id ? 'secondary' : 'default'"
                class="h-7 flex-1 text-[11px]"
                :disabled="currentStyle === opt.id"
                @click="apply(opt)"
              >
                {{ currentStyle === opt.id ? '使用中' : '应用' }}
              </Button>
              <Button
                size="xs"
                variant="ghost"
                class="h-7 px-2 text-[10px]"
                title="导出 JSON"
                @click="onExport(opt)"
              >
                <Download class="size-3" />
              </Button>
              <Button
                v-if="!opt.builtin"
                size="xs"
                variant="ghost"
                class="h-7 px-2 text-[10px] text-destructive hover:text-destructive"
                :disabled="deleting.has(opt.id)"
                @click="onDelete(opt)"
              >
                <Loader2 v-if="deleting.has(opt.id)" class="size-3 animate-spin" />
                <Trash2 v-else class="size-3" />
              </Button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="shrink-0 border-t px-4 py-2.5 text-[10px] leading-relaxed text-muted-foreground">
      预览由 <code class="rounded bg-muted px-1">/api/convert</code> 真实渲染。
      外部主题存于 <code class="rounded bg-muted px-1">themes/*.json</code>。
      导入 JSON 主题包后立即生效；内置主题可导出为模板再改。
    </div>
  </div>
</template>

<style scoped>
/* Shrink full article HTML into a dense card thumbnail */
.theme-preview-scale {
  width: 200%;
  transform: scale(0.5);
  font-size: 14px;
  line-height: 1.5;
}
.theme-preview-scale :deep(img) {
  max-height: 48px !important;
  object-fit: cover;
}
.theme-preview-scale :deep(pre) {
  max-height: 56px;
  overflow: hidden !important;
  font-size: 11px !important;
}
.theme-preview-scale :deep(table) {
  font-size: 10px !important;
}
</style>
