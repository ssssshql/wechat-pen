<script setup lang="ts">
import { computed } from 'vue'
import { Codemirror } from 'vue-codemirror'
import { markdown as mdLang, markdownLanguage } from '@codemirror/lang-markdown'
import { EditorView } from '@codemirror/view'
import { HighlightStyle, syntaxHighlighting } from '@codemirror/language'
import { tags as t } from '@lezer/highlight'
import type { Extension } from '@codemirror/state'

const model = defineModel<string>({ required: true })

const props = withDefaults(
  defineProps<{
    placeholder?: string
  }>(),
  {
    placeholder: '在此编写 Markdown…',
  },
)

const emit = defineEmits<{
  ready: [view: EditorView]
}>()

let view: EditorView | null = null

// Syntax colors tuned for light shadcn UI (not oneDark).
const mdHighlight = HighlightStyle.define([
  { tag: t.heading1, color: '#0f172a', fontWeight: '700', fontSize: '1.15em' },
  { tag: t.heading2, color: '#0f172a', fontWeight: '700', fontSize: '1.08em' },
  { tag: t.heading3, color: '#1e293b', fontWeight: '650' },
  { tag: t.heading4, color: '#334155', fontWeight: '650' },
  { tag: t.strong, color: '#0f172a', fontWeight: '700' },
  { tag: t.emphasis, color: '#334155', fontStyle: 'italic' },
  { tag: t.strikethrough, textDecoration: 'line-through', color: '#94a3b8' },
  { tag: t.link, color: '#2563eb', textDecoration: 'underline' },
  { tag: t.url, color: '#64748b' },
  { tag: t.monospace, color: '#be123c', backgroundColor: 'rgba(244,63,94,0.08)', borderRadius: '3px' },
  { tag: t.quote, color: '#64748b', fontStyle: 'italic' },
  { tag: t.list, color: '#475569' },
  { tag: t.meta, color: '#94a3b8' },
  { tag: t.comment, color: '#94a3b8', fontStyle: 'italic' },
  { tag: t.keyword, color: '#7c3aed' },
  { tag: t.string, color: '#059669' },
  { tag: t.number, color: '#d97706' },
  { tag: t.bool, color: '#d97706' },
  { tag: t.operator, color: '#64748b' },
  { tag: t.punctuation, color: '#94a3b8' },
  { tag: t.function(t.variableName), color: '#2563eb' },
  { tag: t.definition(t.variableName), color: '#0f172a' },
  { tag: t.typeName, color: '#0891b2' },
  { tag: t.className, color: '#0891b2' },
  { tag: t.propertyName, color: '#0e7490' },
  { tag: t.atom, color: '#c026d3' },
  { tag: t.processingInstruction, color: '#7c3aed' },
  { tag: t.contentSeparator, color: '#cbd5e1' },
])

const extensions = computed<Extension[]>(() => [
  mdLang({ base: markdownLanguage }),
  syntaxHighlighting(mdHighlight),
  EditorView.lineWrapping,
  EditorView.theme({
    '&': {
      height: '100%',
      fontSize: '13.5px',
      backgroundColor: 'transparent',
      color: 'var(--foreground)',
    },
    '.cm-scroller': {
      fontFamily:
        'ui-monospace, SFMono-Regular, Menlo, Consolas, "Cascadia Code", monospace',
      lineHeight: '1.7',
      overflow: 'auto',
    },
    '.cm-content': {
      padding: '18px 16px 48px 8px',
      caretColor: 'var(--foreground)',
      minHeight: '100%',
    },
    '.cm-line': {
      padding: '0 2px',
    },
    '.cm-gutters': {
      backgroundColor: 'transparent',
      border: 'none',
      color: 'var(--muted-foreground)',
      minWidth: '40px',
    },
    '.cm-gutterElement': {
      padding: '0 8px 0 12px',
      opacity: '0.45',
    },
    '.cm-activeLineGutter': {
      backgroundColor: 'transparent',
      opacity: '0.85',
      color: 'var(--foreground)',
    },
    '&.cm-focused': {
      outline: 'none',
    },
    '.cm-activeLine': {
      backgroundColor: 'color-mix(in oklab, var(--muted) 65%, transparent)',
    },
    '.cm-selectionBackground, &.cm-focused .cm-selectionBackground': {
      backgroundColor: 'color-mix(in oklab, var(--primary) 18%, transparent) !important',
    },
    '.cm-cursor, .cm-dropCursor': {
      borderLeftColor: 'var(--foreground)',
    },
    '.cm-placeholder': {
      color: 'var(--muted-foreground)',
      fontStyle: 'normal',
    },
    // fenced code block feel
    '.cm-line:has(.tok-monospace)': {},
  }),
])

function onReady(payload: { view: EditorView }) {
  view = payload.view
  emit('ready', payload.view)
}

function insertAtCursor(text: string) {
  if (!view) {
    model.value = (model.value || '') + text
    return
  }
  const { state } = view
  const from = state.selection.main.from
  const to = state.selection.main.to
  view.dispatch({
    changes: { from, to, insert: text },
    selection: { anchor: from + text.length },
  })
  view.focus()
}

function wrapSelection(before: string, after: string, placeholder = '') {
  if (!view) {
    model.value = before + (model.value || placeholder) + after
    return
  }
  const { state } = view
  const { from, to } = state.selection.main
  const selected = state.sliceDoc(from, to) || placeholder
  const insert = before + selected + after
  view.dispatch({
    changes: { from, to, insert },
    selection: {
      anchor: from + before.length,
      head: from + before.length + selected.length,
    },
  })
  view.focus()
}

function prefixLines(prefix: string) {
  if (!view) {
    model.value = prefix + (model.value || '')
    return
  }
  const { state } = view
  const { from, to } = state.selection.main
  const startLine = state.doc.lineAt(from)
  const endLine = state.doc.lineAt(to)
  const changes: { from: number; to: number; insert: string }[] = []
  for (let n = startLine.number; n <= endLine.number; n++) {
    const line = state.doc.line(n)
    changes.push({ from: line.from, to: line.from, insert: prefix })
  }
  view.dispatch({ changes })
  view.focus()
}

function runToolbar(action: string) {
  switch (action) {
    case 'h2':
      prefixLines('## ')
      break
    case 'bold':
      wrapSelection('**', '**', '加粗')
      break
    case 'italic':
      wrapSelection('*', '*', '斜体')
      break
    case 'quote':
      prefixLines('> ')
      break
    case 'ul':
      prefixLines('- ')
      break
    case 'ol':
      prefixLines('1. ')
      break
    case 'code':
      wrapSelection('\n```\n', '\n```\n', 'code')
      break
    case 'link':
      wrapSelection('[', '](https://)', '链接文字')
      break
    case 'image':
      insertAtCursor('\n![说明](https://example.com/img.png)\n')
      break
    default:
      break
  }
}

defineExpose({ insertAtCursor, wrapSelection, prefixLines, runToolbar, getView: () => view })
</script>

<template>
  <div class="cm-host min-h-0 flex-1 overflow-hidden">
    <Codemirror
      v-model="model"
      :extensions="extensions"
      :autofocus="false"
      :indent-with-tab="true"
      :tab-size="2"
      :placeholder="placeholder"
      class="h-full"
      @ready="onReady"
    />
  </div>
</template>

<style>
.cm-host,
.cm-host .v-codemirror,
.cm-host .cm-editor {
  height: 100%;
}
.cm-host .cm-editor.cm-focused {
  outline: none;
}
.cm-host .cm-editor {
  background: transparent !important;
}
.cm-host .cm-gutters {
  background: transparent !important;
  border-right: none !important;
}
/* CodeMirror uses its own scroller — match app scrollbar */
.cm-host .cm-scroller {
  scrollbar-width: thin;
  scrollbar-color: color-mix(in oklab, var(--muted-foreground) 28%, transparent) transparent;
}
.cm-host .cm-scroller::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}
.cm-host .cm-scroller::-webkit-scrollbar-thumb {
  background: color-mix(in oklab, var(--muted-foreground) 28%, transparent);
  border-radius: 999px;
  border: 2px solid transparent;
  background-clip: content-box;
}
.cm-host .cm-scroller::-webkit-scrollbar-thumb:hover {
  background: color-mix(in oklab, var(--muted-foreground) 48%, transparent);
  border: 2px solid transparent;
  background-clip: content-box;
}
.cm-host .cm-scroller::-webkit-scrollbar-track {
  background: transparent;
}
</style>
