<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import { Loader2, Sparkles, X, ArrowUp, RotateCcw } from '@lucide/vue'
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

const emit = defineEmits(['close', 'streamStart', 'streamToken', 'streamEnd', 'streamCancel'])

const editorContent = defineModel<string>('editorContent', { required: false })

const inputText = ref('')
const generating = ref(false)
const charCount = ref(0)
const currentResponse = ref('')
let chatAbort: (() => void) | null = null

const chatMessages = ref<ChatMessage[]>([])
const messagesContainer = ref<HTMLDivElement | null>(null)

const savedStyles = ref<WritingStyle[]>([])
const selectedStyleId = ref('')

const selectedStyleName = computed(() => {
  if (!selectedStyleId.value) return ''
  return savedStyles.value.find(s => s.id === selectedStyleId.value)?.nickname || ''
})

async function loadStyles() {
  try { savedStyles.value = await fetchWritingStyles() } catch { /* ignore */ }
}

const quickPrompts = [
  { label: '续写文章', desc: '基于当前内容继续往下写', prompt: '请基于当前文章内容继续往下续写' },
  { label: '润色优化', desc: '优化表达和措辞', prompt: '请对当前文章进行润色，优化表达和措辞' },
  { label: '生成摘要', desc: '为当前文章生成摘要', prompt: '请为当前文章生成一段简短的摘要' },
  { label: '改写段落', desc: '选中文字后改写', prompt: '请帮我改写这段文字，保持原意但换个表达方式' },
]

function scrollToBottom() {
  nextTick(() => {
    const el = messagesContainer.value
    if (el) el.scrollTop = el.scrollHeight
  })
}

function applyQuickPrompt(prompt: string) {
  inputText.value = prompt
  sendMessage()
}

function sendMessage() {
  const text = inputText.value.trim()
  if (!text || generating.value) return

  chatMessages.value.push({ role: 'user', content: text })
  inputText.value = ''
  charCount.value = 0
  currentResponse.value = ''
  generating.value = true

  scrollToBottom()
  emit('streamStart')

  chatAbort = aiChatStream(
    {
      messages: chatMessages.value,
      currentContent: editorContent?.value || '',
      styleId: selectedStyleId.value || undefined,
    },
    (token) => {
      charCount.value += token.length
      currentResponse.value += token
      emit('streamToken', token)
      scrollToBottom()
    },
    () => {
      generating.value = false
      if (currentResponse.value) {
        chatMessages.value.push({ role: 'assistant', content: currentResponse.value })
      }
      currentResponse.value = ''
      emit('streamEnd')
      scrollToBottom()
    },
    (err) => {
      generating.value = false
      if (currentResponse.value) {
        chatMessages.value.push({ role: 'assistant', content: currentResponse.value + '\n\n[生成中断: ' + err + ']' })
      }
      toast.error('AI 生成失败: ' + err)
      currentResponse.value = ''
      emit('streamEnd')
    },
  )
}

function cancelGenerate() {
  chatAbort?.()
  generating.value = false
  emit('streamCancel')
}

function clearChat() {
  if (generating.value) cancelGenerate()
  chatMessages.value = []
  charCount.value = 0
  currentResponse.value = ''
}

function closePanel() {
  if (generating.value) cancelGenerate()
  chatMessages.value = []
  charCount.value = 0
  currentResponse.value = ''
  console.log('[AI] closePanel calling emit')
  console.log('[AI] closePanel calling emit')
  emit('close')
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
  <div class="flex flex-col h-full bg-background border-l" style="width: 360px;">
    <!-- Header -->
    <div class="flex items-center justify-between px-3 py-1.5 border-b shrink-0">
      <div class="flex items-center gap-1.5">
        <Sparkles class="size-3.5 text-primary" />
        <span class="text-xs font-medium">AI 写作助手</span>
        <span v-if="selectedStyleName" class="inline-flex items-center rounded-full bg-secondary px-2 py-0.5 text-[10px] text-secondary-foreground">
          {{ selectedStyleName }}
        </span>
      </div>
      <div class="flex items-center gap-0.5">
        <button type="button" class="flex size-6 items-center justify-center rounded-md text-muted-foreground hover:text-foreground hover:bg-black/5" title="清空对话" @click="clearChat">
          <RotateCcw class="size-3" />
        </button>
        <button type="button" class="flex size-6 items-center justify-center rounded-md text-muted-foreground hover:text-foreground hover:bg-black/5" title="关闭" @click="closePanel">
          <X class="size-3" />
        </button>
      </div>
    </div>

    <!-- Body -->
    <div v-if="!chatMessages.length && !generating" class="flex-1 flex flex-col items-center justify-center px-4 py-6 min-h-[120px]">
      <Sparkles class="size-8 text-muted-foreground/20 mb-3" />
      <p class="text-xs text-muted-foreground mb-4">选择一个快捷操作，或直接输入你的需求</p>
      <div class="grid grid-cols-2 gap-2 w-full max-w-sm">
        <button
          v-for="q in quickPrompts"
          :key="q.label"
          type="button"
          class="text-left rounded-lg border px-3 py-2 hover:bg-muted/60 transition-colors"
          @click="applyQuickPrompt(q.prompt)"
        >
          <span class="text-xs font-medium block mb-0.5">{{ q.label }}</span>
          <span class="text-[11px] text-muted-foreground line-clamp-2 leading-snug">{{ q.desc }}</span>
        </button>
      </div>
    </div>

    <div v-show="chatMessages.length || generating" ref="messagesContainer" class="flex-1 min-h-[80px] overflow-y-auto">
      <div class="px-3 py-3 space-y-3">
        <template v-for="(msg, idx) in chatMessages" :key="idx">
          <div v-if="msg.role === 'user'" class="flex justify-end">
            <div class="bg-primary text-primary-foreground rounded-xl rounded-br-sm px-3 py-1.5 text-xs max-w-[85%] whitespace-pre-wrap break-words">
              {{ msg.content }}
            </div>
          </div>
          <div v-if="msg.role === 'assistant'" class="flex justify-start">
            <div class="bg-muted rounded-xl rounded-bl-sm px-3 py-1.5 text-xs max-w-[90%] whitespace-pre-wrap break-words leading-relaxed">
              {{ msg.content }}
            </div>
          </div>
        </template>
        <div v-if="generating && currentResponse" class="flex justify-start">
          <div class="bg-muted rounded-xl rounded-bl-sm px-3 py-1.5 text-xs max-w-[90%] whitespace-pre-wrap break-words leading-relaxed">
            {{ currentResponse }}<span class="inline-block w-1.5 h-3 bg-foreground/40 ml-0.5 animate-pulse align-text-bottom" />
          </div>
        </div>
      </div>
    </div>

    <!-- Streaming status -->
    <div v-show="generating" class="flex items-center gap-2 px-3 py-1.5 border-t text-xs shrink-0 bg-muted/30">
      <Loader2 class="size-3 animate-spin text-primary" />
      <span class="text-muted-foreground">正在生成</span>
      <span class="inline-flex items-center rounded-full bg-secondary px-1.5 py-0.5 text-[10px] tabular-nums text-secondary-foreground">
        {{ charCount }} 字
      </span>
      <div class="ml-auto">
        <button type="button" class="flex items-center gap-1 text-muted-foreground hover:text-foreground text-[11px]" @click="cancelGenerate">
          <X class="size-3" />停止
        </button>
      </div>
    </div>

    <!-- Footer -->
    <div class="border-t shrink-0">
      <div class="flex items-center gap-1.5 px-3 pt-2 pb-1">
        <Select v-model="selectedStyleId">
          <SelectTrigger class="w-[130px] h-7 text-[11px] shrink-0">
            <SelectValue placeholder="自由写作" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="s in savedStyles" :key="s.id" :value="s.id" class="text-xs">
              {{ s.nickname }}
            </SelectItem>
          </SelectContent>
        </Select>
        <span class="ml-auto text-[10px] text-muted-foreground tabular-nums">
          {{ [...inputText].length }} 字
        </span>
      </div>
      <div class="flex items-end gap-1.5 px-3 pb-2">
        <Input
          v-model="inputText"
          class="h-7 text-xs flex-1 min-w-0"
          placeholder="告诉我你想写什么…"
          :disabled="generating"
          @keydown="onKeydown"
        />
        <button
          type="button"
          class="flex size-7 shrink-0 items-center justify-center rounded-md bg-primary text-primary-foreground disabled:opacity-50 disabled:pointer-events-none"
          :disabled="!inputText.trim() || generating"
          @click="sendMessage"
        >
          <ArrowUp class="size-3.5" />
        </button>
      </div>
    </div>
  </div>
</template>
