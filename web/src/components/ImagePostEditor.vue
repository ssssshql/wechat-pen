<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { toast } from 'vue-sonner'
import { Image, Loader2, Plus, Sparkles, X, Upload } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { generateImage } from '@/lib/api'

interface ImageSlot {
  url: string
  mediaId?: string
}

const props = defineProps<{
  title: string
  text: string
  images: (ImageSlot | null)[]
}>()

const emit = defineEmits<{
  'update:title': [value: string]
  'update:text': [value: string]
  'update:images': [value: (ImageSlot | null)[]]
}>()

const localTitle = ref(props.title)
const localText = ref(props.text)
const localImages = ref<(ImageSlot | null)[]>(props.images?.length === 4 ? [...props.images] : [null, null, null, null])

watch(() => props.title, v => localTitle.value = v)
watch(() => props.text, v => localText.value = v)
watch(() => props.images, v => {
  if (v?.length === 4) localImages.value = [...v]
  else localImages.value = [null, null, null, null]
}, { deep: true })

watch(localTitle, v => emit('update:title', v))
watch(localText, v => emit('update:text', v))
watch(localImages, v => emit('update:images', [...v]), { deep: true })

const uploadingSlot = ref(-1)
const aiPrompt = ref('')
const aiGenerating = ref(false)
const showAiInput = ref(false)

const textCharCount = computed(() => [...localText.value].length)
const filledCount = computed(() => localImages.value.filter(i => i?.url).length)

const firstEmptySlot = computed(() => {
  for (let i = 0; i < localImages.value.length; i++) {
    if (!localImages.value[i]) return i
  }
  return -1
})

function removeImage(index: number) {
  localImages.value[index] = null
}

function readFileAsDataURL(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as string)
    reader.onerror = () => reject(new Error('读取文件失败'))
    reader.readAsDataURL(file)
  })
}

function triggerUpload(index: number) {
  const input = document.createElement('input')
  input.type = 'file'
  input.accept = 'image/*'
  input.onchange = async () => {
    const file = input.files?.[0]
    if (!file) return
    uploadingSlot.value = index
    try {
      const dataUrl = await readFileAsDataURL(file)
      localImages.value[index] = { url: dataUrl }
    } catch (e: any) {
      toast.error('加载图片失败: ' + (e.message || e))
    } finally {
      uploadingSlot.value = -1
    }
  }
  input.click()
}

async function aiGenerateForSlot(slot: number) {
  if (slot < 0 || !aiPrompt.value.trim()) return
  aiGenerating.value = true
  try {
    const result = await generateImage(aiPrompt.value.trim())
    if (result.data?.[0]?.url) {
      localImages.value[slot] = { url: result.data[0].url }
    }
    showAiInput.value = false
    aiPrompt.value = ''
  } catch (e: any) {
    toast.error('图片生成失败: ' + (e.message || e))
  } finally {
    aiGenerating.value = false
  }
}
</script>

<template>
  <div class="flex flex-1 flex-col bg-[#fbfbfa] overflow-hidden">
    <!-- Title -->
    <div class="shrink-0 px-5 pt-3 pb-1">
      <Input
        v-model="localTitle"
        class="h-9 text-base font-semibold border-transparent bg-transparent hover:border-[#eaeaea] focus:border-[#07c160]"
        placeholder="贴图标题"
      />
    </div>

    <!-- Image strip -->
    <div class="shrink-0 px-5 pb-2">
      <div class="flex gap-2">
        <div
          v-for="(_, i) in 4"
          :key="i"
          class="relative w-[calc(25%-6px)] aspect-square shrink-0 rounded-lg overflow-hidden group cursor-pointer"
          :class="localImages[i]?.url ? 'border border-[#eaeaea]' : ''"
          @click="triggerUpload(i)"
        >
          <template v-if="localImages[i]?.url">
            <img :src="localImages[i]!.url" class="size-full object-cover" referrerpolicy="no-referrer" loading="lazy" />
            <div class="absolute inset-0 bg-black/0 group-hover:bg-black/30 transition-colors flex items-center justify-center gap-1 opacity-0 group-hover:opacity-100">
              <button type="button" class="flex size-6 items-center justify-center rounded bg-white/90 hover:bg-white shadow-sm" @click.stop="triggerUpload(i)" title="替换">
                <Upload class="size-3" />
              </button>
              <button type="button" class="flex size-6 items-center justify-center rounded bg-white/90 hover:bg-white shadow-sm text-red-500" @click.stop="removeImage(i)" title="删除">
                <X class="size-3" />
              </button>
            </div>
          </template>
          <template v-else-if="uploadingSlot === i">
            <div class="flex size-full items-center justify-center bg-[#f5f5f5]">
              <Loader2 class="size-4 animate-spin text-[#787774]" />
            </div>
          </template>
          <template v-else>
            <div class="flex size-full items-center justify-center border-2 border-dashed border-[#ddd] rounded-lg bg-[#fafafa] hover:border-[#07c160] hover:bg-[#f0faf4] transition-colors">
              <Plus class="size-5 text-[#ccc]" />
            </div>
          </template>
        </div>
      </div>

      <!-- AI + info row -->
      <div class="mt-2 flex items-center gap-2">
        <Button variant="outline" size="xs" class="h-6 text-[10px] gap-1" @click="showAiInput = !showAiInput">
          <Sparkles class="size-3" />
          AI 生图
        </Button>
        <span class="text-[10px] text-[#a0a09a]">{{ filledCount }}/4</span>
      </div>
      <div v-if="showAiInput" class="mt-1.5 flex gap-2">
        <Input v-model="aiPrompt" class="h-7 text-xs flex-1" placeholder="描述你想生成的图片..." @keydown.enter="aiGenerateForSlot(firstEmptySlot)" />
        <Button variant="default" size="xs" class="h-7 w-7" :disabled="aiGenerating || !aiPrompt.trim()" @click="aiGenerateForSlot(firstEmptySlot)">
          <Loader2 v-if="aiGenerating" class="size-3 animate-spin" />
          <Image v-else class="size-3" />
        </Button>
      </div>
    </div>

    <!-- Text (main area) -->
    <div class="flex-1 min-h-0 px-5 pb-4 pt-1">
      <div class="relative h-full">
        <textarea
          v-model="localText"
          maxlength="1000"
          class="w-full h-full resize-none rounded-lg border border-[#eaeaea] px-3 py-2 text-sm text-[#2f3437] bg-white outline-none focus:border-[#07c160] transition-colors placeholder:text-[#a0a09a]"
          placeholder="补充说明文字（不超过1000字）..."
        />
        <span class="absolute bottom-2 right-3 text-[10px] tabular-nums pointer-events-none" :class="textCharCount > 900 ? 'text-amber-500' : 'text-[#bbb]'">
          {{ textCharCount }}/1000
        </span>
      </div>
    </div>
  </div>
</template>
