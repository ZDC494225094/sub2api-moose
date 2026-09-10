import * as THREE from 'three'
import { createLandDots } from './globe-land'
import { createGlobeDecalGeometry } from './globe-decal-geometry'

const globeHub = { lat: 31.2304, lon: 121.4737 }

const globeRoutes = [
  { start: { lat: 40.7128, lon: -74.006 }, end: globeHub, width: 1, speed: 0.48 },
  { start: { lat: -33.8688, lon: 151.2093 }, end: globeHub, width: 0.96, speed: 0.44 },
  { start: { lat: 25.2048, lon: 55.2708 }, end: globeHub, width: 0.88, speed: 0.52 },
  { start: { lat: 37.7749, lon: -122.4194 }, end: globeHub, width: 0.92, speed: 0.46 },
  { start: { lat: 48.8566, lon: 2.3522 }, end: globeHub, width: 0.82, speed: 0.42 },
  { start: { lat: -33.9249, lon: 18.4241 }, end: globeHub, width: 0.9, speed: 0.4 },
]

// These are schematic network nodes, not provider office locations.
export const globeProviderMarkers = [
  { id: 'openai', name: 'OpenAI', model: 'gpt' },
  { id: 'gemini', name: 'Gemini', model: 'gemini' },
  { id: 'deepseek', name: 'DeepSeek', model: 'deepseek' },
  { id: 'claude', name: 'Claude', model: 'claude' },
  { id: 'qwen', name: 'Qwen', model: 'qwen' },
  { id: 'grok', name: 'Grok', model: 'grok' },
]

function latLonToVector3(THREE, lat, lon, radius) {
  const phi = ((90 - lat) * Math.PI) / 180
  const theta = (lon * Math.PI) / 180
  const x = radius * Math.sin(phi) * Math.sin(theta)
  const z = radius * Math.sin(phi) * Math.cos(theta)
  const y = radius * Math.cos(phi)
  return new THREE.Vector3(x, y, z)
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
      const altitude = radius + (segmentAngle * 0.15 + totalAngle * 0.035) * altitudeCurve
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
  const noop = Object.assign(() => {}, { setTheme: () => {}, setAnimating: () => {}, setSiteLogo: () => {} })
  if (!canvas || typeof window === 'undefined') {
    return noop
  }

  const parent = canvas.parentElement || canvas

  if (!canvas.isConnected) {
    return noop
  }

  const renderer = new THREE.WebGLRenderer({
    canvas,
    alpha: true,
    antialias: true,
    // Keep the completed transparent frame while CSS moves/scales the canvas.
    preserveDrawingBuffer: true,
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
  group.rotation.set(0.2, -1.7, 0)
  group.scale.setScalar(1)
  scene.add(group)

  const introStartedAt = performance.now()
  const introEnabled = (options.animate ?? !window.matchMedia('(prefers-reduced-motion: reduce)').matches) && window.scrollY < 100

  const globeShell = new THREE.Mesh(
    new THREE.SphereGeometry(2, 64, 64),
    new THREE.ShaderMaterial({
      transparent: true,
      uniforms: {
        centerColor: { value: new THREE.Color('#fdfdfe') },
        edgeColor: { value: new THREE.Color('#eaebed') },
        emergence: { value: introEnabled ? 0 : 1 },
      },
      vertexShader: `
        varying vec3 vNormal;
        varying vec3 vViewPosition;

        void main() {
          vec4 mvPosition = modelViewMatrix * vec4(position, 1.0);
          vNormal = normalize(normalMatrix * normal);
          vViewPosition = -mvPosition.xyz;
          gl_Position = projectionMatrix * mvPosition;
        }
      `,
      fragmentShader: `
        uniform vec3 centerColor;
        uniform vec3 edgeColor;
        uniform float emergence;
        varying vec3 vNormal;
        varying vec3 vViewPosition;

        void main() {
          float facing = max(dot(normalize(vNormal), normalize(vViewPosition)), 0.0);
          float centerWeight = smoothstep(0.1, 1.0, pow(facing, 0.72));
          vec3 color = mix(edgeColor, centerColor, centerWeight);
          gl_FragColor = vec4(color, emergence);
          #include <colorspace_fragment>
        }
      `,
    }),
  )
  globeShell.renderOrder = -1
  group.add(globeShell)

  // Bundled land coordinates are available on the very first render.
  const earthDots = createLandDots()
  group.add(earthDots)

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

  const render = () => {
    const progress = introEnabled ? Math.min(1, Math.max(0, (performance.now() - introStartedAt - 300) / 1500)) : 1
    globeShell.material.uniforms.emergence.value = progress * progress * (3 - 2 * progress)
    renderer.render(scene, camera)
  }
  let disposed = false
  let dark = Boolean(options.isDark)
  const decals = new Map()
  let siteLogoRevision = 0

  const loadLogoTexture = async (url, circular = false) => {
    const image = await new THREE.ImageLoader().loadAsync(url)
    const source = document.createElement('canvas')
    source.width = source.height = 256
    const context = source.getContext('2d')
    if (!context) throw new Error('Logo texture canvas is unavailable')
    if (circular) {
      context.beginPath()
      context.arc(128, 128, 120, 0, Math.PI * 2)
      context.clip()
    }
    const scale = 240 / Math.max(image.width, image.height)
    const width = image.width * scale, height = image.height * scale
    context.drawImage(image, (256 - width) / 2, (256 - height) / 2, width, height)
    const texture = new THREE.CanvasTexture(source)
    texture.colorSpace = THREE.SRGBColorSpace
    texture.anisotropy = Math.min(8, renderer.capabilities.getMaxAnisotropy())
    texture.minFilter = THREE.LinearMipmapLinearFilter
    return texture
  }

  const installDecal = (id, position, texture, monochrome = false) => {
    const previous = decals.get(id)
    if (previous) {
      group.remove(previous)
      previous.geometry.dispose()
      previous.material.dispose()
      previous.userData.mapTexture.dispose()
    }
    const material = new THREE.ShaderMaterial({
      uniforms: {
        map: { value: texture },
        monochrome: { value: monochrome },
        ink: { value: new THREE.Color(dark ? '#edf6ff' : '#162233') },
      },
      transparent: true,
      depthTest: true,
      depthWrite: false,
      // A tiny depth bias avoids coplanar flicker without lifting the logo above the surface.
      polygonOffset: true,
      polygonOffsetFactor: -1,
      polygonOffsetUnits: -1,
      vertexShader: `
        varying vec2 vUv;
        void main() {
          vUv = uv;
          gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
        }
      `,
      fragmentShader: `
        uniform sampler2D map;
        uniform bool monochrome;
        uniform vec3 ink;
        varying vec2 vUv;
        void main() {
          vec4 texel = texture2D(map, vUv);
          if (texel.a < 0.01) discard;
          gl_FragColor = vec4(monochrome ? ink : texel.rgb, texel.a);
          #include <colorspace_fragment>
        }
      `,
    })
    const mesh = new THREE.Mesh(createGlobeDecalGeometry(position.lat, position.lon, id === 'site' ? 0.078 : 0.063), material)
    mesh.userData.mapTexture = texture
    mesh.renderOrder = 2
    decals.set(id, mesh)
    group.add(mesh)
    const nodeIndex = id === 'site' ? 0 : globeProviderMarkers.findIndex((marker) => marker.id === id) + 1
    if (nodeGroup.children[nodeIndex]) nodeGroup.children[nodeIndex].visible = false
    render()
  }

  globeRoutes.forEach((route, index) => {
    const { id } = globeProviderMarkers[index]
    const url = options.providerLogos?.[id]
    if (!url) return
    void loadLogoTexture(url).then((texture) => {
      if (disposed) { texture.dispose(); return }
      installDecal(id, route.start, texture, id === 'openai' || id === 'grok')
    }).catch((error) => console.warn('Globe provider logo unavailable:', id, error))
  })

  const setSiteLogo = (url) => {
    if (disposed) return
    const revision = ++siteLogoRevision
    const source = url || '/logo.svg'
    void loadLogoTexture(source, true).catch((error) => {
      if (source === '/logo.svg') throw error
      console.warn('Globe site logo fallback:', error)
      return loadLogoTexture('/logo.svg', true)
    }).then((texture) => {
      if (disposed || revision !== siteLogoRevision) { texture.dispose(); return }
      installDecal('site', globeHub, texture)
    }).catch((error) => console.warn('Globe site logo unavailable:', error))
  }

  const setTheme = (isDark) => {
    dark = isDark
    globeShell.material.uniforms.centerColor.value.set(dark ? '#142b49' : '#fdfdfe')
    globeShell.material.uniforms.edgeColor.value.set(dark ? '#071220' : '#dfe8f5')
    earthDots.material.color.set(dark ? '#72a8d7' : '#6d829d')
    earthDots.material.opacity = dark ? 0.92 : 0.82
    scene.fog.color.set(dark ? '#0b1220' : '#f3f6fc')
    animatedFlows.forEach((flow) => flow.uniforms.color.value.set(dark ? '#67e8f9' : '#238af5'))
    decals.forEach((mesh) => mesh.material.uniforms.ink.value.set(dark ? '#edf6ff' : '#162233'))
    render()
  }
  setTheme(dark)

  const maxSize = options.maxSize || 620
  let renderedSize = 0
  const resize = () => {
    const rect = parent.getBoundingClientRect()
    const size = Math.floor(Math.min(rect.width || 0, rect.height || rect.width || 0, maxSize))
    if (size < 8 || size === renderedSize) return
    renderedSize = size

    camera.aspect = 1
    camera.updateProjectionMatrix()
    renderer.setSize(size, size, false)
    render()
  }

  resize()
  const resizeObserver = new ResizeObserver(resize)
  resizeObserver.observe(parent)

  const clock = new THREE.Clock()
  let animationFrame = 0
  let dragging = false
  let activePointerId = null
  let lastPointerX = 0
  let lastPointerY = 0
  let rotationX = group.rotation.x
  let rotationY = group.rotation.y
  let inertialVelocityX = 0
  let inertialVelocityY = 0
  let animationEnabled = options.animate ?? !window.matchMedia('(prefers-reduced-motion: reduce)').matches
  let inView = true
  canvas.style.touchAction = 'pan-y'
  canvas.style.cursor = 'grab'

  const clampRotationX = (value) => Math.max(-0.45, Math.min(0.55, value))

  const onPointerDown = (event) => {
    if (event.button !== 0 || dragging) return
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
    render()
  }

  const endPointerInteraction = (event) => {
    if (activePointerId !== null && event.pointerId !== undefined && event.pointerId !== activePointerId) return
    dragging = false
    activePointerId = null
    canvas.style.cursor = 'grab'
    if (canvas.hasPointerCapture?.(event.pointerId)) canvas.releasePointerCapture(event.pointerId)
  }

  canvas.addEventListener('pointerdown', onPointerDown)
  canvas.addEventListener('pointermove', onPointerMove)
  canvas.addEventListener('pointerup', endPointerInteraction)
  canvas.addEventListener('pointercancel', endPointerInteraction)
  canvas.addEventListener('lostpointercapture', endPointerInteraction)

  const tick = () => {
    animationFrame = 0
    if (disposed) return

    const delta = Math.min(clock.getDelta(), 0.05)
    const elapsed = clock.elapsedTime

    if (!dragging) {
      rotationY += delta * 0.05 + inertialVelocityY * delta * 60
      rotationX = clampRotationX(rotationX + inertialVelocityX * delta * 60)
      inertialVelocityY *= Math.pow(0.94, delta * 60)
      inertialVelocityX *= Math.pow(0.9, delta * 60)
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

    group.position.y = Math.sin(elapsed * 0.6) * 0.025
    hubGlow.material.opacity = (dark ? 0.3 : 0.2) + Math.sin(elapsed * 1.8) * 0.06

    render()
    if (animationEnabled && !document.hidden && inView) animationFrame = window.requestAnimationFrame(tick)
  }

  const syncAnimation = () => {
    if (disposed) return
    if (animationFrame) window.cancelAnimationFrame(animationFrame)
    animationFrame = 0
    clock.getDelta()
    if (animationEnabled && !document.hidden && inView) animationFrame = window.requestAnimationFrame(tick)
    else render()
  }
  const visibilityObserver = new IntersectionObserver(([entry]) => {
    inView = entry.isIntersecting
    syncAnimation()
  })
  visibilityObserver.observe(canvas)
  document.addEventListener('visibilitychange', syncAnimation)
  syncAnimation()

  const cleanup = () => {
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
    visibilityObserver.disconnect()
    document.removeEventListener('visibilitychange', syncAnimation)
    group.traverse((item) => {
      item.userData.mapTexture?.dispose()
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
  try {
    // Await actual shader preparation rather than exposing the canvas after an arbitrary RAF.
    await renderer.compileAsync(scene, camera)
    renderer.setClearColor(0x000000, 0)
    renderer.clear(true, true, true)
    render()
    renderer.getContext().finish()
  } catch (error) {
    cleanup()
    throw error
  }
  return Object.assign(cleanup, {
    setTheme: (isDark) => { if (!disposed) setTheme(isDark) },
    setAnimating: (enabled) => { animationEnabled = enabled; syncAnimation() },
    setSiteLogo,
  })
}
