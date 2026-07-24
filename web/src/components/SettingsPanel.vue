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

const emit = defineEmits<{ styleChange: [v: unknown]; insert: [text: string] }>()
const styleOptions = ref<StyleOption[]>(STYLE_PRESETS.map((s) => ({ id: s.value, name: s.label, description: s.desc || '', primary: s.color, builtin: true })))
const loadingStyles = ref(false)
const themeFileInput = ref<HTMLInputElement | null>(null)
const current = computed(() => styleOptions.value.find((s) => s.id === style.value))
const canDelete = computed(() => !!current.value && !current.value.builtin)

async function loadStyleOptions() { loadingStyles.value = true; try { const list = await fetchStyles(); if (list.length) styleOptions.value = list } catch (e) { console.warn(e) } finally { loadingStyles.value = false } }
async function onReloadThemes() { try { const n = await reloadThemes(); await loadStyleOptions(); toast.success(`重载 ${n} 个主题`) } catch (e) { toast.error(e instanceof Error ? e.message : '重载失败') } }
function onStyleSelect(v: unknown) { emit('styleChange', v); const opt = styleOptions.value.find((s) => s.id === v); if (opt?.primary) primaryColor.value = opt.primary }
async function onImportFile(e: Event) { const input = e.target as HTMLInputElement; const file = input.files?.[0]; if (!file) return; try { const pack = JSON.parse(await file.text()); const saved = await importTheme(pack); await loadStyleOptions(); style.value = saved.id; emit('styleChange', saved.id); if (saved.primary) primaryColor.value = saved.primary; toast.success(`导入：${saved.name}`) } catch (err) { toast.error(err instanceof Error ? err.message : '导入失败') }; input.value = '' }
async function onDeleteTheme() { if (!canDelete.value || !current.value) return; const id = current.value.id; const name = current.value.name; if (!window.confirm(`删除「${name}」？`)) return; try { await deleteTheme(id); await loadStyleOptions(); if (style.value === id) { style.value = 'simple'; emit('styleChange', 'simple'); primaryColor.value = '#07c160' }; toast.success(`已删除：${name}`) } catch (err) { toast.error(err instanceof Error ? err.message : '删除失败') } }
onMounted(() => { loadStyleOptions() })
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

    <!-- 组件 -->
    <div class="space-y-2.5 px-3 py-3">
      <span class="text-[10px] font-semibold tracking-widest uppercase text-muted-foreground">组件</span>
      <div class="flex flex-wrap gap-1">
        <Button v-for="s in SNIPPETS" :key="s.label" variant="outline" size="xs" class="snippet-btn" @click="emit('insert', s.insert)">{{ s.label }}</Button>
      </div>
    </div>
  </div>
</template>
