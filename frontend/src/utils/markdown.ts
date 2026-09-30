import { marked } from 'marked'
import mermaid from 'mermaid'

let mermaidReady = false

const ensureMermaid = () => {
  if (mermaidReady) {
    return
  }
  mermaid.initialize({ startOnLoad: false, securityLevel: 'strict' })
  mermaidReady = true
}

export const markdownToHtml = (markdown: string): string => {
  return marked(markdown || '') as string
}

export const renderMermaid = async (root?: ParentNode | null) => {
  const scope = root ?? document
  const nodes = scope.querySelectorAll<HTMLElement>('pre > code.language-mermaid')
  if (nodes.length === 0) {
    return
  }
  ensureMermaid()
  const blocks: HTMLElement[] = []
  nodes.forEach((node) => {
    const pre = node.parentElement
    if (!pre || pre.dataset.mermaidRendered === 'true') {
      return
    }
    const host = document.createElement('div')
    host.className = 'mermaid'
    host.textContent = node.textContent || ''
    pre.replaceWith(host)
    blocks.push(host)
  })
  if (blocks.length === 0) {
    return
  }
  try {
    await mermaid.run({ nodes: blocks })
  } catch {
    blocks.forEach((block) => {
      block.dataset.mermaidError = 'true'
    })
  }
}
