<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { toast } from 'vue-sonner'
import { Image, Copy, Loader2, RefreshCw, ChevronLeft, ChevronRight, X, Trash2, ImageUp } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { fetchMaterials, deleteMaterial } from '@/lib/api'
import type { MaterialItem } from '@/lib/types'

const emit = defineEmits<{ insert: [text: string]; close: []; selectCover: [mediaId: string] }>()

const items = ref<MaterialItem[]>([])
const totalCount = ref(0)
const offset = ref(0)
const loading = ref(false)
const deleting = ref<Set<string>>(new Set())
const selectedURL = ref<string | null>(null)
const pageSize = 20

async function loadList() {
  loading.value = true
  try {
    const data = await fetchMaterials('image', offset.value, pageSize)
    items.value = data.items
    totalCount.value = data.total_count
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '获取素材列表失败')
  } finally {
    loading.value = false
  }
}

function insertImage(item: MaterialItem) {
  selectedURL.value = item.url
  const md = `![${item.name}](${item.url})`
  emit('insert', md)
  toast.success('已插入图片')
  setTimeout(() => { selectedURL.value = null }, 1500)
}

async function doDelete(item: MaterialItem) {
  if (!window.confirm(`确定删除「${item.name}」？此操作不可撤销。`)) return
  deleting.value.add(item.media_id)
  try {
    await deleteMaterial(item.media_id)
    items.value = items.value.filter(i => i.media_id !== item.media_id)
    totalCount.value--
    toast.success('已删除')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : '删除失败')
  } finally {
    deleting.value.delete(item.media_id)
  }
}

function prevPage() {
  offset.value = Math.max(0, offset.value - pageSize)
  loadList()
}
function nextPage() {
  offset.value = offset.value + pageSize
  loadList()
}

const totalPages = () => Math.max(1, Math.ceil(totalCount.value / pageSize))
const currentPage = () => Math.floor(offset.value / pageSize) + 1

onMounted(() => { loadList() })
</script>

<template>
  <div class="flex h-full flex-col bg-background">
    <!-- Header -->
    <div class="flex h-9 shrink-0 items-center justify-between border-b px-3">
      <div class="flex items-center gap-1.5 text-xs font-medium">
        <Image class="size-3.5" />
        素材库
      </div>
      <Button variant="ghost" size="icon-xs" @click="emit('close')">
        <X class="size-3.5" />
      </Button>
    </div>

    <!-- Toolbar -->
    <div class="flex shrink-0 items-center justify-between px-3 py-2">
      <span class="text-[11px] text-muted-foreground">
        共 {{ totalCount }} 张
        <template v-if="totalCount > 0">· 第 {{ currentPage() }}/{{ totalPages() }} 页</template>
      </span>
      <Button variant="ghost" size="icon-xs" :disabled="loading" @click="loadList">
        <RefreshCw class="size-3" :class="loading ? 'animate-spin' : ''" />
      </Button>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="flex flex-1 items-center justify-center py-8">
      <Loader2 class="size-5 animate-spin text-muted-foreground" />
    </div>

    <!-- Empty -->
    <div v-else-if="!items.length" class="flex flex-1 items-center justify-center py-8">
      <p class="text-xs text-muted-foreground">暂无素材</p>
    </div>

    <!-- Grid -->
    <div v-else class="grid flex-1 auto-rows-min grid-cols-3 gap-2 overflow-auto px-3 pb-2">
      <div
        v-for="item in items"
        :key="item.media_id"
        class="group relative overflow-hidden rounded-lg border"
        :class="selectedURL === item.url ? 'ring-2 ring-primary' : 'bg-muted/30'"
      >
        <img
          :src="item.url"
          :alt="item.name"
          class="h-24 w-full object-cover"
          referrerpolicy="no-referrer"
          loading="lazy"
        />
        <div class="p-1.5">
          <p class="truncate text-[10px] font-medium">{{ item.name }}</p>
          <p class="text-[9px] text-muted-foreground">{{ new Date(item.update_time * 1000).toLocaleDateString() }}</p>
        </div>
        <!-- Overlay actions -->
        <div class="absolute inset-0 flex items-center justify-center gap-1 bg-black/0 opacity-0 transition-all group-hover:bg-black/30 group-hover:opacity-100">
          <Button
            size="xs"
            variant="secondary"
            class="h-7 gap-1 text-[10px]"
            @click="emit('selectCover', item.media_id); toast.success('已选择封面')"
          >
            <ImageUp class="size-3" />封面
          </Button>
          <Button
            size="xs"
            variant="secondary"
            class="h-7 gap-1 text-[10px]"
            @click="insertImage(item)"
          >
            <Copy class="size-3" />插入
          </Button>
          <Button
            size="icon-xs"
            variant="secondary"
            class="size-7"
            :disabled="deleting.has(item.media_id)"
            @click="doDelete(item)"
          >
            <Loader2 v-if="deleting.has(item.media_id)" class="size-3 animate-spin" />
            <Trash2 v-else class="size-3" />
          </Button>
        </div>
      </div>
    </div>

    <!-- Pagination -->
    <div v-if="totalCount > pageSize" class="flex shrink-0 items-center justify-between border-t px-3 py-2">
      <Button variant="ghost" size="xs" :disabled="offset === 0" @click="prevPage">
        <ChevronLeft class="size-3 mr-1" />上一页
      </Button>
      <Button variant="ghost" size="xs" :disabled="offset + pageSize >= totalCount" @click="nextPage">
        下一页<ChevronRight class="size-3 ml-1" />
      </Button>
    </div>
  </div>
</template>
