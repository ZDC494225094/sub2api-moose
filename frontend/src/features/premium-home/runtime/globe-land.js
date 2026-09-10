import * as THREE from 'three'
import coordinates from './globe-land-points.json'

/** Static geographic points: first render and later frames use exactly the same coastline. */
export function createLandDots(radius = 2.002) {
  const geometry = new THREE.CircleGeometry(0.011, 6)
  const material = new THREE.MeshBasicMaterial({ color: 0x71717a, transparent: true, opacity: 0.82 })
  const mesh = new THREE.InstancedMesh(geometry, material, coordinates.length / 2)
  const dummy = new THREE.Object3D()
  const normal = new THREE.Vector3()
  const target = new THREE.Vector3()
  for (let i = 0; i < coordinates.length; i += 2) {
    const lat = coordinates[i] / 1000 * Math.PI / 180
    const lon = coordinates[i + 1] / 1000 * Math.PI / 180
    normal.set(Math.cos(lat) * Math.sin(lon), Math.sin(lat), Math.cos(lat) * Math.cos(lon))
    dummy.position.copy(normal).multiplyScalar(radius)
    dummy.lookAt(target.copy(dummy.position).add(normal))
    dummy.updateMatrix()
    mesh.setMatrixAt(i / 2, dummy.matrix)
  }
  mesh.instanceMatrix.needsUpdate = true
  mesh.computeBoundingSphere()
  return mesh
}
