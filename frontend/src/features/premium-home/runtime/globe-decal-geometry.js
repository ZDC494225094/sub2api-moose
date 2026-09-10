import * as THREE from 'three'

// Project a subdivided UV square onto the sphere, rather than using a flat billboard.
export function createGlobeDecalGeometry(lat, lon, halfAngle = 0.11, radius = 2.006, segments = 20) {
  const latitude = THREE.MathUtils.degToRad(lat)
  const longitude = THREE.MathUtils.degToRad(lon)
  const normal = new THREE.Vector3(Math.cos(latitude) * Math.sin(longitude), Math.sin(latitude), Math.cos(latitude) * Math.cos(longitude))
  const east = new THREE.Vector3(Math.cos(longitude), 0, -Math.sin(longitude))
  const north = new THREE.Vector3().crossVectors(normal, east)
  const extent = Math.tan(halfAngle)
  const positions = [], normals = [], uvs = [], indices = []
  const point = new THREE.Vector3()
  for (let y = 0; y <= segments; y++) {
    for (let x = 0; x <= segments; x++) {
      const u = x / segments, v = y / segments
      point.copy(normal).addScaledVector(east, (u * 2 - 1) * extent).addScaledVector(north, (v * 2 - 1) * extent).normalize()
      normals.push(point.x, point.y, point.z)
      positions.push(point.x * radius, point.y * radius, point.z * radius)
      uvs.push(u, v)
      if (x < segments && y < segments) {
        const a = y * (segments + 1) + x, b = a + segments + 1
        indices.push(a, a + 1, b, a + 1, b + 1, b)
      }
    }
  }
  const geometry = new THREE.BufferGeometry()
  geometry.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3))
  geometry.setAttribute('normal', new THREE.Float32BufferAttribute(normals, 3))
  geometry.setAttribute('uv', new THREE.Float32BufferAttribute(uvs, 2))
  geometry.setIndex(indices)
  return geometry
}
