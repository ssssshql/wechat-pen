<script setup lang="ts">
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

const view = defineModel<'preview' | 'html'>('view', { required: true })
defineProps<{ preview: string; html: string }>()
</script>

<template>
  <section class="flex h-full min-h-0 flex-col overflow-hidden">
    <div class="flex h-8 shrink-0 items-center gap-1 border-b px-2">
      <Tabs v-model="view" class="w-auto">
        <TabsList>
          <TabsTrigger value="preview">预览</TabsTrigger>
          <TabsTrigger value="html">HTML</TabsTrigger>
        </TabsList>
      </Tabs>
    </div>

    <!-- Preview: the iframe content already contains the phone shell -->
    <div v-show="view === 'preview'" class="min-h-0 flex-1 bg-[#e8eaed]">
      <iframe
        class="size-full border-0"
        title="preview"
        sandbox="allow-same-origin allow-scripts"
        :srcdoc="preview"
      />
    </div>

    <!-- HTML source -->
    <pre
      v-show="view === 'html'"
      class="text-muted-foreground scroll-panel w-full p-4 font-mono text-xs leading-relaxed whitespace-pre-wrap break-words"
    >{{ html }}</pre>
  </section>
</template>
