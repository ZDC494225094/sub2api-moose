import { marked } from 'marked'
import DOMPurify from 'dompurify'

marked.setOptions({
  breaks: true,
  gfm: true,
})

export interface GuideTocItem {
  id: string
  text: string
  level: number
}

/**
 * Render admin-authored markdown to sanitized HTML and build a table of
 * contents from its h2/h3 headings (ids are injected before sanitizing).
 */
export function renderMarkdown(md: string): { html: string; toc: GuideTocItem[] } {
  if (!md.trim()) return { html: '', toc: [] }
  const raw = marked.parse(md) as string
  const doc = new DOMParser().parseFromString(raw, 'text/html')
  const toc: GuideTocItem[] = []
  let n = 0
  doc.querySelectorAll('h2, h3').forEach((el) => {
    n += 1
    const id = `guide-h-${n}`
    el.setAttribute('id', id)
    toc.push({ id, text: el.textContent || '', level: el.tagName === 'H2' ? 2 : 3 })
  })
  doc.querySelectorAll('a[href^="http"]').forEach((el) => {
    el.setAttribute('target', '_blank')
    el.setAttribute('rel', 'noopener')
  })
  const html = DOMPurify.sanitize(doc.body.innerHTML, { ADD_ATTR: ['target'] })
  return { html, toc }
}
