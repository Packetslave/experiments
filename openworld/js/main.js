import * as THREE from 'three';
import { RoomEnvironment } from 'three/addons/environments/RoomEnvironment.js';
import { Terrain } from './terrain.js';
import { buildWorld, PADS } from './world.js';
import { Player } from './player.js';

// Entry point: renderer, lighting, world, player, HUD, and the frame loop.

const SEED = 7;
const SKY = 0xbfe3dd; // the backdrop colour of the three.js keyframes example

const canvas = document.getElementById('scene');
const overlay = document.getElementById('overlay');
const overlayTitle = document.getElementById('overlay-title');
const overlayHint = document.getElementById('overlay-hint');
const startBtn = document.getElementById('start');
const hud = document.getElementById('hud');
const hudPos = document.getElementById('hud-pos');
const hudFps = document.getElementById('hud-fps');
const hudPoi = document.getElementById('hud-poi');

const renderer = new THREE.WebGLRenderer({ canvas, antialias: true, powerPreference: 'high-performance' });
renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
renderer.setSize(window.innerWidth, window.innerHeight);
renderer.toneMapping = THREE.ACESFilmicToneMapping;
renderer.toneMappingExposure = 1.05;
renderer.shadowMap.enabled = true;
renderer.shadowMap.type = THREE.PCFShadowMap;

const scene = new THREE.Scene();
scene.background = new THREE.Color(SKY);
scene.fog = new THREE.Fog(SKY, 140, 620);

const pmrem = new THREE.PMREMGenerator(renderer);
scene.environment = pmrem.fromScene(new RoomEnvironment(), 0.04).texture;
scene.environmentIntensity = 0.55;
pmrem.dispose();

const camera = new THREE.PerspectiveCamera(72, window.innerWidth / window.innerHeight, 0.1, 1200);

// Lights: soft sky/ground fill plus a warm sun with a shadow frustum that
// follows the player.
const hemi = new THREE.HemisphereLight(0xe8f4ff, 0x9fc9a0, 0.7);
scene.add(hemi);
const sun = new THREE.DirectionalLight(0xfff1dc, 2.4);
sun.castShadow = true;
sun.shadow.mapSize.set(2048, 2048);
sun.shadow.camera.near = 10;
sun.shadow.camera.far = 420;
sun.shadow.camera.left = -110;
sun.shadow.camera.right = 110;
sun.shadow.camera.top = 110;
sun.shadow.camera.bottom = -110;
sun.shadow.bias = -0.0004;
sun.shadow.normalBias = 0.6;
sun.shadow.camera.updateProjectionMatrix();
scene.add(sun);
scene.add(sun.target);
const SUN_OFFSET = new THREE.Vector3(90, 150, 70);

// World.
const terrain = new Terrain({ size: 520, segments: 260, seed: SEED, pads: PADS });
scene.add(terrain.mesh);
const world = buildWorld(scene, terrain, SEED);

// Player spawns between the houses and the plaza, looking at the fountain.
const player = new Player(camera, renderer.domElement, terrain, world.colliders);
player.spawn(PADS[0].x - 3, PADS[0].z + 13, -13);

// --- UI -----------------------------------------------------------------------

if (!matchMedia('(pointer: fine)').matches) {
  overlayHint.textContent = 'This demo uses pointer-lock mouse look and WASD keys, so it wants a desktop browser with a mouse.';
}

startBtn.addEventListener('click', () => player.lock());
player.controls.addEventListener('lock', () => {
  overlay.classList.add('hidden');
  hud.classList.remove('hidden');
});
player.controls.addEventListener('unlock', () => {
  overlayTitle.textContent = 'Paused';
  startBtn.textContent = 'Click to resume';
  overlay.classList.remove('hidden');
});

window.addEventListener('resize', () => {
  camera.aspect = window.innerWidth / window.innerHeight;
  camera.updateProjectionMatrix();
  renderer.setSize(window.innerWidth, window.innerHeight);
});

// --- Loop -------------------------------------------------------------------------

const timer = new THREE.Timer();
let elapsed = 0;
let fpsAccum = 0;
let fpsFrames = 0;
let currentPoi = null;
const snap = 4;

function frame() {
  timer.update();
  const dt = timer.getDelta();
  elapsed += dt;

  player.update(dt);
  world.update(dt, elapsed);

  // Keep the shadow frustum centred on the player, snapped to reduce shimmer.
  const px = Math.round(camera.position.x / snap) * snap;
  const pz = Math.round(camera.position.z / snap) * snap;
  sun.target.position.set(px, 0, pz);
  sun.position.set(px + SUN_OFFSET.x, SUN_OFFSET.y, pz + SUN_OFFSET.z);

  // HUD.
  fpsAccum += dt;
  fpsFrames++;
  if (fpsAccum >= 0.5) {
    hudFps.textContent = `${Math.round(fpsFrames / fpsAccum)} fps`;
    fpsAccum = 0;
    fpsFrames = 0;
    hudPos.textContent = `${camera.position.x.toFixed(0)}, ${camera.position.z.toFixed(0)} · alt ${player.feet.y.toFixed(1)} m${player.swimming ? ' · swimming' : ''}`;
    let near = null;
    for (const p of world.pois) {
      if (Math.hypot(camera.position.x - p.x, camera.position.z - p.z) < p.r) { near = p; break; }
    }
    if (near !== currentPoi) {
      currentPoi = near;
      hudPoi.textContent = near ? near.name : '';
      hudPoi.classList.toggle('show', !!near);
    }
  }

  renderer.render(scene, camera);
  requestAnimationFrame(frame);
}

overlayTitle.textContent = 'Driftwood Isle';
startBtn.disabled = false;
startBtn.textContent = 'Click to explore';
requestAnimationFrame(frame);

// Handy for poking at the scene from the console.
window.driftwood = { scene, camera, player, terrain, world };
