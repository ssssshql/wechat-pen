<script setup lang="ts">
import {
  Bold,
  Code2,
  Heading2,
  Image as ImageIcon,
  Italic,
  Link2,
  List,
  ListOrdered,
  Quote,
} from '@lucide/vue'
import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'

const emit = defineEmits<{
  action: [type: string]
}>()

const actions: { id: string; label: string; icon: unknown }[] = [
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
    <div class="toolbar-row flex flex-wrap items-center gap-px border-b px-1.5 py-0.5">
      <Tooltip v-for="a in actions" :key="a.id">
        <TooltipTrigger as-child>
          <Button
            variant="ghost"
            size="icon-xs"
            class="text-muted-foreground hover:text-foreground"
            @click="emit('action', a.id)"
          >
            <component :is="a.icon" class="size-3.5" />
          </Button>
        </TooltipTrigger>
        <TooltipContent side="bottom">{{ a.label }}</TooltipContent>
      </Tooltip>
    </div>
  </TooltipProvider>
</template>
