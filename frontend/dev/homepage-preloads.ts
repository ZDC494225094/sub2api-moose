import type { Plugin } from 'vite'

/** Discover the homepage's lazy chunks at build time, before router startup. */
export function homepagePreloads(): Plugin {
  return {
    name: 'homepage-critical-preloads',
    apply: 'build',
    enforce: 'post',
    generateBundle(_options, bundle) {
      const html = bundle['index.html']
      if (!html || html.type !== 'asset' || typeof html.source !== 'string') return
      const collect = (suffix: string) => {
        const root = Object.values(bundle).find(item => item.type === 'chunk' && item.facadeModuleId?.replace(/\\/g, '/').endsWith(suffix))
        if (!root || root.type !== 'chunk') throw new Error(`Missing homepage preload entry: ${suffix}`)
        const files = new Set<string>()
        const visit = (file: string) => {
          if (files.has(file)) return
          files.add(file)
          const chunk = bundle[file]
          if (chunk?.type !== 'chunk') return
          chunk.imports.forEach(visit)
          const metadata = (chunk as typeof chunk & { viteMetadata?: { importedCss: Set<string> } }).viteMetadata
          metadata?.importedCss.forEach(css => files.add(css))
        }
        visit(root.fileName)
        return Array.from(files)
      }
      const data = {
        home: collect('/features/premium-home/runtime/PremiumHomeView.vue'),
        en: collect('/i18n/locales/en/public.ts'),
        zh: collect('/i18n/locales/zh/public.ts'),
      }
      // Only after the backend gate returns the app document. This script never
      // runs on the independent captcha page or preloads home on console routes.
      const script = `<script nonce="__CSP_NONCE_VALUE__">(()=>{if(location.pathname!=="/")return;const chunks=${JSON.stringify(data)};let locale;try{locale=localStorage.getItem("sub2api_locale")}catch{}if(locale!=="en"&&locale!=="zh")locale=navigator.language.toLowerCase().startsWith("zh")?"zh":"en";for(const file of new Set([...chunks.home,...chunks[locale]])){const href="/"+file;const link=document.createElement("link");link.href=href;link.crossOrigin="";if(file.endsWith(".css")){link.rel="preload";link.as="style"}else{link.rel="modulepreload"}document.head.appendChild(link)}})();</script>`
      // Run before the render-blocking CSS, not at the end of <head> where a
      // classic script would first wait for preceding stylesheets to download.
      html.source = html.source.replace('</title>', `</title>${script}`)

      // The tiny inline boot page can paint before the full application's CSS.
      // Bootstrap still waits for these styles before mounting Vue.
      html.source = html.source.replace(/<link\b[^>]*rel="stylesheet"[^>]*>/g, link => {
        const href = /href="([^"]+)"/.exec(link)?.[1]
        if (!href) return link
        return `<script nonce="__CSP_NONCE_VALUE__">(()=>{const link=document.createElement("link");link.rel="stylesheet";link.href=${JSON.stringify(href)};link.crossOrigin="";link.media="print";link.dataset.appStyles="pending";link.addEventListener("load",()=>{link.media="all";link.dataset.appStyles="ready"});link.addEventListener("error",()=>{link.dataset.appStyles="error"});document.head.appendChild(link)})();</script>`
      })
    },
  }
}
