// Pointer field for the glass theme's honeycomb backdrop.
//
// theme.css draws a flat-top hexagon grid on the console backdrop
// (.bg-mesh-gradient), the auth backdrop (.auth-layout-decor) and the homepage /
// docs backdrop (.home-backdrop). This keeps a few
// CSS variables on those layers in step with the mouse so the grid lights up
// around the pointer:
//   --ct-px / --ct-py  pointer position inside the layer (spotlight)
//   --ct-cx / --ct-cy  centre of the hexagon under the pointer (cell highlight)
//   --ct-pointer       1 while a mouse or pen is over the page, 0 otherwise
// Presentation only: passive listeners, at most one write per animation frame,
// nothing for touch, and no work at all while the theme attribute is absent.

/** Hexagon side in CSS pixels; must match the 72 × 41.569px tile in theme.css. */
export const HEX_SIDE = 24
/** Distance between two hexagon centres in the same column. */
export const HEX_ROW = HEX_SIDE * Math.sqrt(3)

const COLUMN_PITCH = HEX_SIDE * 1.5
const FIRST_COLUMN_X = HEX_SIDE / 2
const LAYER_CLASSES = ['bg-mesh-gradient', 'auth-layout-decor', 'home-backdrop'] as const

export interface Point {
  x: number
  y: number
}

/**
 * Centre of the hexagon that contains (x, y), in layer coordinates.
 *
 * The grid is anchored at the layer's top-left corner: column c is centred at
 * x = 12 + 36c, and odd columns sit half a row lower. Hexagons are the Voronoi
 * cells of their centres, so the nearest centre among the two candidate columns
 * is the containing cell.
 */
export function hexCellAt(x: number, y: number): Point {
  const firstColumn = Math.floor((x - FIRST_COLUMN_X) / COLUMN_PITCH)
  let best: Point = { x: 0, y: 0 }
  let bestDistance = Number.POSITIVE_INFINITY
  for (let column = firstColumn; column <= firstColumn + 1; column += 1) {
    const centreX = FIRST_COLUMN_X + column * COLUMN_PITCH
    const shift = Math.abs(column % 2) === 1 ? HEX_ROW / 2 : 0
    const centreY = Math.round((y - shift) / HEX_ROW) * HEX_ROW + shift
    const distance = (x - centreX) ** 2 + (y - centreY) ** 2
    if (distance < bestDistance) {
      bestDistance = distance
      best = { x: centreX, y: centreY }
    }
  }
  return best
}

/**
 * Starts tracking the pointer for the backdrop layers. `isActive` is checked on
 * every frame, so the listeners can stay installed while the theme is switched
 * off and on. Returns a function that removes everything again.
 */
export function installPointerField(isActive: () => boolean): () => void {
  if (typeof window === 'undefined' || typeof document === 'undefined') return () => {}
  if (typeof window.requestAnimationFrame !== 'function') return () => {}

  const layers = LAYER_CLASSES.map(name => document.getElementsByClassName(name))
  let frame = 0
  let pointerX = 0
  let pointerY = 0
  let present = false

  const paint = () => {
    frame = 0
    if (!isActive()) return
    for (const collection of layers) {
      for (const element of Array.from(collection)) {
        const layer = element as HTMLElement
        if (!present) {
          layer.style.setProperty('--ct-pointer', '0')
          continue
        }
        const rect = layer.getBoundingClientRect()
        if (rect.width === 0 && rect.height === 0) continue
        const x = pointerX - rect.left
        const y = pointerY - rect.top
        const cell = hexCellAt(x, y)
        layer.style.setProperty('--ct-px', `${x.toFixed(1)}px`)
        layer.style.setProperty('--ct-py', `${y.toFixed(1)}px`)
        layer.style.setProperty('--ct-cx', `${cell.x.toFixed(2)}px`)
        layer.style.setProperty('--ct-cy', `${cell.y.toFixed(2)}px`)
        layer.style.setProperty('--ct-pointer', '1')
      }
    }
  }

  const schedule = () => {
    if (!frame) frame = window.requestAnimationFrame(paint)
  }

  const onPointerMove = (event: PointerEvent) => {
    if (!isActive() || event.pointerType === 'touch') return
    pointerX = event.clientX
    pointerY = event.clientY
    present = true
    schedule()
  }

  const onPointerGone = () => {
    if (!present) return
    present = false
    schedule()
  }

  const onScroll = () => {
    // Fixed layers do not move, but the auth backdrop scrolls with its page.
    if (present) schedule()
  }

  const root = document.documentElement
  window.addEventListener('pointermove', onPointerMove, { passive: true })
  window.addEventListener('scroll', onScroll, { passive: true })
  window.addEventListener('blur', onPointerGone)
  root.addEventListener('mouseleave', onPointerGone)

  return () => {
    window.removeEventListener('pointermove', onPointerMove)
    window.removeEventListener('scroll', onScroll)
    window.removeEventListener('blur', onPointerGone)
    root.removeEventListener('mouseleave', onPointerGone)
    if (frame) window.cancelAnimationFrame(frame)
    frame = 0
    for (const collection of layers) {
      for (const element of Array.from(collection)) {
        const style = (element as HTMLElement).style
        for (const variable of ['--ct-px', '--ct-py', '--ct-cx', '--ct-cy', '--ct-pointer']) {
          style.removeProperty(variable)
        }
      }
    }
  }
}
