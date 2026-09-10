import { describe, expect, it, vi } from 'vitest'
import { Matrix4, Vector3 } from 'three'
import { createLandDots } from '../globe-land'
import coordinates from '../globe-land-points.json'

function nearestDegrees(lat, lon) {
  let nearest = Infinity
  for (let i = 0; i < coordinates.length; i += 2) {
    const dlat = coordinates[i] / 1000 - lat
    const dlon = (((coordinates[i + 1] / 1000 - lon + 540) % 360) - 180) * Math.cos(lat * Math.PI / 180)
    nearest = Math.min(nearest, Math.hypot(dlat, dlon))
  }
  return nearest
}

describe('bundled globe land', () => {
  it('covers the major land masses, including Greenland and Antarctica', () => {
    for (const [lat, lon] of [[40, -100], [-15, -60], [5, 20], [50, 90], [-25, 135], [72, -40], [-80, 0]]) {
      expect(nearestDegrees(lat, lon)).toBeLessThan(1.2)
    }
  })

  it('leaves open ocean empty instead of drawing approximate continent ellipses', () => {
    for (const [lat, lon] of [[0, -140], [0, -30], [-35, 80]]) {
      expect(nearestDegrees(lat, lon)).toBeGreaterThan(5)
    }
  })

  it('builds surface-facing dots synchronously without fetching or loading images', () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('offline'))
    const imageSpy = vi.spyOn(document, 'createElementNS')
    const mesh = createLandDots()
    try {
      expect(mesh.count).toBeGreaterThan(10000)
      expect(mesh.count).toBeLessThan(15000)
      expect(fetchSpy).not.toHaveBeenCalled()
      expect(imageSpy).not.toHaveBeenCalled()
      expect(mesh.material.map).toBeNull()
      const matrix = new Matrix4()
      for (let i = 0; i < mesh.count; i += 101) {
        mesh.getMatrixAt(i, matrix)
        const position = new Vector3().setFromMatrixPosition(matrix)
        expect(position.length()).toBeCloseTo(2.002, 5)
        const facing = new Vector3(0, 0, 1).transformDirection(matrix)
        expect(facing.dot(position.normalize())).toBeCloseTo(1, 5)
      }
    } finally {
      mesh.geometry.dispose()
      mesh.material.dispose()
      fetchSpy.mockRestore()
      imageSpy.mockRestore()
    }
  })
})
