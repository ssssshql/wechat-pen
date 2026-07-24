<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'

const props = withDefaults(
  defineProps<{
    modelValue: number[] // panel flex weights e.g. [260, 1, 1] first is px for settings
    minPx?: number[]
  }>(),
  {
    minPx: () => [200, 280, 280],
  },
)

const emit = defineEmits<{
  'update:modelValue': [number[]]
}>()

const root = ref<HTMLElement | null>(null)
let dragging = -1
let startX = 0
let startWeights: number[] = []

function onPointerDown(i: number, e: PointerEvent) {
  dragging = i
  startX = e.clientX
  startWeights = [...props.modelValue]
  ;(e.target as HTMLElement).setPointerCapture(e.pointerId)
}

function onPointerMove(e: PointerEvent) {
  if (dragging < 0 || !root.value) return
  const rect = root.value.getBoundingClientRect()
  const dx = e.clientX - startX
  const next = [...startWeights]

  if (dragging === 0) {
    // settings px
    next[0] = clamp(startWeights[0] + dx, props.minPx[0], rect.width * 0.4)
  } else {
    // split editor/preview by adjusting relative flex (stored as ratios * 1000)
    const totalFlex = startWeights[1] + startWeights[2]
    const editorPx =
      ((rect.width - startWeights[0]) * startWeights[1]) / totalFlex + dx
    const avail = rect.width - next[0]
    const editor = clamp(editorPx, props.minPx[1], avail - props.minPx[2])
    const preview = avail - editor
    next[1] = editor
    next[2] = preview
  }
  emit('update:modelValue', next)
}

function onPointerUp() {
  dragging = -1
}

function clamp(n: number, min: number, max: number) {
  return Math.max(min, Math.min(max, n))
}

onMounted(() => {
  window.addEventListener('pointermove', onPointerMove)
  window.addEventListener('pointerup', onPointerUp)
})
onBeforeUnmount(() => {
  window.removeEventListener('pointermove', onPointerMove)
  window.removeEventListener('pointerup', onPointerUp)
})

watch(
  () => props.modelValue,
  () => {
    /* parent owns layout */
  },
)
</script>

<template>
  <div ref="root" class="relative flex min-h-0 min-w-0 flex-1 overflow-hidden">
    <div class="min-h-0 overflow-hidden" :style="{ width: modelValue[0] + 'px', flex: '0 0 auto' }">
      <slot name="left" />
    </div>
    <div
      class="bg-border hover:bg-primary/50 z-10 w-1 shrink-0 cursor-col-resize transition-colors"
      @pointerdown="onPointerDown(0, $event)"
    />
    <div class="min-h-0 min-w-0 overflow-hidden" :style="{ flex: modelValue[1] + ' 1 0' }">
      <slot name="center" />
    </div>
    <div
      class="bg-border hover:bg-primary/50 z-10 w-1 shrink-0 cursor-col-resize transition-colors"
      @pointerdown="onPointerDown(1, $event)"
    />
    <div class="min-h-0 min-w-0 overflow-hidden" :style="{ flex: modelValue[2] + ' 1 0' }">
      <slot name="right" />
    </div>
  </div>
</template>
