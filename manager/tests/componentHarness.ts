import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import { fileURLToPath } from 'node:url'
import * as Vue from 'vue'
import ts from 'typescript'
import { vi } from 'vitest'
import { ApiError } from '../app/composables/useApi'
import messages from '../i18n/locales/en.json'

// Compile the actual SFC, including its template. Only UI primitives and
// external boundaries are supplied by tests; component handlers stay real.
const require = createRequire(import.meta.url)
const vueRequire = createRequire(require.resolve('vue'))
const compiler = vueRequire('@vue/compiler-sfc') as {
  parse: (source: string, options: { filename: string }) => { descriptor: unknown }
  compileScript: (descriptor: unknown, options: { id: string, inlineTemplate: boolean }) => { content: string }
}

export function translate(key: string, values?: Record<string, unknown>): string {
  let value: unknown = messages
  for (const part of key.split('.')) {
    value = (value as Record<string, unknown>)?.[part]
  }
  return typeof value === 'string'
    ? value.replace(/\{(\w+)\}/g, (match, name: string) => String(values?.[name] ?? match))
    : key
}

export function installComponentGlobals() {
  for (const [name, value] of Object.entries(Vue)) {
    vi.stubGlobal(name, value)
  }
  vi.stubGlobal('useI18n', () => ({ t: translate }))
}

const compiledSources = new Map<string, { source: string, javascript: string }>()

function transpile(source: string): string {
  return ts.transpileModule(source, {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS }
  }).outputText
}

function compiledSource(filename: string, compile: (source: string) => string): string {
  const source = readFileSync(filename, 'utf8')
  let cached = compiledSources.get(filename)
  if (cached?.source !== source) {
    cached = { source, javascript: compile(source) }
    compiledSources.set(filename, cached)
  }
  return cached.javascript
}

function evaluate(javascript: string, imports: Record<string, unknown>): unknown {
  const exports: Record<string, unknown> = {}
  const resolve = (name: string) => {
    if (!(name in imports)) {
      throw new Error(`Component harness needs an explicit dependency for ${name}`)
    }
    const imported = imports[name]
    return imported && typeof imported === 'object' && 'default' in imported
      ? { __esModule: true, ...imported }
      : imported
  }
  new Function('require', 'exports', javascript)(resolve, exports)
  return exports
}

export function loadComponent(relativePath: string, imports: Record<string, unknown> = {}): Vue.Component {
  const filename = fileURLToPath(new URL(`../app/${relativePath}`, import.meta.url))
  const javascript = compiledSource(filename, (source) => {
    const descriptor = compiler.parse(source, { filename }).descriptor
    const compiled = compiler.compileScript(descriptor, { id: relativePath, inlineTemplate: true })
    return transpile(compiled.content)
  })
  const module = evaluate(javascript, {
    'vue': Vue,
    '~/composables/useApi': { ApiError },
    '~/components/shared/FileUploadPreview.vue': { default: primitive('FileUploadPreview') },
    ...imports
  }) as { default: Vue.Component }
  return module.default
}

export function loadScript(relativePath: string, imports: Record<string, unknown>): Record<string, unknown> {
  const filename = fileURLToPath(new URL(`../app/${relativePath}`, import.meta.url))
  return evaluate(compiledSource(filename, transpile), imports) as Record<string, unknown>
}

export interface RenderNode {
  type: string
  text: string
  props: Record<string, unknown>
  children: RenderNode[]
  parent: RenderNode | null
}

function node(type: string, text = ''): RenderNode {
  return { type, text, props: {}, children: [], parent: null }
}

function remove(child: RenderNode) {
  if (child.parent) {
    child.parent.children.splice(child.parent.children.indexOf(child), 1)
    child.parent = null
  }
}

function insert(child: RenderNode, parent: RenderNode, anchor: RenderNode | null = null) {
  remove(child)
  const index = anchor ? parent.children.indexOf(anchor) : -1
  parent.children.splice(index < 0 ? parent.children.length : index, 0, child)
  child.parent = parent
}

const renderer = Vue.createRenderer<RenderNode, RenderNode>({
  createElement: type => node(type),
  createText: text => node('#text', text),
  createComment: text => node('#comment', text),
  setText: (target, text) => { target.text = text },
  setElementText: (target, text) => {
    target.text = text
    target.children = []
  },
  patchProp: (target, key, _previous, value) => { target.props[key] = value },
  insert,
  remove,
  parentNode: target => target.parent,
  nextSibling: target => target.parent?.children[target.parent.children.indexOf(target) + 1] ?? null,
  insertStaticContent: (content, parent, anchor) => {
    const target = node('#static', content)
    insert(target, parent, anchor)
    return [target, target]
  }
})

export function primitive(name: string): Vue.Component {
  return Vue.defineComponent({
    name,
    inheritAttrs: false,
    props: ['modelValue'],
    setup(props, { attrs, slots }) {
      return () => Vue.h(name, { ...attrs, modelValue: props.modelValue }, [
        typeof attrs.label === 'string' ? attrs.label : '',
        ...Object.entries(slots).flatMap(([slotName, slot]) => {
          if (name === 'UFileUpload' && slotName === 'file-leading') {
            const files = Array.isArray(props.modelValue) ? props.modelValue : props.modelValue ? [props.modelValue] : []
            return files.flatMap((file, index) => slot?.({ file, index, ui: { fileLeadingAvatar: () => '' } }) ?? [])
          }
          return slot?.({}) ?? []
        })
      ])
    }
  })
}

const primitiveNames = [
  'UPageCard', 'UCard', 'UAlert', 'UFormField', 'UInput', 'UTextarea',
  'UFileUpload', 'UButton', 'UTabs', 'USelect', 'UAvatar', 'UEmpty',
  'UDashboardPanel', 'UDashboardNavbar', 'UDashboardSidebarCollapse',
  'USkeleton', 'UModal'
]
const mounted: Vue.App[] = []

export async function flushComponent() {
  for (let i = 0; i < 10; i += 1) {
    await Promise.resolve()
    await Vue.nextTick()
  }
}

export async function mountComponent(component: Vue.Component, props: Record<string, unknown> = {}) {
  const root = node('root')
  const reactiveProps = Vue.reactive(props)
  const app = renderer.createApp({
    render: () => Vue.h(Vue.Suspense, {}, { default: () => Vue.h(component, reactiveProps) })
  })
  for (const name of primitiveNames) {
    app.component(name, primitive(name))
  }
  app.mount(root)
  mounted.push(app)
  await flushComponent()

  function findAll(type: string): RenderNode[] {
    const matches: RenderNode[] = []
    function visit(target: RenderNode) {
      if (target.type === type) matches.push(target)
      target.children.forEach(visit)
    }
    visit(root)
    return matches
  }

  function text(target: RenderNode = root): string {
    return target.type === '#comment' ? '' : [target.text, ...target.children.map(child => text(child))].join('')
  }

  function button(label: string): RenderNode {
    const target = findAll('UButton').find(target => target.props.label === label)
    if (!target) throw new Error(`Button not found: ${label}`)
    return target
  }

  async function trigger(target: RenderNode, event: string, value?: unknown) {
    const handler = target.props[event] as (value?: unknown) => unknown
    if (typeof handler !== 'function') throw new Error(`Missing handler ${event} on ${target.type}`)
    const result = handler(value)
    await flushComponent()
    return result
  }

  return { root, props: reactiveProps, findAll, text, button, trigger }
}

export function unmountComponents() {
  mounted.splice(0).forEach(app => app.unmount())
}
