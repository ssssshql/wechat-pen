<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Loader2, Sparkles, X, ArrowUp } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { toast } from 'vue-sonner'
import {
  fetchWritingStyles,
  aiChatStream,
  type WritingStyle,
  type ChatMessage,
} from '@/lib/api'

const emit = defineEmits<{
  streamStart: []
  streamToken: [token: string]
  streamEnd: []
  streamCancel: []
}>()

const show = defineModel<boolean>('show', { required: true })
const editorContent = defineModel<string>('editorContent', { required: false })

// ---- Input state ----
const inputText = ref('')
const generating = ref(false)
const charCount = ref(0)
let chatAbort: (() => void) | null = null

// Conversation context (kept silently, not displayed)
const chatMessages = ref<ChatMessage[]>([])

// ---- Writing style ----
const savedStyles = ref<WritingStyle[]>([])
const selectedStyleId = ref('')

async function loadStyles() {
  try { savedStyles.value = await fetchWritingStyles() } catch { /* ignore */ }
}

// ---- Actions ----
function sendMessage() {
  const text = inputText.value.trim()
  if (!text || generating.value) return

  // Add to conversation context
  chatMessages.value.push({ role: 'user', content: text })
  inputText.value = ''
  charCount.value = 0
  generating.value = true

  emit('streamStart')

  chatAbort = aiChatStream(
    {
      messages: chatMessages.value,
      currentContent: editorContent?.value || '',
      styleId: selectedStyleId.value || undefined,
    },
    (token) => {
      charCount.value += token.length
      emit('streamToken', token)
    },
    () => {
      generating.value = false
      chatMessages.value.push({ role: 'assistant', content: `✓ 已生成 ${charCount.value} 字` })
      emit('streamEnd')
    },
    (err) => {
      generating.value = false
      toast.error('AI 生成失败: ' + err)
      emit('streamEnd')
    },
  )
}

function cancelGenerate() {
  chatAbort?.()
  generating.value = false
  emit('streamCancel')
}

function closePanel() {
  if (generating.value) cancelGenerate()
  chatMessages.value = []
  charCount.value = 0
  show.value = false
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    sendMessage()
  }
  if (e.key === 'Escape') {
    closePanel()
  }
}

onMounted(() => { loadStyles() })
</script>

<template>
  <div class="ai-input-bar shrink-0 flex items-center gap-1.5 px-3 py-1.5 border-t bg-background/80">
    <Select v-model="selectedStyleId">
      <SelectTrigger class="h-7 w-[88px] text-[10px] shrink-0">
        <SelectValue placeholder="写作风格" />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="">自由写作</SelectItem>
        <SelectItem v-for="s in savedStyles" :key="s.id" :value="s.id">
          {{ s.nickname }}
        </SelectItem>
      </SelectContent>
    </Select>

    <div v-if="generating" class="flex items-center gap-2 flex-1 min-w-0">
      <Loader2 class="size-3 animate-spin text-primary shrink-0" />
      <span class="text-[11px] text-muted-foreground truncate">
        正在生成{{ charCount > 0 ? ` (${charCount}字)` : '' }}...
      </span>
      <Button variant="ghost" size="icon-xs" class="shrink-0" @click="cancelGenerate">
        <X class="size-3.5" />
      </Button>
    </div>

    <Input
      v-else
      v-model="inputText"
      class="h-7 text-xs flex-1 min-w-0"
      placeholder="告诉我你想写什么…"
      @keydown="onKeydown"
    />

    <Button
      v-if="!generating"
      size="icon-xs"
      :disabled="!inputText.trim()"
      @click="sendMessage"
    >
      <ArrowUp class="size-3.5" />
    </Button>
  </div>
</template>
