<script setup lang="ts">
import {
  Bold,
  Code2,
  Heading2,
  Image as ImageIcon,
  Italic,
  Link2,
  List,
  ListIndentIncrease,
  ListOrdered,
  Puzzle,
  Quote,
  Sparkles,
  TextAlignJustify,
  TextAlignStart,
} from '@lucide/vue'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Separator } from '@/components/ui/separator'
import { fetchStyles, type StyleOption } from '@/lib/api'
import {
  HIGHLIGHT_THEMES,
  SNIPPETS,
  STYLE_PRESETS,
  type HighlightTheme,
  type StylePack,
} from '@/lib/types'
import { onMounted, ref, watch } from 'vue'

const style = defineModel<StylePack>('style', { required: true })
const highlightTheme = defineModel<HighlightTheme>('highlightTheme', { required: true })
const textIndent = defineModel<boolean>('textIndent', { required: true })
const justify = defineModel<boolean>('justify', { required: true })

const emit = defineEmits(['action', 'insert', 'styleChange', 'primaryColor', 'toggleAi'])

const styleOptions = ref<StyleOption[]>(
  STYLE_PRESETS.map((s) => ({
    id: s.value,
    name: s.label,
    description: s.desc || '',
    primary: s.color,
    builtin: true,
  })),
)

async function loadStyleOptions() {
  try {
    const list = await fetchStyles()
    if (list.length) styleOptions.value = list
  } catch (e) {
    console.warn(e)
  }
}

function onStyleSelect(v: unknown) {
  emit('styleChange', v)
  const opt = styleOptions.value.find((s) => s.id === v)
  if (opt?.primary) emit('primaryColor', opt.primary)
}

onMounted(() => {
  loadStyleOptions()
})

const props = defineProps<{ refreshKey?: number; aiActive?: boolean }>()
watch(() => props.refreshKey, () => { loadStyleOptions() })

defineExpose({ reloadStyles: loadStyleOptions })

const formatActions: { id: string; label: string; icon: unknown }[] = [
  { id: 'h2', label: '二级标题', icon: Heading2 },
  { id: 'bold', label: '加粗 Ctrl+B', icon: Bold },
  { id: 'italic', label: '斜体 Ctrl+I', icon: Italic },
  { id: 'quote', label: '引用', icon: Quote },
  { id: 'ul', label: '无序列表', icon: List },
  { id: 'ol', label: '有序列表', icon: ListOrdered },
  { id: 'code', label: '代码块', icon: Code2 },
  { id: 'link', label: '链接 Ctrl+K', icon: Link2 },
  { id: 'image', label: '图片', icon: ImageIcon },
]
</script>

<template>
  <TooltipProvider :delay-duration="200">
    <div class="toolbar-row flex flex-wrap items-center gap-1 border-b px-2 py-1">
      <!-- Format buttons -->
      <Tooltip v-for="a in formatActions" :key="a.id">
        <TooltipTrigger as-child @click="emit('action', a.id)">
          <Button
            variant="ghost"
            size="icon-xs"
            class="text-muted-foreground hover:text-foreground"
          >
            <component :is="a.icon" class="size-3.5" />
          </Button>
        </TooltipTrigger>
        <TooltipContent side="bottom">{{ a.label }}</TooltipContent>
      </Tooltip>

      <Separator orientation="vertical" class="mx-1 h-4" />

      <!-- Style selector -->
      <Select :model-value="style" @update:model-value="onStyleSelect">
        <SelectTrigger class="h-6 w-[80px] text-[11px] border-none bg-transparent px-1.5">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem v-for="s in styleOptions" :key="s.id" :value="s.id">
            {{ s.name }}
          </SelectItem>
        </SelectContent>
      </Select>

      <!-- Highlight theme selector -->
      <Select v-model="highlightTheme">
        <SelectTrigger class="h-6 w-[100px] text-[11px] border-none bg-transparent px-1.5">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem v-for="h in HIGHLIGHT_THEMES" :key="h.value" :value="h.value">
            {{ h.label }}
          </SelectItem>
        </SelectContent>
      </Select>

      <Separator orientation="vertical" class="mx-1 h-4" />

      <!-- Text indent toggle -->
      <Tooltip>
        <TooltipTrigger as-child @click="textIndent = !textIndent">
          <Button
            :variant="textIndent ? 'secondary' : 'ghost'"
            size="icon-xs"
            class="text-muted-foreground hover:text-foreground"
          >
            <ListIndentIncrease class="size-3.5" />
          </Button>
        </TooltipTrigger>
        <TooltipContent side="bottom">首行缩进</TooltipContent>
      </Tooltip>

      <!-- Justify toggle -->
      <Tooltip>
        <TooltipTrigger as-child @click="justify = !justify">
          <Button
            :variant="justify ? 'secondary' : 'ghost'"
            size="icon-xs"
            class="text-muted-foreground hover:text-foreground"
          >
            <component :is="justify ? TextAlignJustify : TextAlignStart" class="size-3.5" />
          </Button>
        </TooltipTrigger>
        <TooltipContent side="bottom">{{ justify ? '两端对齐' : '左对齐' }}</TooltipContent>
      </Tooltip>

      <Separator orientation="vertical" class="mx-1 h-4" />

      <!-- AI button -->
      <Tooltip>
        <TooltipTrigger as-child>
          <Button
            variant="ghost"
            size="icon-xs"
            class="text-muted-foreground hover:text-foreground"
            @click="$emit('toggleAi')"
          >
            <Sparkles class="size-3.5" />
          </Button>
        </TooltipTrigger>
        <TooltipContent side="bottom">AI 写作助手</TooltipContent>
      </Tooltip>

      <!-- Snippets dropdown -->
      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <Button variant="ghost" size="icon-xs" class="text-muted-foreground hover:text-foreground" title="插入组件">
            <Puzzle class="size-3.5" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem
            v-for="s in SNIPPETS"
            :key="s.label"
            @select="emit('insert', s.insert)"
          >
            {{ s.label }}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  </TooltipProvider>
</template>
