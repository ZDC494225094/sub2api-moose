const THREE_CDN_URL = 'https://unpkg.com/three@0.160.0/build/three.module.js'
const EARTH_TEXTURE_URL = 'https://unpkg.com/three-globe/example/img/earth-water.png'

const globeHub = { lat: 31.2304, lon: 121.4737 }

const globeRoutes = [
  { start: { lat: 40.7128, lon: -74.006 }, end: globeHub, width: 1, speed: 0.48 },
  { start: { lat: -33.8688, lon: 151.2093 }, end: globeHub, width: 0.96, speed: 0.44 },
  { start: { lat: 25.2048, lon: 55.2708 }, end: globeHub, width: 0.88, speed: 0.52 },
  { start: { lat: 37.7749, lon: -122.4194 }, end: globeHub, width: 0.92, speed: 0.46 },
  { start: { lat: 48.8566, lon: 2.3522 }, end: globeHub, width: 0.82, speed: 0.42 },
  { start: { lat: -33.9249, lon: 18.4241 }, end: globeHub, width: 0.9, speed: 0.4 },
]

let threePromise = null

function loadThree() {
  if (!threePromise) {
    threePromise = import(/* @vite-ignore */ THREE_CDN_URL)
  }
  return threePromise
}

function latLonToVector3(THREE, lat, lon, radius) {
  const phi = ((90 - lat) * Math.PI) / 180
  const theta = (lon * Math.PI) / 180
  const x = radius * Math.sin(phi) * Math.sin(theta)
  const z = radius * Math.sin(phi) * Math.cos(theta)
  const y = radius * Math.cos(phi)
  return new THREE.Vector3(x, y, z)
}

function approximateLandMask(lat, lon) {
  const deltaLon = (a, b) => Math.abs((((a - b + 540) % 360) - 180))
  const ellipse = (centerLat, centerLon, radiusLat, radiusLon) => {
    const y = (lat - centerLat) / radiusLat
    const x = deltaLon(lon, centerLon) / radiusLon
    return x * x + y * y <= 1
  }

  return (
    ellipse(49, -101, 27, 56) ||
    ellipse(61, -150, 12, 28) ||
    ellipse(17, -92, 10, 30) ||
    ellipse(-18, -61, 36, 18) ||
    ellipse(51, 16, 17, 30) ||
    ellipse(8, 22, 39, 25) ||
    ellipse(25, 45, 15, 25) ||
    ellipse(45, 91, 28, 72) ||
    ellipse(8, 111, 18, 31) ||
    ellipse(20, 78, 14, 11) ||
    ellipse(-25, 135, 15, 23) ||
    ellipse(37, 139, 8, 8) ||
    ellipse(-20, 47, 11, 5)
  )
}

function createFallbackDots(THREE, radius) {
  const positions = []
  const step = 1.55

  for (let lat = -58; lat <= 72; lat += step) {
    for (let lon = -180; lon <= 180; lon += step) {
      const noise = Math.sin((lat * 12.9898 + lon * 78.233) * 43758.5453)
      const keep = noise - Math.floor(noise)
      if (!approximateLandMask(lat, lon) || keep < 0.18) continue

      const point = latLonToVector3(THREE, lat + (keep - 0.5) * 0.42, lon + (0.5 - keep) * 0.42, radius)
      positions.push(point.x, point.y, point.z)
    }
  }

  const geometry = new THREE.BufferGeometry()
  geometry.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3))
  const material = new THREE.PointsMaterial({
    color: 0x71717a,
    size: 0.031,
    transparent: true,
    opacity: 0.8,
    sizeAttenuation: true,
    depthWrite: false,
  })

  return new THREE.Points(geometry, material)
}

async function createTextureDots(THREE) {
  const texture = await new THREE.TextureLoader().loadAsync(EARTH_TEXTURE_URL)
  const numPoints = 42000
  const radius = 2.002
  const dummy = new THREE.Object3D()
  const normal = new THREE.Vector3()
  const geometry = new THREE.CircleGeometry(0.011, 6)
  const material = new THREE.MeshBasicMaterial({
    color: 0x71717a,
    transparent: true,
    opacity: 0.82,
  })
  const mesh = new THREE.InstancedMesh(geometry, material, numPoints)
  const phi = Math.PI * (3 - Math.sqrt(5))

  for (let i = 0; i < numPoints; i += 1) {
    const y = 1 - (i / (numPoints - 1)) * 2
    const rAtY = Math.sqrt(1 - y * y)
    const theta = phi * i
    const x = Math.cos(theta) * rAtY
    const z = Math.sin(theta) * rAtY

    dummy.position.set(x * radius, y * radius, z * radius)
    normal.set(x, y, z).normalize()
    dummy.lookAt(dummy.position.clone().add(normal))
    dummy.updateMatrix()
    mesh.setMatrixAt(i, dummy.matrix)
  }

  mesh.instanceMatrix.needsUpdate = true
  mesh.frustumCulled = false

  material.onBeforeCompile = (shader) => {
    shader.uniforms.uMap = { value: texture }
    shader.vertexShader = `
      uniform sampler2D uMap;
      ${shader.vertexShader}
    `.replace(
      '#include <project_vertex>',
      `
      vec3 instPos = vec3(instanceMatrix[3][0], instanceMatrix[3][1], instanceMatrix[3][2]);
      vec3 instPosNorm = normalize(instPos);
      float u = 0.5 + atan(instPosNorm.x, instPosNorm.z) / (2.0 * 3.14159265);
      float v = 0.5 + asin(instPosNorm.y) / 3.14159265;
      vec4 texColor = texture2D(uMap, vec2(u, v));
      if (texColor.r > 0.2) {
        transformed *= 0.0;
      }
      #include <project_vertex>
      `,
    )
  }
  material.customProgramCacheKey = () => 'premium-home-globe-dots-v1'
  material.needsUpdate = true

  return mesh
}

async function createEarthDots(THREE) {
  try {
    return await createTextureDots(THREE)
  } catch {
    return createFallbackDots(THREE, 2.012)
  }
}

function createNode(THREE, position, scale = 1, color = 0x3b82f6, radius = 0.03) {
  const geometry = new THREE.SphereGeometry(radius, 16, 16)
  const material = new THREE.MeshBasicMaterial({
    color,
    transparent: true,
    opacity: 0.96,
    blending: THREE.AdditiveBlending,
  })
  const mesh = new THREE.Mesh(geometry, material)
  mesh.position.copy(position)
  mesh.scale.setScalar(scale)
  return mesh
}

function orientFlatMesh(mesh, position, THREE) {
  mesh.quaternion.setFromUnitVectors(new THREE.Vector3(0, 0, 1), position.clone().normalize())
}

function createPulseRing(THREE, position, baseScale, opacity) {
  const geometry = new THREE.RingGeometry(0.052, 0.074, 64)
  const material = new THREE.MeshBasicMaterial({
    color: 0x46b8ff,
    transparent: true,
    opacity,
    side: THREE.DoubleSide,
    depthWrite: false,
    blending: THREE.AdditiveBlending,
  })
  const mesh = new THREE.Mesh(geometry, material)
  mesh.position.copy(position)
  mesh.scale.setScalar(baseScale)
  orientFlatMesh(mesh, position, THREE)
  return mesh
}

function buildArcSegments(THREE, route) {
  const radius = 2.002
  const start = latLonToVector3(THREE, route.start.lat, route.start.lon, radius)
  const end = latLonToVector3(THREE, route.end.lat, route.end.lon, radius)
  const totalAngle = start.angleTo(end)
  let axis = start.clone().cross(end).normalize()

  if (axis.lengthSq() < 0.0001) {
    axis = start.clone().cross(new THREE.Vector3(0, 1, 0)).normalize()
    if (axis.lengthSq() < 0.0001) {
      axis = start.clone().cross(new THREE.Vector3(1, 0, 0)).normalize()
    }
  }

  const segments = []
  const segmentCount = 1
  const segmentAngle = totalAngle / segmentCount

  for (let i = 0; i < segmentCount; i += 1) {
    const segmentStart = start.clone().applyAxisAngle(axis, i * segmentAngle)
    const segmentEnd = start.clone().applyAxisAngle(axis, (i + 1) * segmentAngle)
    const points = []
    const pointCount = 20

    for (let j = 0; j <= pointCount; j += 1) {
      const t = j / pointCount
      const point = segmentStart.clone().applyAxisAngle(axis, segmentAngle * t)
      const altitudeCurve = 1 - Math.pow(2 * t - 1, 2)
      const altitude = radius + segmentAngle * 0.15 * altitudeCurve + totalAngle * 0.035
      point.normalize().multiplyScalar(altitude)
      points.push(point)
    }

    segments.push({
      curve: new THREE.CatmullRomCurve3(points),
      endPoint: segmentEnd,
    })
  }

  return { start, end, segments }
}

function createFlowSegment(THREE, curve, width, speed, initialOffset) {
  const geometry = new THREE.TubeGeometry(curve, 44, 0.012 * width, 8, false)
  const uniforms = {
    time: { value: initialOffset },
    color: { value: new THREE.Color('#22d3ee') },
  }

  const material = new THREE.ShaderMaterial({
    transparent: true,
    depthWrite: false,
    blending: THREE.AdditiveBlending,
    uniforms,
    vertexShader: `
      varying vec2 vUv;
      void main() {
        vUv = uv;
        gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
      }
    `,
    fragmentShader: `
      uniform float time;
      uniform vec3 color;
      varying vec2 vUv;

      void main() {
        float head = fract(vUv.x * 2.0 + time);
        float mask = smoothstep(0.0, 0.38, head) * (1.0 - smoothstep(0.38, 0.44, head));
        gl_FragColor = vec4(color, mask * 0.92);
      }
    `,
  })

  return {
    mesh: new THREE.Mesh(geometry, material),
    uniforms,
    speed,
  }
}

function createArcGroup(THREE, route, animatedFlows) {
  const { segments } = buildArcSegments(THREE, route)
  const group = new THREE.Group()
  const initialOffset = Math.random()

  segments.forEach((segment) => {
    const baseMesh = new THREE.Mesh(
      new THREE.TubeGeometry(segment.curve, 44, 0.008 * (route.width || 1), 8, false),
      new THREE.MeshBasicMaterial({
        color: 0xa855f7,
        transparent: true,
        opacity: 0.52,
        depthWrite: false,
      }),
    )
    const flow = createFlowSegment(THREE, segment.curve, route.width || 1, route.speed || 0.45, initialOffset)

    group.add(baseMesh)
    group.add(flow.mesh)
    animatedFlows.push(flow)
  })

  return group
}

function collectUniqueNodes() {
  const nodes = [{ ...globeHub, scale: 1.2, color: 0x3b82f6 }]
  const seen = new Set([`${globeHub.lat}:${globeHub.lon}`])

  globeRoutes.forEach((route) => {
    const key = `${route.start.lat}:${route.start.lon}`
    if (seen.has(key)) return
    seen.add(key)
    nodes.push({
      lat: route.start.lat,
      lon: route.start.lon,
      scale: 1,
      color: 0x93c5fd,
    })
  })

  return nodes
}

export async function mountPremiumHomeGlobe(canvas, options = {}) {
  if (!canvas || typeof window === 'undefined') {
    return () => {}
  }

  const parent = canvas.parentElement || canvas
  const THREE = await loadThree()

  if (!canvas.isConnected) {
    return () => {}
  }

  const renderer = new THREE.WebGLRenderer({
    canvas,
    alpha: true,
    antialias: true,
    powerPreference: 'high-performance',
  })
  renderer.setClearColor(0x000000, 0)
  renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, options.maxPixelRatio || 2))

  const scene = new THREE.Scene()
  scene.fog = new THREE.Fog(0xf3f3f3, 6.8, 18)

  const camera = new THREE.PerspectiveCamera(42, 1, 0.1, 100)
  camera.position.set(0, 0, 7)

  scene.add(new THREE.AmbientLight(0xffffff, 3.4))

  const primaryLight = new THREE.DirectionalLight(0xffffff, 2.4)
  primaryLight.position.set(0, 5, 10)
  scene.add(primaryLight)

  const secondaryLight = new THREE.DirectionalLight(0xf8fafc, 1.4)
  secondaryLight.position.set(-10, -5, 5)
  scene.add(secondaryLight)

  const group = new THREE.Group()
  group.rotation.set(0.2, 1.02, 0)
  group.scale.setScalar(1)
  scene.add(group)

  const globeShell = new THREE.Mesh(
    new THREE.SphereGeometry(2, 64, 64),
    new THREE.MeshBasicMaterial({
      color: 0xf3f3f3,
    }),
  )
  group.add(globeShell)

  group.add(await createEarthDots(THREE))

  const animatedFlows = []
  globeRoutes.forEach((route) => {
    group.add(createArcGroup(THREE, route, animatedFlows))
  })

  const nodeGroup = new THREE.Group()
  collectUniqueNodes().forEach((node) => {
    const position = latLonToVector3(THREE, node.lat, node.lon, 2.03)
    nodeGroup.add(createNode(THREE, position, node.scale, node.color, node.lat === globeHub.lat ? 0.032 : 0.022))
  })
  group.add(nodeGroup)

  const pulseState = []
  const hubPosition = latLonToVector3(THREE, globeHub.lat, globeHub.lon, 2.035)
  const hubGlow = new THREE.Mesh(
    new THREE.SphereGeometry(0.12, 24, 24),
    new THREE.MeshBasicMaterial({
      color: 0x38bdf8,
      transparent: true,
      opacity: 0.22,
      blending: THREE.AdditiveBlending,
      depthWrite: false,
    }),
  )
  hubGlow.position.copy(hubPosition)
  group.add(hubGlow)

  const hubRingA = createPulseRing(THREE, hubPosition, 1, 0.28)
  const hubRingB = createPulseRing(THREE, hubPosition, 1, 0.18)
  group.add(hubRingA)
  group.add(hubRingB)
  pulseState.push({ mesh: hubRingA, phase: 0, baseScale: 1 })
  pulseState.push({ mesh: hubRingB, phase: 0.45, baseScale: 0.82 })

  const maxSize = options.maxSize || 620
  const resize = () => {
    const rect = parent.getBoundingClientRect()
    const size = Math.floor(Math.min(rect.width || 0, rect.height || rect.width || 0, maxSize))
    if (size < 8) return

    camera.aspect = 1
    camera.updateProjectionMatrix()
    renderer.setSize(size, size, false)
    canvas.style.width = `${size}px`
    canvas.style.height = `${size}px`
  }

  resize()
  const resizeObserver = new ResizeObserver(resize)
  resizeObserver.observe(parent)

  const clock = new THREE.Clock()
  let animationFrame = 0
  let disposed = false
  let dragging = false
  let activePointerId = null
  let lastPointerX = 0
  let lastPointerY = 0
  let rotationX = group.rotation.x
  let rotationY = group.rotation.y
  let inertialVelocityX = 0
  let inertialVelocityY = 0

  canvas.style.touchAction = 'none'
  canvas.style.cursor = 'grab'

  const clampRotationX = (value) => Math.max(-0.45, Math.min(0.55, value))

  const onPointerDown = (event) => {
    event.preventDefault()
    dragging = true
    activePointerId = event.pointerId
    lastPointerX = event.clientX
    lastPointerY = event.clientY
    inertialVelocityX = 0
    inertialVelocityY = 0
    canvas.style.cursor = 'grabbing'
    canvas.setPointerCapture?.(event.pointerId)
  }

  const onPointerMove = (event) => {
    if (!dragging || event.pointerId !== activePointerId) return
    event.preventDefault()

    const deltaX = event.clientX - lastPointerX
    const deltaY = event.clientY - lastPointerY
    lastPointerX = event.clientX
    lastPointerY = event.clientY

    rotationY += deltaX * 0.0095
    rotationX = clampRotationX(rotationX + deltaY * 0.0065)
    inertialVelocityY = deltaX * 0.00085
    inertialVelocityX = deltaY * 0.00055

    group.rotation.x = rotationX
    group.rotation.y = rotationY
  }

  const endPointerInteraction = (event) => {
    if (activePointerId !== null && event.pointerId !== undefined && event.pointerId !== activePointerId) return
    dragging = false
    activePointerId = null
    canvas.style.cursor = 'grab'
    canvas.releasePointerCapture?.(event.pointerId)
  }

  canvas.addEventListener('pointerdown', onPointerDown)
  canvas.addEventListener('pointermove', onPointerMove)
  canvas.addEventListener('pointerup', endPointerInteraction)
  canvas.addEventListener('pointercancel', endPointerInteraction)
  canvas.addEventListener('lostpointercapture', endPointerInteraction)

  const tick = () => {
    if (disposed) return

    const delta = clock.getDelta()
    const elapsed = clock.elapsedTime

    if (!dragging) {
      rotationY += delta * 0.05 + inertialVelocityY
      rotationX = clampRotationX(rotationX + inertialVelocityX)
      inertialVelocityY *= 0.94
      inertialVelocityX *= 0.9
      group.rotation.x = rotationX
      group.rotation.y = rotationY
    }
    animatedFlows.forEach((flow) => {
      flow.uniforms.time.value -= delta * flow.speed * 2
    })
    pulseState.forEach((pulse) => {
      const cycle = (elapsed * 0.55 + pulse.phase) % 1
      const scale = pulse.baseScale * (1 + cycle * 1.25)
      pulse.mesh.scale.setScalar(scale)
      pulse.mesh.material.opacity = Math.max(0, 0.28 * (1 - cycle))
    })

    renderer.render(scene, camera)
    animationFrame = window.requestAnimationFrame(tick)
  }

  tick()

  return () => {
    if (disposed) return
    disposed = true

    if (animationFrame) {
      window.cancelAnimationFrame(animationFrame)
    }

    canvas.removeEventListener('pointerdown', onPointerDown)
    canvas.removeEventListener('pointermove', onPointerMove)
    canvas.removeEventListener('pointerup', endPointerInteraction)
    canvas.removeEventListener('pointercancel', endPointerInteraction)
    canvas.removeEventListener('lostpointercapture', endPointerInteraction)
    canvas.style.cursor = ''
    canvas.style.touchAction = ''
    resizeObserver.disconnect()
    group.traverse((item) => {
      if (item.geometry) {
        item.geometry.dispose?.()
      }
      if (Array.isArray(item.material)) {
        item.material.forEach((material) => material?.dispose?.())
      } else {
        item.material?.dispose?.()
      }
    })

    renderer.dispose()
    renderer.forceContextLoss?.()
  }
}
