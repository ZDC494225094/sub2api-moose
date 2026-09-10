import { describe, expect, it } from 'vitest'
import { Vector3 } from 'three'
import { createGlobeDecalGeometry } from '../globe-decal-geometry'

describe('curved globe logo geometry', () => {
  it.each([[31.2304, 121.4737], [-33.8688, 151.2093], [89, -179]])('keeps the entire decal on the sphere at %s, %s', (lat, lon) => {
    const geometry = createGlobeDecalGeometry(lat, lon)
    const positions = geometry.getAttribute('position')
    const point = new Vector3()
    for (let i = 0; i < positions.count; i++) {
      point.fromBufferAttribute(positions, i)
      expect(point.length()).toBeCloseTo(2.006, 6)
    }
    geometry.dispose()
  })

  it('has outward faces so logos are naturally culled on the far side', () => {
    const geometry = createGlobeDecalGeometry(31.2304, 121.4737)
    const positions = geometry.getAttribute('position')
    const indices = geometry.index.array
    for (let i = 0; i < indices.length; i += 3) {
      const a = new Vector3().fromBufferAttribute(positions, indices[i])
      const b = new Vector3().fromBufferAttribute(positions, indices[i + 1])
      const c = new Vector3().fromBufferAttribute(positions, indices[i + 2])
      expect(b.sub(a).cross(c.sub(a)).dot(a)).toBeGreaterThan(0)
    }
    geometry.dispose()
  })

  it('maps the logo upright with curved corners instead of a tangent plane', () => {
    const geometry = createGlobeDecalGeometry(0, 0)
    const positions = geometry.getAttribute('position')
    const uvs = geometry.getAttribute('uv')
    expect(uvs.getX(0)).toBe(0)
    expect(uvs.getY(0)).toBe(0)
    expect(uvs.getY(positions.count - 1)).toBe(1)
    expect(positions.getY(positions.count - 1)).toBeGreaterThan(positions.getY(0))
    expect(positions.getZ(0)).toBeLessThan(positions.getZ(220))
    geometry.dispose()
  })
})
